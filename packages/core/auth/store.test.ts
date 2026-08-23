// @vitest-environment node

import { describe, expect, it, vi } from "vitest";
import { EMPTY_USER } from "../api/schemas";
import type { ApiClient } from "../api/client";
import type { StorageAdapter, User } from "../types";
import {
  AUTHENTICATED_ACCOUNT_STORAGE_KEY,
  createAuthStore,
} from "./store";

const fakeUser: User = {
  ...EMPTY_USER,
  id: "u1",
  name: "Alice",
  email: "alice@example.com",
  avatar_url: null,
};

function makeStorage(initial: Record<string, string> = {}): StorageAdapter & {
  snapshot: () => Record<string, string>;
} {
  const data = { ...initial };
  return {
    getItem: (k) => data[k] ?? null,
    setItem: (k, v) => {
      data[k] = v;
    },
    removeItem: (k) => {
      delete data[k];
    },
    snapshot: () => ({ ...data }),
  };
}

function makeApi(overrides: Partial<ApiClient> = {}): ApiClient {
  return {
    setToken: vi.fn(),
    ...overrides,
  } as unknown as ApiClient;
}

describe("authStore", () => {
  it("publishes a retry request instead of silently ignoring it", () => {
    const storage = makeStorage({ multica_token: "t" });
    const api = makeApi();
    const store = createAuthStore({ api, storage });

    store.setState({ isLoading: true, status: "recovering" });
    store.getState().retryAuthentication();

    expect(store.getState().status).toBe("authenticating");
    expect(store.getState().retryGeneration).toBe(1);
  });

  it("explicit logout still clears credentials and publishes unauthenticated state", () => {
    const storage = makeStorage({
      multica_token: "t",
      [AUTHENTICATED_ACCOUNT_STORAGE_KEY]: fakeUser.id,
    });
    const api = makeApi();
    const onLogout = vi.fn();
    const store = createAuthStore({ api, storage, onLogout });

    store.setState({ user: fakeUser, status: "authenticated", isLoading: false });
    store.getState().logout();

    expect(storage.snapshot().multica_token).toBeUndefined();
    expect(storage.snapshot()[AUTHENTICATED_ACCOUNT_STORAGE_KEY]).toBeUndefined();
    expect(api.setToken).toHaveBeenCalledWith(null);
    expect(onLogout).toHaveBeenCalledOnce();
    expect(store.getState().user).toBeNull();
    expect(store.getState().status).toBe("unauthenticated");
  });

  it("merges a refreshed appearance without reverting newer profile state", async () => {
    let resolveRefresh!: (user: User) => void;
    const refresh = new Promise<User>((resolve) => {
      resolveRefresh = resolve;
    });
    const api = makeApi({ getMe: vi.fn(() => refresh) });
    const store = createAuthStore({ api, storage: makeStorage() });
    store.setState({
      user: { ...fakeUser, name: "Before refresh" },
      status: "authenticated",
      isLoading: false,
    });

    const pending = store.getState().refreshAppearancePreferences();
    store.setState({ user: { ...fakeUser, name: "Saved while refreshing" } });
    resolveRefresh({
      ...fakeUser,
      name: "Before refresh",
      skin: "field",
      appearance: "dark",
      appearanceUpdatedAt: "2026-08-23T12:00:00.000Z",
      appearanceTokenVersion: 1,
    });
    await pending;

    expect(store.getState().user).toMatchObject({
      name: "Saved while refreshing",
      skin: "field",
      appearance: "dark",
    });
  });
});
