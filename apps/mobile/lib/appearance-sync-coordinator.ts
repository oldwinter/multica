import {
  markAppearanceSyncFailed,
  reconcileAppearancePreferences,
  type AppearanceEnvironment,
  type AppearancePreferenceField,
  type AppearancePreferences,
} from "@multica/core/appearance";
import {
  classifyAppearanceSyncError,
  readServerAppearance,
  toAppearanceUpdateRequest,
  type AppearanceUpdateRequest,
} from "./appearance-sync";

export interface AppearanceSyncCoordinatorDependencies {
  getUser(): unknown | null;
  getUserId(user: unknown): string | null;
  getPreferences(): AppearancePreferences;
  isLocalCacheWritable(): boolean;
  refreshEnvironment(): Promise<AppearanceEnvironment>;
  prepareLocalForUser(
    userId: string,
    environment: AppearanceEnvironment,
  ): Promise<AppearancePreferences>;
  switchToSystemAppearance(): Promise<AppearanceEnvironment>;
  commit(preferences: AppearancePreferences, persist?: boolean): Promise<void>;
  addRecoveredFields(
    fields: readonly AppearancePreferenceField[],
    recovered: boolean,
  ): void;
  updateAppearance(data: AppearanceUpdateRequest): Promise<unknown>;
  refreshUser(): Promise<unknown>;
}

export interface AppearanceSyncCoordinator {
  reconcileUser(user: unknown | null): Promise<void>;
  requestSync(): Promise<void>;
  refreshAuthenticated(): Promise<void>;
}

export function createAppearanceSyncCoordinator(
  dependencies: AppearanceSyncCoordinatorDependencies,
): AppearanceSyncCoordinator {
  let syncInFlight: Promise<void> | null = null;
  let syncQueued = false;
  let userReconciliationVersion = 0;
  let activeUserId: string | null = null;

  async function reconcileWithServer(
    current: AppearancePreferences,
    user: unknown,
    initialEnvironment: AppearanceEnvironment,
  ) {
    let environment = initialEnvironment;
    let server = readServerAppearance(user, environment.systemAppearance);
    let reconciled = reconcileAppearancePreferences(
      current,
      server.preferences,
      environment.systemAppearance,
    );
    if (
      dependencies.getPreferences() !== current ||
      dependencies.getUser() !== user
    ) {
      return { environment, server, reconciled, stale: true };
    }

    if (
      server.writable &&
      reconciled.winner === "server" &&
      server.preferences?.requestedAppearance === "system" &&
      current.requestedAppearance !== "system"
    ) {
      environment = await dependencies.switchToSystemAppearance();
      server = readServerAppearance(user, environment.systemAppearance);
      reconciled = reconcileAppearancePreferences(
        current,
        server.preferences,
        environment.systemAppearance,
      );
    }

    if (
      dependencies.getPreferences() !== current ||
      dependencies.getUser() !== user
    ) {
      return { environment, server, reconciled, stale: true };
    }
    return { environment, server, reconciled, stale: false };
  }

  async function syncOneAppearanceChange(): Promise<void> {
    const user = dependencies.getUser();
    if (!user) return;
    const userId = dependencies.getUserId(user);
    if (!userId) return;
    if (!dependencies.isLocalCacheWritable()) {
      await dependencies.commit(
        markAppearanceSyncFailed(dependencies.getPreferences(), "conflict"),
        false,
      );
      return;
    }

    const initialEnvironment = await dependencies.refreshEnvironment();
    if (!initialEnvironment.online) return;

    const current = dependencies.getPreferences();
    const { environment, server, reconciled, stale } =
      await reconcileWithServer(current, user, initialEnvironment);
    if (stale || dependencies.getPreferences() !== current) {
      syncQueued = true;
      return;
    }
    dependencies.addRecoveredFields(server.recoveredFields, server.writable);
    if (!server.writable) {
      await dependencies.commit(markAppearanceSyncFailed(current, "conflict"));
      return;
    }

    await dependencies.commit(
      reconciled.preferences,
      reconciled.shouldPersistLocal ||
        reconciled.winner === "default" ||
        (current.source === "default" && server.preferences === null),
    );
    const repairsRecoveredServerValue = server.recoveredFields.length > 0;
    if (!reconciled.shouldSyncServer && !repairsRecoveredServerValue) return;

    const outgoing = repairsRecoveredServerValue
      ? {
          ...reconciled.preferences,
          source: "local" as const,
          syncState: { status: "pending" as const },
        }
      : reconciled.preferences;
    if (repairsRecoveredServerValue) await dependencies.commit(outgoing);
    try {
      const updated = await dependencies.updateAppearance(
        toAppearanceUpdateRequest(outgoing),
      );
      if (dependencies.getUserId(dependencies.getUser()) !== userId) return;
      const acknowledged = readServerAppearance(
        updated,
        environment.systemAppearance,
      );
      dependencies.addRecoveredFields(
        acknowledged.recoveredFields,
        acknowledged.writable,
      );
      const latest = dependencies.getPreferences();
      if (!acknowledged.writable || !acknowledged.preferences) {
        await dependencies.commit(
          markAppearanceSyncFailed(latest, "conflict"),
        );
        return;
      }

      const settled = reconcileAppearancePreferences(
        latest,
        acknowledged.preferences,
        environment.systemAppearance,
      );
      await dependencies.commit(settled.preferences);
      if (settled.shouldSyncServer) syncQueued = true;
    } catch (error) {
      if (dependencies.getUserId(dependencies.getUser()) !== userId) return;
      await dependencies.commit(
        markAppearanceSyncFailed(
          dependencies.getPreferences(),
          classifyAppearanceSyncError(error),
        ),
      );
    }
  }

  function requestSync(): Promise<void> {
    if (syncInFlight) {
      syncQueued = true;
      return syncInFlight;
    }
    syncInFlight = (async () => {
      do {
        syncQueued = false;
        await syncOneAppearanceChange();
      } while (syncQueued);
    })().finally(() => {
      syncInFlight = null;
    });
    return syncInFlight;
  }

  async function reconcileUser(user: unknown | null): Promise<void> {
    const version = ++userReconciliationVersion;
    if (!user) {
      activeUserId = null;
      return;
    }
    const userId = dependencies.getUserId(user);
    if (!userId) return;

    const environment = await dependencies.refreshEnvironment();
    if (version !== userReconciliationVersion) return;
    const current =
      activeUserId === userId
        ? dependencies.getPreferences()
        : await dependencies.prepareLocalForUser(userId, environment);
    activeUserId = userId;
    if (version !== userReconciliationVersion) return;

    const { server, reconciled, stale } = await reconcileWithServer(
      current,
      user,
      environment,
    );
    if (
      stale ||
      version !== userReconciliationVersion ||
      dependencies.getPreferences() !== current
    ) {
      return;
    }
    dependencies.addRecoveredFields(server.recoveredFields, server.writable);
    if (!server.writable) {
      await dependencies.commit(markAppearanceSyncFailed(current, "conflict"));
      return;
    }

    await dependencies.commit(
      reconciled.preferences,
      reconciled.shouldPersistLocal ||
        reconciled.winner === "default" ||
        (current.source === "default" && server.preferences === null),
    );
    if (reconciled.shouldSyncServer || server.recoveredFields.length > 0) {
      await requestSync();
    }
  }

  async function refreshAuthenticated(): Promise<void> {
    if (!dependencies.getUser()) return;
    try {
      const updated = await dependencies.refreshUser();
      await reconcileUser(updated);
    } catch {
      await requestSync();
    }
  }

  return { reconcileUser, requestSync, refreshAuthenticated };
}
