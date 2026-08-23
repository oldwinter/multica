// @vitest-environment node

import { describe, expect, it, vi } from "vitest";
import {
  APPEARANCE_PREFERENCES_VERSION,
  APPEARANCE_TOKEN_CONTRACT_VERSION,
  createDefaultAppearancePreferences,
  type AppearanceEnvironment,
  type AppearancePreferences,
} from "@multica/core/appearance";
import { createAppearanceSyncCoordinator } from "./appearance-sync-coordinator";
import type { AppearanceUpdateRequest } from "./appearance-sync";

function preference(
  skin: AppearancePreferences["skin"],
  updatedAt: string,
): AppearancePreferences {
  return {
    version: APPEARANCE_PREFERENCES_VERSION,
    tokenContractVersion: APPEARANCE_TOKEN_CONTRACT_VERSION,
    skin,
    requestedAppearance: "dark",
    resolvedAppearance: "dark",
    source: "local",
    updatedAt,
    syncState: { status: "pending" },
  };
}

function userFrom(
  id: string,
  preferences: AppearancePreferences | null,
): Record<string, unknown> {
  return {
    id,
    skin: preferences?.skin ?? null,
    appearance: preferences?.requestedAppearance ?? null,
    appearanceUpdatedAt: preferences?.updatedAt ?? null,
    appearanceTokenVersion: preferences?.tokenContractVersion ?? null,
  };
}

function userFromUpdate(id: string, update: AppearanceUpdateRequest) {
  return {
    id,
    skin: update.skin,
    appearance: update.appearance,
    appearanceUpdatedAt: update.appearanceUpdatedAt,
    appearanceTokenVersion: update.appearanceTokenVersion,
  };
}

function createHarness(options?: {
  preferences?: AppearancePreferences;
  user?: Record<string, unknown>;
  online?: boolean;
  writable?: boolean;
}) {
  let preferences =
    options?.preferences ?? createDefaultAppearancePreferences("light");
  let user = options?.user ?? userFrom("user-a", null);
  let environment: AppearanceEnvironment = {
    systemAppearance: "light",
    reducedMotion: false,
    forcedColors: false,
    online: options?.online ?? true,
  };
  let writable = options?.writable ?? true;
  let refreshedUser: Record<string, unknown> = user;
  const accountCaches = new Map<string, AppearancePreferences>();
  const commits: AppearancePreferences[] = [];
  const persistCalls: boolean[] = [];

  const updateAppearance = vi.fn(
    async (update: AppearanceUpdateRequest): Promise<unknown> => {
      user = userFromUpdate(String(user.id), update);
      return user;
    },
  );
  const refreshUser = vi.fn(async (): Promise<unknown> => {
    user = refreshedUser;
    return user;
  });
  const prepareLocalForUser = vi.fn(
    async (userId: string): Promise<AppearancePreferences> => {
      preferences =
        accountCaches.get(userId) ?? createDefaultAppearancePreferences("light");
      return preferences;
    },
  );

  const coordinator = createAppearanceSyncCoordinator({
    getUser: () => user,
    getUserId: (candidate) =>
      typeof candidate === "object" &&
      candidate !== null &&
      "id" in candidate &&
      typeof candidate.id === "string"
        ? candidate.id
        : null,
    getPreferences: () => preferences,
    isLocalCacheWritable: () => writable,
    refreshEnvironment: async () => environment,
    prepareLocalForUser,
    switchToSystemAppearance: async () => environment,
    commit: async (next, persist = true) => {
      preferences = next;
      commits.push(next);
      persistCalls.push(persist);
    },
    addRecoveredFields: () => undefined,
    updateAppearance,
    refreshUser,
  });

  return {
    coordinator,
    updateAppearance,
    refreshUser,
    prepareLocalForUser,
    accountCaches,
    commits,
    persistCalls,
    get preferences() {
      return preferences;
    },
    get user() {
      return user;
    },
    set user(next: Record<string, unknown>) {
      user = next;
    },
    set localPreferences(next: AppearancePreferences) {
      preferences = next;
    },
    set online(next: boolean) {
      environment = { ...environment, online: next };
    },
    set refreshedUser(next: Record<string, unknown>) {
      refreshedUser = next;
    },
    set writable(next: boolean) {
      writable = next;
    },
  };
}

