/**
 * Mobile auth store — Zustand. Logic mirrors packages/core/auth/store.ts:
 *   - Token written ONLY on successful verifyCode
 *   - 401 → clear token; non-401 (5xx / network blip) → preserve token so
 *     the next launch can retry
 *   - logout = clear token + clear in-memory user + setToken(null)
 *
 * NOT shared with web/desktop (per Sharing Principles in root CLAUDE.md).
 * Storage backend is expo-secure-store (mobile only); web uses HttpOnly
 * cookies, desktop uses localStorage via StorageAdapter.
 */
import { create } from "zustand";
import type { User } from "@multica/core/types";
import { api, ApiError } from "./api";
import { clearToken, getToken, setToken } from "./secure-storage";
import { useWorkspaceStore } from "./workspace-store";
import type { AppearanceUpdateRequest } from "@/lib/appearance-sync";
import {
  configureMobileAppearanceAnalytics,
  identifyMobileAppearanceAnalytics,
} from "./appearance-analytics";

async function refreshAppearanceAnalyticsConfig(): Promise<void> {
  try {
    configureMobileAppearanceAnalytics(
      await api.getConfig(),
    );
  } catch {
    // Configuration is opportunistic; appearance behavior never waits on it.
  }
}

interface AuthState {
  user: User | null;
  isLoading: boolean;
  initialize: () => Promise<void>;
  sendCode: (email: string) => Promise<void>;
  verifyCode: (email: string, code: string) => Promise<User>;
  logout: () => Promise<void>;
  refreshUser: () => Promise<User>;
  updateAppearancePreferences: (
    data: AppearanceUpdateRequest,
  ) => Promise<User>;
  /** Overwrite the in-memory user — call after PATCH /api/me so name/avatar
   *  edits land without a refetch. Server response is the source of truth. */
  setUser: (user: User) => void;
}

export const useAuthStore = create<AuthState>((set, get) => ({
  user: null,
  isLoading: true,

  initialize: async () => {
    void refreshAppearanceAnalyticsConfig();
    // Restore the persisted workspace slug alongside the auth token so the
    // entry redirect (app/index.tsx) can route directly to the last-used
    // workspace without flashing /select-workspace.
    await useWorkspaceStore.getState().restoreSlug();

    const token = await getToken();
    if (!token) {
      identifyMobileAppearanceAnalytics(null);
      set({ isLoading: false });
      return;
    }
    api.setToken(token);
    try {
      const user = await api.getMe();
      identifyMobileAppearanceAnalytics(user.id);
      set({ user, isLoading: false });
    } catch (err) {
      // Only clear token on a genuine 401. Network blips / 5xx keep the
      // token so the next launch (or a manual refresh) can retry.
      if (err instanceof ApiError && err.status === 401) {
        await clearToken();
        api.setToken(null);
      }
      identifyMobileAppearanceAnalytics(null);
      set({ user: null, isLoading: false });
    }
  },

  sendCode: async (email) => {
    await api.sendCode(email);
  },

  verifyCode: async (email, code) => {
    const { token, user } = await api.verifyCode(email, code);
    await setToken(token);
    api.setToken(token);
    identifyMobileAppearanceAnalytics(user.id);
    set({ user });
    return user;
  },

  logout: async () => {
    api.setToken(null);
    identifyMobileAppearanceAnalytics(null);
    set({ user: null });
    await clearToken();
  },

  refreshUser: async () => {
    const expectedUserId = get().user?.id;
    if (!expectedUserId) throw new Error("No current user to refresh");
    void refreshAppearanceAnalyticsConfig();
    const user = await api.getMe();
    if (user.id !== expectedUserId || get().user?.id !== expectedUserId) {
      throw new Error("Stale current-user response");
    }
    identifyMobileAppearanceAnalytics(user.id);
    set((state) => ({
      user: state.user
        ? {
            ...state.user,
            skin: user.skin,
            appearance: user.appearance,
            appearanceUpdatedAt: user.appearanceUpdatedAt,
            appearanceTokenVersion: user.appearanceTokenVersion,
          }
        : user,
    }));
    return user;
  },

  updateAppearancePreferences: async (data) => {
    const expectedUserId = get().user?.id;
    if (!expectedUserId) throw new Error("No current user to update");
    const user = await api.updateAppearancePreferences(data);
    if (user.id !== expectedUserId || get().user?.id !== expectedUserId) {
      throw new Error("Stale appearance sync response");
    }
    identifyMobileAppearanceAnalytics(user.id);
    set((state) => ({
      user: state.user
        ? {
            ...state.user,
            skin: user.skin,
            appearance: user.appearance,
            appearanceUpdatedAt: user.appearanceUpdatedAt,
            appearanceTokenVersion: user.appearanceTokenVersion,
          }
        : user,
    }));
    return user;
  },

  setUser: (user) => {
    identifyMobileAppearanceAnalytics(user.id);
    set({ user });
  },
}));
