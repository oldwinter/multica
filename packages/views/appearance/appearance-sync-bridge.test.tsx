import type { ReactNode } from "react";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  APPEARANCE_PREFERENCES_VERSION,
  APPEARANCE_TOKEN_CONTRACT_VERSION,
  type AppearanceAdapterEvent,
  type AppearanceEnvironment,
  type AppearancePreferenceAdapter,
  type AppearancePreferences,
} from "@multica/core/appearance";
import { EMPTY_USER } from "@multica/core/api/schemas";
import type { UpdateMeRequest, User } from "@multica/core/types";

const mockUpdateMe = vi.hoisted(() => vi.fn());
const mockGetMe = vi.hoisted(() => vi.fn());
const mockSetSkin = vi.hoisted(() => vi.fn());
const mockSetTheme = vi.hoisted(() => vi.fn());
const userRef = vi.hoisted(() => ({ current: null as User | null }));

vi.mock("@multica/core/analytics", () => ({ captureEvent: vi.fn() }));

vi.mock("@multica/ui/components/common/theme-provider", () => ({
  useSkin: () => ({ skin: "tension", setSkin: mockSetSkin }),
  useTheme: () => ({ theme: "system", setTheme: mockSetTheme }),
}));

import {
  AppearanceSyncBridge,
  useAppearancePreferences,
} from "./appearance-sync-bridge";

const DARK_ENVIRONMENT: AppearanceEnvironment = {
  systemAppearance: "dark",
  reducedMotion: false,
  forcedColors: false,
  online: true,
};

function preference(
  overrides: Partial<AppearancePreferences> = {},
): AppearancePreferences {
  return {
    version: APPEARANCE_PREFERENCES_VERSION,
    tokenContractVersion: APPEARANCE_TOKEN_CONTRACT_VERSION,
    skin: "relay",
    requestedAppearance: "system",
    resolvedAppearance: "dark",
    source: "local",
    updatedAt: "2026-08-23T10:00:00.000Z",
    syncState: { status: "pending" },
    ...overrides,
  };
}

function serverUser(overrides: Partial<User> = {}): User {
  return {
    ...EMPTY_USER,
    id: "user-1",
    name: "Ada",
    email: "ada@example.com",
    ...overrides,
  };
}

function createAdapter(initial: unknown | null) {
  let stored = initial;
  let applied: AppearancePreferences | null = null;
  const listeners = new Set<(event: AppearanceAdapterEvent) => void>();
  const adapter: AppearancePreferenceAdapter = {
    source: "web",
    supportsRemoteSync: true,
    load: () => stored,
    persist: (next) => {
      stored = next;
    },
    apply: (next) => {
      applied = next;
    },
    getEnvironment: () => DARK_ENVIRONMENT,
    subscribe: (listener) => {
      listeners.add(listener);
      return () => listeners.delete(listener);
    },
  };
  return {
    adapter,
    stored: () => stored as AppearancePreferences,
    applied: () => applied,
    emit: (event: AppearanceAdapterEvent) => {
      for (const listener of listeners) listener(event);
    },
  };
}

function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((resolvePromise) => {
    resolve = resolvePromise;
  });
  return { promise, resolve };
}

function Probe() {
  const {
    preferences,
    isReady,
    selectSkin,
    selectAppearance,
    retry,
    reset,
  } = useAppearancePreferences();
  return (
    <div>
      <output data-testid="ready">{String(isReady)}</output>
      <output data-testid="skin">{preferences.skin}</output>
      <output data-testid="appearance">{preferences.requestedAppearance}</output>
      <output data-testid="resolved">{preferences.resolvedAppearance}</output>
      <output data-testid="sync">{preferences.syncState.status}</output>
      <button type="button" onClick={() => selectSkin("field")}>
        Field
      </button>
      <button type="button" onClick={() => selectAppearance("system")}>
        System
      </button>
      <button type="button" onClick={() => selectAppearance("dark")}>
        Dark
      </button>
      <button type="button" onClick={retry}>
        Retry
      </button>
      <button type="button" onClick={reset}>
        Reset
      </button>
    </div>
  );
}