describe("mobile appearance sync coordinator", () => {
  it("persists a clean default when a new account becomes active", async () => {
    const harness = createHarness({ user: userFrom("user-b", null) });

    await harness.coordinator.reconcileUser(harness.user);

    expect(harness.preferences).toMatchObject({
      skin: "tension",
      requestedAppearance: "system",
    });
    expect(harness.persistCalls).toContain(true);
    expect(harness.updateAppearance).not.toHaveBeenCalled();
  });

  it("PATCHes a newer account-scoped local tuple and settles the response", async () => {
    const local = preference("field", "2026-08-23T12:00:00.000Z");
    const remote = preference("relay", "2026-08-23T10:00:00.000Z");
    const harness = createHarness({ user: userFrom("user-a", remote) });
    harness.accountCaches.set("user-a", local);

    await harness.coordinator.reconcileUser(harness.user);

    expect(harness.updateAppearance).toHaveBeenCalledOnce();
    expect(harness.updateAppearance).toHaveBeenCalledWith({
      skin: "field",
      appearance: "dark",
      appearanceUpdatedAt: "2026-08-23T12:00:00.000Z",
      appearanceTokenVersion: 1,
    });
    expect(harness.preferences).toMatchObject({
      skin: "field",
      source: "server",
      syncState: { status: "synced" },
    });
  });

  it("keeps an offline change pending, GETs on reconnect, then PATCHes", async () => {
    const local = preference("field", "2026-08-23T12:00:00.000Z");
    const remote = preference("relay", "2026-08-23T10:00:00.000Z");
    const harness = createHarness({
      user: userFrom("user-a", remote),
      online: false,
    });
    harness.accountCaches.set("user-a", local);

    await harness.coordinator.reconcileUser(harness.user);
    expect(harness.updateAppearance).not.toHaveBeenCalled();
    expect(harness.preferences.syncState.status).toBe("pending");

    harness.online = true;
    harness.refreshedUser = userFrom("user-a", remote);
    await harness.coordinator.refreshAuthenticated();

    expect(harness.refreshUser).toHaveBeenCalledOnce();
    expect(harness.updateAppearance).toHaveBeenCalledOnce();
    expect(harness.preferences.skin).toBe("field");
    expect(harness.preferences.syncState.status).toBe("synced");
  });

  it("does not let an in-flight PATCH overwrite a newer local choice", async () => {
    const first = preference("relay", "2026-08-23T11:00:00.000Z");
    const newer = preference("field", "2026-08-23T12:00:00.000Z");
    const harness = createHarness({ user: userFrom("user-a", null) });
    harness.accountCaches.set("user-a", first);
    let resolveFirst!: (value: unknown) => void;
    const firstResponse = new Promise<unknown>((resolve) => {
      resolveFirst = resolve;
    });
    harness.updateAppearance
      .mockImplementationOnce(async (update) => {
        const updated = await firstResponse;
        harness.user = updated as Record<string, unknown>;
        return updated;
      })
      .mockImplementationOnce(async (update) => {
        const updated = userFromUpdate("user-a", update);
        harness.user = updated;
        return updated;
      });

    const syncing = harness.coordinator.reconcileUser(harness.user);
    await vi.waitFor(() => expect(harness.updateAppearance).toHaveBeenCalled());
    harness.localPreferences = newer;
    resolveFirst(
      userFromUpdate("user-a", harness.updateAppearance.mock.calls[0][0]),
    );
    await syncing;

    expect(harness.updateAppearance).toHaveBeenCalledTimes(2);
    expect(harness.preferences).toMatchObject({
      skin: "field",
      source: "server",
      syncState: { status: "synced" },
    });
  });

  it("loads each account cache before reconciliation", async () => {
    const accountA = preference("relay", "2026-08-23T12:00:00.000Z");
    const harness = createHarness({ user: userFrom("user-a", null) });
    harness.accountCaches.set("user-a", accountA);
    await harness.coordinator.reconcileUser(harness.user);
    expect(harness.updateAppearance).toHaveBeenCalledOnce();

    harness.user = userFrom("user-b", null);
    await harness.coordinator.reconcileUser(harness.user);

    expect(harness.prepareLocalForUser).toHaveBeenLastCalledWith(
      "user-b",
      expect.objectContaining({ online: true }),
    );
    expect(harness.updateAppearance).toHaveBeenCalledOnce();
    expect(harness.preferences).toMatchObject({
      skin: "tension",
      source: "default",
    });
  });

  it("discards an in-flight PATCH after the account changes", async () => {
    const local = preference("relay", "2026-08-23T12:00:00.000Z");
    const harness = createHarness({ user: userFrom("user-a", null) });
    harness.accountCaches.set("user-a", local);
    let resolvePatch!: (value: unknown) => void;
    harness.updateAppearance.mockImplementationOnce(
      () =>
        new Promise((resolve) => {
          resolvePatch = resolve;
        }),
    );

    const syncing = harness.coordinator.reconcileUser(harness.user);
    await vi.waitFor(() => expect(harness.updateAppearance).toHaveBeenCalled());
    harness.user = userFrom("user-b", null);
    resolvePatch(
      userFromUpdate("user-a", harness.updateAppearance.mock.calls[0][0]),
    );
    await syncing;

    expect(harness.user).toMatchObject({ id: "user-b" });
    expect(harness.commits.at(-1)).toMatchObject({
      skin: "relay",
      syncState: { status: "pending" },
    });
  });

  it("fails closed without PATCHing when the local token is newer", async () => {
    const local = preference("field", "2026-08-23T12:00:00.000Z");
    const harness = createHarness({
      preferences: local,
      user: userFrom("user-a", null),
      writable: false,
    });
    harness.accountCaches.set("user-a", local);

    await harness.coordinator.reconcileUser(harness.user);
    await harness.coordinator.requestSync();

    expect(harness.updateAppearance).not.toHaveBeenCalled();
    expect(harness.preferences.syncState).toEqual({
      status: "failed",
      errorClass: "conflict",
    });
  });
});
