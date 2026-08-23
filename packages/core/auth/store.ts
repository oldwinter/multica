import { create } from "zustand";
import type { User, StorageAdapter, UpdateMeRequest } from "../types";
import { identify as identifyAnalytics, resetAnalytics } from "../analytics";
import type { ApiClient } from "../api/client";
import { setCurrentWorkspace } from "../platform/workspace-storage";

export const AUTHENTICATED_ACCOUNT_STORAGE_KEY =
  "multica-authenticated-account-id";

export interface AuthStoreOptions {
  api: ApiClient;
  storage: StorageAdapter;
  onLogin?: () => void;
  onLogout?: () => void;
  /** When true, rely on HttpOnly cookies instead of localStorage for auth tokens. */
  cookieAuth?: boolean;
}

export type AuthStatus =
  | "authenticating"
  | "authenticated"
  | "unauthenticated"
  | "recovering";

export type AppearanceUpdateRequest = Required<
  Pick<
    UpdateMeRequest,
    "skin" | "appearance" | "appearanceUpdatedAt" | "appearanceTokenVersion"
  >
>;

export interface AuthState {
  user: User | null;
  isLoading: boolean;
  status: AuthStatus;
  retryGeneration: number;

  retryAuthentication: () => void;
  sendCode: (email: string) => Promise<void>;
  verifyCode: (email: string, code: string) => Promise<User>;
  loginWithGoogle: (code: string, redirectUri: string) => Promise<User>;
  loginWithToken: (token: string) => Promise<User>;
  logout: () => void;
  setUser: (user: User) => void;
  refreshMe: () => Promise<void>;
  updateAppearancePreferences: (
    data: AppearanceUpdateRequest,
  ) => Promise<User>;
  refreshAppearancePreferences: () => Promise<User>;
}

function mergeAppearanceFields(current: User, updated: User): User {
  return {
    ...current,
    skin: updated.skin,
    appearance: updated.appearance,
    appearanceUpdatedAt: updated.appearanceUpdatedAt,
    appearanceTokenVersion: updated.appearanceTokenVersion,
  };
}

export function createAuthStore(options: AuthStoreOptions) {
  const { api, storage, onLogin, onLogout, cookieAuth } = options;

  return create<AuthState>((set, get) => ({
    user: null,
    isLoading: true,
    status: "authenticating",
    retryGeneration: 0,

    retryAuthentication: () => {
      set((state) => ({
        isLoading: true,
        status: "authenticating",
        retryGeneration: state.retryGeneration + 1,
      }));
    },

    sendCode: async (email: string) => {
      await api.sendCode(email);
    },

    verifyCode: async (email: string, code: string) => {
      const { token, user } = await api.verifyCode(email, code);
      if (!cookieAuth) {
        // Token mode: persist for Electron / legacy.
        storage.setItem("multica_token", token);
        api.setToken(token);
      }
      storage.setItem(AUTHENTICATED_ACCOUNT_STORAGE_KEY, user.id);
      onLogin?.();
      identifyAnalytics(user.id, { email: user.email, name: user.name });
      set({ user, isLoading: false, status: "authenticated" });
      return user;
    },

    loginWithGoogle: async (code: string, redirectUri: string) => {
      const { token, user } = await api.googleLogin(code, redirectUri);
      if (!cookieAuth) {
        storage.setItem("multica_token", token);
        api.setToken(token);
      }
      storage.setItem(AUTHENTICATED_ACCOUNT_STORAGE_KEY, user.id);
      onLogin?.();
      identifyAnalytics(user.id, { email: user.email, name: user.name });
      set({ user, isLoading: false, status: "authenticated" });
      return user;
    },

    loginWithToken: async (token: string) => {
      storage.setItem("multica_token", token);
      api.setToken(token);
      const user = await api.getMe();
      storage.setItem(AUTHENTICATED_ACCOUNT_STORAGE_KEY, user.id);
      onLogin?.();
      identifyAnalytics(user.id, { email: user.email, name: user.name });
      set({ user, isLoading: false, status: "authenticated" });
      return user;
    },

    logout: () => {
      if (cookieAuth) {
        // Clear server-side HttpOnly cookie.
        api.logout().catch(() => {});
      }
      storage.removeItem("multica_token");
      storage.removeItem(AUTHENTICATED_ACCOUNT_STORAGE_KEY);
      api.setToken(null);
      setCurrentWorkspace(null, null);
      resetAnalytics();
      onLogout?.();
      set({ user: null, isLoading: false, status: "unauthenticated" });
    },

    setUser: (user: User) => {
      storage.setItem(AUTHENTICATED_ACCOUNT_STORAGE_KEY, user.id);
      set({ user, isLoading: false, status: "authenticated" });
    },

    refreshMe: async () => {
      const user = await api.getMe();
      storage.setItem(AUTHENTICATED_ACCOUNT_STORAGE_KEY, user.id);
      set({ user, isLoading: false, status: "authenticated" });
    },

    updateAppearancePreferences: async (data) => {
      const accountId = get().user?.id;
      if (!accountId) throw new Error("No current user to update");
      const updated = await api.updateMe(data);
      if (updated.id !== accountId || get().user?.id !== accountId) {
        throw new Error("Stale appearance sync response");
      }
      set((state) => ({
        user:
          state.user?.id === accountId
            ? mergeAppearanceFields(state.user, updated)
            : state.user,
      }));
      return updated;
    },

    refreshAppearancePreferences: async () => {
      const accountId = get().user?.id;
      if (!accountId) throw new Error("No current user to refresh");
      const updated = await api.getMe();
      if (updated.id !== accountId || get().user?.id !== accountId) {
        throw new Error("Stale appearance refresh response");
      }
      set((state) => ({
        user:
          state.user?.id === accountId
            ? mergeAppearanceFields(state.user, updated)
            : state.user,
      }));
      return updated;
    },
  }));
}
