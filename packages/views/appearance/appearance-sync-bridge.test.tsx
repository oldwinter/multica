import { useCallback, useRef, useState, type ReactNode } from "react";
import { flushSync } from "react-dom";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  APPEARANCE_PREFERENCES_VERSION,
  APPEARANCE_TOKEN_CONTRACT_VERSION,
  createDefaultAppearancePreferences,
  markAppearanceSyncFailed,
  type AppearanceAdapterEvent,
  type AppearanceEnvironment,
  type AppearancePreferenceAdapter,
  type AppearancePreferences,
  type AppearanceUndoReceipt,
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

function createAdapter(
  initial: unknown | null,
  environment: AppearanceEnvironment = DARK_ENVIRONMENT,
) {
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
    getEnvironment: () => environment,
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
    diagnostics,
    isReady,
    canRetry,
    canCopyDiagnostics,
    recoveryNoticePending,
    selectSkin,
    selectAppearance,
    undo,
    retry,
    reset,
  } = useAppearancePreferences();
  const receiptRef = useRef<AppearanceUndoReceipt | null>(null);
  const [undoOutcome, setUndoOutcome] = useState("none");
  return (
    <div>
      <output data-testid="ready">{String(isReady)}</output>
      <output data-testid="skin">{preferences.skin}</output>
      <output data-testid="appearance">{preferences.requestedAppearance}</output>
      <output data-testid="resolved">{preferences.resolvedAppearance}</output>
      <output data-testid="sync">{preferences.syncState.status}</output>
      <output data-testid="can-retry">{String(canRetry)}</output>
      <output data-testid="can-copy">{String(canCopyDiagnostics)}</output>
      <output data-testid="recovery-notice">
        {String(recoveryNoticePending)}
      </output>
      <output data-testid="recovered-fields">
        {diagnostics.recoveredFields.join(",")}
      </output>
      <output data-testid="undo-outcome">{undoOutcome}</output>
      <button
        type="button"
        onClick={() => {
          receiptRef.current = selectSkin("field");
        }}
      >
        Field
      </button>
      <button
        type="button"
        onClick={() => {
          receiptRef.current = selectAppearance("system");
        }}
      >
        System
      </button>
      <button
        type="button"
        onClick={() => {
          receiptRef.current = selectAppearance("dark");
        }}
      >
        Dark
      </button>
      <button
        type="button"
        onClick={() => {
          const receipt = receiptRef.current;
          if (!receipt) return;
          void undo(receipt).then(setUndoOutcome);
        }}
      >
        Undo
      </button>
      <button type="button" onClick={retry}>
        Retry
      </button>
      <button
        type="button"
        onClick={() => {
          receiptRef.current = reset();
        }}
      >
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

  it("keeps an authenticated offline choice pending until connectivity returns", async () => {
    userRef.current = serverUser();
    const harness = createAdapter(null, {
      ...DARK_ENVIRONMENT,
      online: false,
    });
    mockGetMe.mockResolvedValue(serverUser());
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

    await waitFor(() =>
      expect(screen.getByTestId("ready")).toHaveTextContent("true"),
    );
    await user.click(screen.getByRole("button", { name: "Field" }));

    await waitFor(() =>
      expect(screen.getByTestId("skin")).toHaveTextContent("field"),
    );
    expect(screen.getByTestId("sync")).toHaveTextContent("pending");
    expect(harness.stored().syncState).toEqual({ status: "pending" });
    expect(mockUpdateMe).not.toHaveBeenCalled();

    harness.emit({ type: "connectivity-changed", online: true });

    await waitFor(() => expect(mockUpdateMe).toHaveBeenCalledTimes(1));
    await waitFor(() =>
      expect(screen.getByTestId("sync")).toHaveTextContent("synced"),
    );
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

  it("undoes with the optimistic timestamp as a server compare condition", async () => {
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

    await waitFor(() =>
      expect(screen.getByTestId("ready")).toHaveTextContent("true"),
    );
    await user.click(screen.getByRole("button", { name: "Field" }));
    await waitFor(() =>
      expect(screen.getByTestId("sync")).toHaveTextContent("synced"),
    );
    const selected = mockUpdateMe.mock.calls[0]![0] as UpdateMeRequest;

    await user.click(screen.getByRole("button", { name: "Undo" }));

    await waitFor(() =>
      expect(screen.getByTestId("undo-outcome")).toHaveTextContent("applied"),
    );
    expect(screen.getByTestId("skin")).toHaveTextContent("tension");
    const undone = mockUpdateMe.mock.calls[1]![0] as UpdateMeRequest;
    expect(undone).toMatchObject({
      skin: "tension",
      appearance: "system",
      appearanceExpectedUpdatedAt: selected.appearanceUpdatedAt,
    });
    expect(undone.appearanceUpdatedAt).not.toBe(selected.appearanceUpdatedAt);
  });

  it("expires Undo after a newer cross-tab preference without another write", async () => {
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

    await waitFor(() =>
      expect(screen.getByTestId("ready")).toHaveTextContent("true"),
    );
    await user.click(screen.getByRole("button", { name: "Field" }));
    await waitFor(() =>
      expect(screen.getByTestId("sync")).toHaveTextContent("synced"),
    );
    harness.emit({
      type: "external-preferences-changed",
      accountId: "user-1",
      value: preference({
        skin: "relay",
        updatedAt: "2099-08-23T10:00:00.000Z",
      }),
    });
    await waitFor(() =>
      expect(screen.getByTestId("skin")).toHaveTextContent("relay"),
    );
    const writesBeforeUndo = mockUpdateMe.mock.calls.length;

    await user.click(screen.getByRole("button", { name: "Undo" }));

    await waitFor(() =>
      expect(screen.getByTestId("undo-outcome")).toHaveTextContent("expired"),
    );
    expect(screen.getByTestId("skin")).toHaveTextContent("relay");
    expect(mockUpdateMe).toHaveBeenCalledTimes(writesBeforeUndo);
  });

  it("accepts the server tuple when compare-before-write Undo is stale", async () => {
    userRef.current = serverUser();
    const harness = createAdapter(null);
    mockUpdateMe
      .mockImplementationOnce(async (request: UpdateMeRequest) =>
        serverUser({
          skin: request.skin,
          appearance: request.appearance,
          appearanceUpdatedAt: request.appearanceUpdatedAt,
          appearanceTokenVersion: request.appearanceTokenVersion,
        }),
      )
      .mockResolvedValueOnce(
        serverUser({
          skin: "relay",
          appearance: "dark",
          appearanceUpdatedAt: "2099-08-23T10:00:00.000Z",
          appearanceTokenVersion: APPEARANCE_TOKEN_CONTRACT_VERSION,
        }),
      );
    renderBridge(harness.adapter);
    const user = userEvent.setup();

    await waitFor(() =>
      expect(screen.getByTestId("ready")).toHaveTextContent("true"),
    );
    await user.click(screen.getByRole("button", { name: "Field" }));
    await waitFor(() =>
      expect(screen.getByTestId("sync")).toHaveTextContent("synced"),
    );
    await user.click(screen.getByRole("button", { name: "Undo" }));

    await waitFor(() =>
      expect(screen.getByTestId("undo-outcome")).toHaveTextContent("expired"),
    );
    expect(screen.getByTestId("skin")).toHaveTextContent("relay");
    expect(screen.getByTestId("appearance")).toHaveTextContent("dark");
    expect(harness.stored()).toMatchObject({
      skin: "relay",
      requestedAppearance: "dark",
      syncState: { status: "synced" },
    });
  });

  it("keeps a stale Undo response when the account store publishes it first", async () => {
    const harness = createAdapter(null);
    const capture = vi.fn();
    const now = vi
      .spyOn(Date, "now")
      .mockReturnValue(Date.parse("2026-08-23T12:00:00.000Z"));
    mockUpdateMe.mockImplementation(async (request: UpdateMeRequest) => {
      if (
        request.skin === undefined ||
        request.appearance === undefined ||
        request.appearanceUpdatedAt === undefined ||
        request.appearanceTokenVersion === undefined
      ) {
        throw new Error("appearance test request is incomplete");
      }
      if (request.appearanceExpectedUpdatedAt !== undefined) {
        return serverUser({
          skin: "relay",
          appearance: request.appearance,
          appearanceUpdatedAt: new Date(
            Date.parse(request.appearanceExpectedUpdatedAt) + 1,
          ).toISOString(),
          appearanceTokenVersion: request.appearanceTokenVersion,
        });
      }
      return serverUser({
        skin: request.skin,
        appearance: request.appearance,
        appearanceUpdatedAt: request.appearanceUpdatedAt,
        appearanceTokenVersion: request.appearanceTokenVersion,
      });
    });

    function StoreBackedBridge() {
      const [account, setAccount] = useState(() => serverUser());
      const updateAccountAppearance = useCallback(
        async (request: UpdateMeRequest): Promise<User> => {
          const updated: User = await mockUpdateMe(request);
          flushSync(() => setAccount(updated));
          return updated;
        },
        [],
      );
      return (
        <AppearanceSyncBridge
          adapter={harness.adapter}
          account={account}
          updateAccountAppearance={updateAccountAppearance}
          refreshAccountAppearance={mockGetMe}
          capture={capture}
        >
          <Probe />
        </AppearanceSyncBridge>
      );
    }

    render(<StoreBackedBridge />);
    const user = userEvent.setup();
    await waitFor(() =>
      expect(screen.getByTestId("ready")).toHaveTextContent("true"),
    );
    await user.click(screen.getByRole("button", { name: "Field" }));
    await waitFor(() =>
      expect(screen.getByTestId("sync")).toHaveTextContent("synced"),
    );

    await user.click(screen.getByRole("button", { name: "Undo" }));

    await waitFor(() =>
      expect(screen.getByTestId("undo-outcome")).toHaveTextContent("expired"),
    );
    expect(screen.getByTestId("skin")).toHaveTextContent("relay");
    expect(mockUpdateMe).toHaveBeenCalledTimes(2);
    now.mockRestore();
  });

  it("keeps the Undo compare condition when a failed write is retried", async () => {
    userRef.current = serverUser();
    const harness = createAdapter(null);
    mockUpdateMe.mockImplementationOnce(async (request: UpdateMeRequest) =>
      serverUser({
        skin: request.skin,
        appearance: request.appearance,
        appearanceUpdatedAt: request.appearanceUpdatedAt,
        appearanceTokenVersion: request.appearanceTokenVersion,
      }),
    );
    renderBridge(harness.adapter);
    const user = userEvent.setup();

    await waitFor(() =>
      expect(screen.getByTestId("ready")).toHaveTextContent("true"),
    );
    await user.click(screen.getByRole("button", { name: "Field" }));
    await waitFor(() =>
      expect(screen.getByTestId("sync")).toHaveTextContent("synced"),
    );
    const selected = mockUpdateMe.mock.calls[0]![0] as UpdateMeRequest;
    mockUpdateMe.mockRejectedValueOnce(new TypeError("offline"));

    await user.click(screen.getByRole("button", { name: "Undo" }));
    await waitFor(() =>
      expect(screen.getByTestId("sync")).toHaveTextContent("failed"),
    );
    mockUpdateMe.mockImplementationOnce(async (request: UpdateMeRequest) =>
      serverUser({
        skin: request.skin,
        appearance: request.appearance,
        appearanceUpdatedAt: request.appearanceUpdatedAt,
        appearanceTokenVersion: request.appearanceTokenVersion,
      }),
    );

    await user.click(screen.getByRole("button", { name: "Retry" }));

    await waitFor(() =>
      expect(screen.getByTestId("sync")).toHaveTextContent("synced"),
    );
    expect(mockUpdateMe).toHaveBeenLastCalledWith(
      expect.objectContaining({
        appearanceExpectedUpdatedAt: selected.appearanceUpdatedAt,
      }),
    );
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

  it("exposes bounded diagnostics only after two consecutive sync failures", async () => {
    userRef.current = serverUser();
    const harness = createAdapter(null);
    mockUpdateMe.mockRejectedValue(new TypeError("offline"));
    renderBridge(harness.adapter);
    const user = userEvent.setup();

    await waitFor(() =>
      expect(screen.getByTestId("ready")).toHaveTextContent("true"),
    );
    await user.click(screen.getByRole("button", { name: "Field" }));
    await waitFor(() =>
      expect(screen.getByTestId("sync")).toHaveTextContent("failed"),
    );
    expect(screen.getByTestId("can-retry")).toHaveTextContent("true");
    expect(screen.getByTestId("can-copy")).toHaveTextContent("false");

    await user.click(screen.getByRole("button", { name: "Retry" }));

    await waitFor(() =>
      expect(screen.getByTestId("can-copy")).toHaveTextContent("true"),
    );
  });

  it("reports recovered fields and exposes one recovery notice", async () => {
    const harness = createAdapter({
      ...preference(),
      skin: "not-a-skin",
    });
    renderBridge(harness.adapter);

    await waitFor(() =>
      expect(screen.getByTestId("ready")).toHaveTextContent("true"),
    );
    expect(screen.getByTestId("skin")).toHaveTextContent("tension");
    expect(screen.getByTestId("recovered-fields")).toHaveTextContent("skin");
    expect(screen.getByTestId("recovery-notice")).toHaveTextContent("true");
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

  it("fails closed on an incomplete server appearance tuple instead of waiting to sync", async () => {
    userRef.current = serverUser({
      skin: "field",
      appearance: null,
      appearanceUpdatedAt: "2026-08-23T11:00:00.000Z",
      appearanceTokenVersion: APPEARANCE_TOKEN_CONTRACT_VERSION,
    });
    const harness = createAdapter(
      preference({
        source: "local",
        syncState: { status: "pending" },
      }),
    );
    renderBridge(harness.adapter);

    await waitFor(() => expect(screen.getByTestId("ready")).toHaveTextContent("true"));
    await waitFor(() => expect(screen.getByTestId("sync")).toHaveTextContent("failed"));
    expect(screen.getByTestId("can-retry")).toHaveTextContent("true");
    expect(mockUpdateMe).not.toHaveBeenCalled();
  });

  it("retries an incomplete server tuple by writing the complete local choice", async () => {
    userRef.current = serverUser({
      skin: "field",
      appearance: null,
      appearanceUpdatedAt: "2026-08-23T11:00:00.000Z",
      appearanceTokenVersion: APPEARANCE_TOKEN_CONTRACT_VERSION,
    });
    const harness = createAdapter(
      preference({
        source: "local",
        syncState: { status: "pending" },
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

    await waitFor(() => expect(screen.getByTestId("can-retry")).toHaveTextContent("true"));
    await user.click(screen.getByRole("button", { name: "Retry" }));

    await waitFor(() => expect(screen.getByTestId("sync")).toHaveTextContent("synced"));
    expect(mockUpdateMe).toHaveBeenCalledWith({
      skin: "relay",
      appearance: "system",
      appearanceUpdatedAt: "2026-08-23T10:00:00.000Z",
      appearanceTokenVersion: APPEARANCE_TOKEN_CONTRACT_VERSION,
    });
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

  it("heals a failed default cache when the account has no server tuple", async () => {
    userRef.current = serverUser({ id: "account-b" });
    const failedDefault = markAppearanceSyncFailed(
      createDefaultAppearancePreferences("dark"),
      "server",
    );
    const harness = createAdapter(failedDefault);
    const persistForAccount = vi.fn(async (_accountId, next) => {
      await harness.adapter.persist(next);
    });
    harness.adapter.loadForAccount = () => failedDefault;
    harness.adapter.persistForAccount = persistForAccount;

    renderBridge(harness.adapter);

    await waitFor(() =>
      expect(screen.getByTestId("sync")).toHaveTextContent("local-only"),
    );
    await waitFor(() =>
      expect(persistForAccount).toHaveBeenCalledWith(
        "account-b",
        expect.objectContaining({
          source: "default",
          syncState: { status: "local-only" },
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
    expect(screen.getByTestId("can-retry")).toHaveTextContent("false");
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