function renderBridge(
  adapter: AppearancePreferenceAdapter,
  children: ReactNode = <Probe />,
  capture = vi.fn(),
) {
  return render(
    <AppearanceSyncBridge
      adapter={adapter}
      account={userRef.current}
      updateAccountAppearance={mockUpdateMe}
      refreshAccountAppearance={mockGetMe}
      capture={capture}
    >
      {children}
    </AppearanceSyncBridge>,
  );
}

describe("AppearanceSyncBridge", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  beforeEach(() => {
    vi.clearAllMocks();
    userRef.current = null;
  });

  it("keeps anonymous changes local instead of claiming a pending sync", async () => {
    const harness = createAdapter(preference());
    renderBridge(harness.adapter);
    const user = userEvent.setup();

    await waitFor(() => expect(screen.getByTestId("ready")).toHaveTextContent("true"));
    expect(screen.getByTestId("sync")).toHaveTextContent("local-only");
    await user.click(screen.getByRole("button", { name: "Field" }));

    await waitFor(() => expect(harness.stored().skin).toBe("field"));
    expect(harness.stored().syncState).toEqual({ status: "local-only" });
    expect(mockUpdateMe).not.toHaveBeenCalled();
  });

  it("applies and caches a newer server tuple without writing it back", async () => {
    userRef.current = serverUser({
      skin: "field",
      appearance: "dark",
      appearanceUpdatedAt: "2026-08-23T11:00:00.000Z",
      appearanceTokenVersion: APPEARANCE_TOKEN_CONTRACT_VERSION,
    });
    const harness = createAdapter(preference());
    renderBridge(harness.adapter);

    await waitFor(() => expect(screen.getByTestId("skin")).toHaveTextContent("field"));
    expect(screen.getByTestId("appearance")).toHaveTextContent("dark");
    expect(screen.getByTestId("sync")).toHaveTextContent("synced");
    expect(harness.stored()).toMatchObject({
      skin: "field",
      requestedAppearance: "dark",
      source: "server",
      syncState: { status: "synced" },
    });
    expect(mockUpdateMe).not.toHaveBeenCalled();
  });

  it("writes an explicit local change as one timestamped server tuple", async () => {
    userRef.current = serverUser();
    const harness = createAdapter(null);
    mockUpdateMe.mockImplementation(async (request: UpdateMeRequest) =>
      serverUser({
        skin: request.skin,
        appearance: request.appearance,
        appearanceUpdatedAt: request.appearanceUpdatedAt,
        appearanceTokenVersion: request.appearanceTokenVersion,
      }),
    );
    renderBridge(harness.adapter);
    const user = userEvent.setup();

    await waitFor(() => expect(screen.getByTestId("ready")).toHaveTextContent("true"));
    await user.click(screen.getByRole("button", { name: "Field" }));

    await waitFor(() => expect(screen.getByTestId("sync")).toHaveTextContent("synced"));
    expect(mockUpdateMe).toHaveBeenCalledWith({
      skin: "field",
      appearance: "system",
      appearanceUpdatedAt: expect.stringMatching(/^\d{4}-\d{2}-\d{2}T/),
      appearanceTokenVersion: APPEARANCE_TOKEN_CONTRACT_VERSION,
    });
    expect(harness.applied()).toMatchObject({
      skin: "field",
      requestedAppearance: "system",
      syncState: { status: "synced" },
    });
  });

  it("keeps a failed local choice and retries it without changing timestamp", async () => {
    userRef.current = serverUser();
    const harness = createAdapter(null);
    mockUpdateMe.mockRejectedValueOnce(new TypeError("offline"));
    renderBridge(harness.adapter);
    const user = userEvent.setup();

    await waitFor(() => expect(screen.getByTestId("ready")).toHaveTextContent("true"));
    await user.click(screen.getByRole("button", { name: "Field" }));
    await waitFor(() => expect(screen.getByTestId("sync")).toHaveTextContent("failed"));
    const failedTimestamp = harness.stored().updatedAt;
    mockUpdateMe.mockImplementationOnce(async (request: UpdateMeRequest) =>
      serverUser({
        skin: request.skin,
        appearance: request.appearance,
        appearanceUpdatedAt: request.appearanceUpdatedAt,
        appearanceTokenVersion: request.appearanceTokenVersion,
      }),
    );

    await user.click(screen.getByRole("button", { name: "Retry" }));

    await waitFor(() => expect(screen.getByTestId("sync")).toHaveTextContent("synced"));
    expect(mockUpdateMe).toHaveBeenLastCalledWith(
      expect.objectContaining({ appearanceUpdatedAt: failedTimestamp }),
    );
  });

  it("resets both choices and syncs the product defaults as one tuple", async () => {
    userRef.current = serverUser({
      skin: "field",
      appearance: "dark",
      appearanceUpdatedAt: "2026-08-23T11:00:00.000Z",
      appearanceTokenVersion: APPEARANCE_TOKEN_CONTRACT_VERSION,
    });
    const harness = createAdapter(
      preference({
        skin: "field",
        requestedAppearance: "dark",
        resolvedAppearance: "dark",
        source: "server",
        updatedAt: "2026-08-23T11:00:00.000Z",
        syncState: { status: "synced" },
      }),
    );
    mockUpdateMe.mockImplementation(async (request: UpdateMeRequest) =>
      serverUser({
        skin: request.skin,
        appearance: request.appearance,
        appearanceUpdatedAt: request.appearanceUpdatedAt,
        appearanceTokenVersion: request.appearanceTokenVersion,
      }),
    );
    renderBridge(harness.adapter);
    const user = userEvent.setup();

    await waitFor(() => expect(screen.getByTestId("ready")).toHaveTextContent("true"));
    await user.click(screen.getByRole("button", { name: "Reset" }));

    await waitFor(() => expect(screen.getByTestId("skin")).toHaveTextContent("tension"));
    expect(mockUpdateMe).toHaveBeenCalledWith({
      skin: "tension",
      appearance: "system",
      appearanceUpdatedAt: expect.any(String),
      appearanceTokenVersion: APPEARANCE_TOKEN_CONTRACT_VERSION,
    });
  });

  it("resolves a system change locally without manufacturing a server write", async () => {
    userRef.current = serverUser({
      skin: "relay",
      appearance: "system",
      appearanceUpdatedAt: "2026-08-23T10:00:00.000Z",
      appearanceTokenVersion: APPEARANCE_TOKEN_CONTRACT_VERSION,
    });
    const harness = createAdapter(
      preference({ source: "server", syncState: { status: "synced" } }),
    );
    renderBridge(harness.adapter);

    await waitFor(() => expect(screen.getByTestId("ready")).toHaveTextContent("true"));
    harness.emit({
      type: "system-appearance-changed",
      systemAppearance: "light",
    });

    await waitFor(() => expect(screen.getByTestId("resolved")).toHaveTextContent("light"));
    expect(harness.stored().updatedAt).toBe("2026-08-23T10:00:00.000Z");
    expect(mockUpdateMe).not.toHaveBeenCalled();
  });

  it("does not import another account's bootstrap cache", async () => {
    userRef.current = serverUser({ id: "account-b" });
    const harness = createAdapter(
      preference({
        skin: "field",
        source: "local",
        updatedAt: "2026-08-23T12:00:00.000Z",
        syncState: { status: "failed", errorClass: "network" },
      }),
    );
    harness.adapter.loadForAccount = () => null;
    harness.adapter.persistForAccount = (_accountId, next) => {
      harness.adapter.persist(next);
    };

    renderBridge(harness.adapter);

    await waitFor(() => expect(screen.getByTestId("ready")).toHaveTextContent("true"));
    await waitFor(() => expect(screen.getByTestId("skin")).toHaveTextContent("tension"));
    expect(mockUpdateMe).not.toHaveBeenCalled();
  });

  it("claims the default projection for a new account", async () => {
    userRef.current = serverUser({ id: "account-b" });
    const harness = createAdapter(null);
    const persistForAccount = vi.fn(async (_accountId, next) => {
      await harness.adapter.persist(next);
    });
    harness.adapter.loadForAccount = () => null;
    harness.adapter.persistForAccount = persistForAccount;

    renderBridge(harness.adapter);

    await waitFor(() =>
      expect(persistForAccount).toHaveBeenCalledWith(
        "account-b",
        expect.objectContaining({
          skin: "tension",
          requestedAppearance: "system",
        }),
      ),
    );
    expect(mockUpdateMe).not.toHaveBeenCalled();
  });

  it("drops a PATCH response after logout instead of restoring the old user", async () => {
    userRef.current = serverUser({ id: "account-a" });
    const harness = createAdapter(null);
    const response = deferred<User>();
    mockUpdateMe.mockReturnValueOnce(response.promise);
    const capture = vi.fn();
    const rendered = renderBridge(harness.adapter, <Probe />, capture);
    const user = userEvent.setup();

    await waitFor(() => expect(screen.getByTestId("ready")).toHaveTextContent("true"));
    await user.click(screen.getByRole("button", { name: "Field" }));
    await waitFor(() => expect(mockUpdateMe).toHaveBeenCalledTimes(1));

    userRef.current = null;
    rendered.rerender(
      <AppearanceSyncBridge
        adapter={harness.adapter}
        account={userRef.current}
        updateAccountAppearance={mockUpdateMe}
        refreshAccountAppearance={mockGetMe}
        capture={capture}
      >
        <Probe />
      </AppearanceSyncBridge>,
    );
    response.resolve(serverUser({ id: "account-a", skin: "field" }));

    await waitFor(() => expect(screen.getByTestId("skin")).toHaveTextContent("field"));
    expect(mockUpdateMe).toHaveBeenCalledTimes(1);
  });

  it("drops a reconnect GET response after logout", async () => {
    userRef.current = serverUser({ id: "account-a" });
    const harness = createAdapter(null);
    const response = deferred<User>();
    mockGetMe.mockReturnValueOnce(response.promise);
    const capture = vi.fn();
    const rendered = renderBridge(harness.adapter, <Probe />, capture);

    await waitFor(() => expect(screen.getByTestId("ready")).toHaveTextContent("true"));
    harness.emit({ type: "connectivity-changed", online: true });
    await waitFor(() => expect(mockGetMe).toHaveBeenCalledTimes(1));
    userRef.current = null;
    rendered.rerender(
      <AppearanceSyncBridge
        adapter={harness.adapter}
        account={userRef.current}
        updateAccountAppearance={mockUpdateMe}
        refreshAccountAppearance={mockGetMe}
        capture={capture}
      >
        <Probe />
      </AppearanceSyncBridge>,
    );
    response.resolve(serverUser({ id: "account-a" }));

    await waitFor(() => expect(mockGetMe).toHaveBeenCalledTimes(1));
    expect(mockGetMe).toHaveBeenCalledTimes(1);
  });

  it("keeps both choices when two clicks share the same wall-clock millisecond", async () => {
    userRef.current = serverUser();
    const harness = createAdapter(null);
    const first = deferred<User>();
    const second = deferred<User>();
    mockUpdateMe
      .mockReturnValueOnce(first.promise)
      .mockReturnValueOnce(second.promise);
    const now = vi
      .spyOn(Date, "now")
      .mockReturnValue(Date.parse("2026-08-23T12:00:00.000Z"));
    renderBridge(harness.adapter);
    const user = userEvent.setup();

    await waitFor(() => expect(screen.getByTestId("ready")).toHaveTextContent("true"));
    await user.click(screen.getByRole("button", { name: "Field" }));
    await user.click(screen.getByRole("button", { name: "Dark" }));
    await waitFor(() => expect(mockUpdateMe).toHaveBeenCalledTimes(2));

    const firstRequest = mockUpdateMe.mock.calls[0]![0] as UpdateMeRequest;
    const secondRequest = mockUpdateMe.mock.calls[1]![0] as UpdateMeRequest;
    expect(firstRequest.appearanceUpdatedAt).not.toBe(
      secondRequest.appearanceUpdatedAt,
    );
    second.resolve(
      serverUser({
        skin: secondRequest.skin,
        appearance: secondRequest.appearance,
        appearanceUpdatedAt: secondRequest.appearanceUpdatedAt,
        appearanceTokenVersion: secondRequest.appearanceTokenVersion,
      }),
    );
    await waitFor(() => expect(screen.getByTestId("sync")).toHaveTextContent("synced"));
    first.resolve(
      serverUser({
        skin: firstRequest.skin,
        appearance: firstRequest.appearance,
        appearanceUpdatedAt: firstRequest.appearanceUpdatedAt,
        appearanceTokenVersion: firstRequest.appearanceTokenVersion,
      }),
    );

    await waitFor(() => expect(screen.getByTestId("skin")).toHaveTextContent("field"));
    expect(screen.getByTestId("appearance")).toHaveTextContent("dark");
    now.mockRestore();
  });

  it("does not overwrite a future local contract or upload its fallback", async () => {
    const future = { ...preference(), tokenContractVersion: 2, sentinel: true };
    const harness = createAdapter(future);
    renderBridge(harness.adapter);

    await waitFor(() => expect(screen.getByTestId("ready")).toHaveTextContent("true"));
    expect(screen.getByTestId("skin")).toHaveTextContent("tension");
    expect(screen.getByTestId("sync")).toHaveTextContent("failed");
    expect(harness.stored()).toEqual(future);
    expect(mockUpdateMe).not.toHaveBeenCalled();
  });

  it("ignores a cross-tab cache event owned by another account", async () => {
    userRef.current = serverUser({ id: "account-a" });
    const harness = createAdapter(null);
    renderBridge(harness.adapter);

    await waitFor(() =>
      expect(screen.getByTestId("ready")).toHaveTextContent("true"),
    );
    harness.emit({
      type: "external-preferences-changed",
      accountId: "account-b",
      value: preference({ skin: "field" }),
    });

    expect(screen.getByTestId("skin")).toHaveTextContent("tension");
    expect(mockUpdateMe).not.toHaveBeenCalled();
  });

  it("surfaces an account cache read failure without uploading a fallback", async () => {
    userRef.current = serverUser({
      skin: "relay",
      appearance: "dark",
      appearanceUpdatedAt: "2026-08-23T11:00:00.000Z",
      appearanceTokenVersion: APPEARANCE_TOKEN_CONTRACT_VERSION,
    });
    const harness = createAdapter(null);
    harness.adapter.loadForAccount = () => {
      throw new DOMException("blocked", "SecurityError");
    };
    renderBridge(harness.adapter);

    await waitFor(() =>
      expect(screen.getByTestId("ready")).toHaveTextContent("true"),
    );
    await waitFor(() =>
      expect(screen.getByTestId("skin")).toHaveTextContent("relay"),
    );
    expect(screen.getByTestId("sync")).toHaveTextContent("failed");
    expect(mockUpdateMe).not.toHaveBeenCalled();
  });

  it("keeps a storage write failure visible while the server still settles", async () => {
    userRef.current = serverUser();
    const harness = createAdapter(null);
    harness.adapter.persist = () => {
      throw new DOMException("full", "QuotaExceededError");
    };
    mockUpdateMe.mockImplementation(async (request: UpdateMeRequest) =>
      serverUser({
        skin: request.skin,
        appearance: request.appearance,
        appearanceUpdatedAt: request.appearanceUpdatedAt,
        appearanceTokenVersion: request.appearanceTokenVersion,
      }),
    );
    renderBridge(harness.adapter);
    const user = userEvent.setup();

    await waitFor(() => expect(screen.getByTestId("ready")).toHaveTextContent("true"));
    await user.click(screen.getByRole("button", { name: "Field" }));

    await waitFor(() => expect(screen.getByTestId("skin")).toHaveTextContent("field"));
    await waitFor(() => expect(screen.getByTestId("sync")).toHaveTextContent("failed"));
    expect(mockUpdateMe).toHaveBeenCalledTimes(1);
  });

  it("discards a reconnect GET that finishes after a newer PATCH", async () => {
    userRef.current = serverUser();
    const harness = createAdapter(null);
    const getResponse = deferred<User>();
    mockGetMe.mockReturnValueOnce(getResponse.promise);
    mockUpdateMe.mockImplementation(async (request: UpdateMeRequest) =>
      serverUser({
        skin: request.skin,
        appearance: request.appearance,
        appearanceUpdatedAt: request.appearanceUpdatedAt,
        appearanceTokenVersion: request.appearanceTokenVersion,
      }),
    );
    renderBridge(harness.adapter);
    const user = userEvent.setup();

    await waitFor(() => expect(screen.getByTestId("ready")).toHaveTextContent("true"));
    harness.emit({ type: "connectivity-changed", online: true });
    await user.click(screen.getByRole("button", { name: "Field" }));
    await waitFor(() => expect(screen.getByTestId("sync")).toHaveTextContent("synced"));
    getResponse.resolve(serverUser());

    await waitFor(() => expect(screen.getByTestId("skin")).toHaveTextContent("field"));
    expect(mockUpdateMe).toHaveBeenCalledTimes(1);
  });
});
