import type { ReactNode } from "react";
import {
  describe,
  it,
  expect,
  beforeAll,
  beforeEach,
  afterAll,
  afterEach,
  vi,
} from "vitest";
import { render, screen, act, cleanup, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { I18nProvider } from "@multica/core/i18n/react";
import enCommon from "../../locales/en/common.json";
import enAuth from "../../locales/en/auth.json";
import enSettings from "../../locales/en/settings.json";

type AppearanceMockState = {
  preferences: {
    skin: string;
    requestedAppearance: string;
    resolvedAppearance: string;
    source: string;
    syncState: { status: string; errorClass?: string };
  };
  diagnostics: {
    preferenceVersion: number;
    tokenContractVersion: number;
    skin: string;
    requestedAppearance: string;
    resolvedAppearance: string;
    preferenceSource: string;
    adapterSource: string;
    syncStatus: string;
    lastSyncErrorClass: string | null;
    reducedMotion: boolean;
    forcedColors: boolean;
    recoveredFields: string[];
  };
  canRetry: boolean;
  canCopyDiagnostics: boolean;
  recoveryNoticePending: boolean;
};

const navigationState = vi.hoisted(() => ({ search: "", replace: vi.fn() }));
vi.mock("../../navigation", () => ({
  useNavigation: () => ({
    pathname: "/acme/settings",
    searchParams: new URLSearchParams(navigationState.search),
    replace: navigationState.replace,
  }),
}));
const mockPersist = vi.hoisted(() => vi.fn());
const mockUpdateMe = vi.hoisted(() => vi.fn());
const mockReload = vi.hoisted(() => vi.fn());
const mockToastWarning = vi.hoisted(() => vi.fn());
const mockToastError = vi.hoisted(() => vi.fn());
const mockToastSuccess = vi.hoisted(() => vi.fn());
const mockSetTheme = vi.hoisted(() => vi.fn());
const mockSetSkin = vi.hoisted(() => vi.fn());
const mockResetAppearance = vi.hoisted(() => vi.fn());
const mockRetryAppearance = vi.hoisted(() => vi.fn());
const mockUndoAppearance = vi.hoisted(() => vi.fn());
const mockAcknowledgeRecovery = vi.hoisted(() => vi.fn());
const mockCopyText = vi.hoisted(() => vi.fn());
const mockSetUser = vi.hoisted(() => vi.fn());
const appearanceRef = vi.hoisted(() => ({
  current: {
    preferences: {
      skin: "tension",
      requestedAppearance: "system",
      resolvedAppearance: "light",
      source: "local",
      syncState: { status: "local-only" },
    },
    diagnostics: {
      preferenceVersion: 1,
      tokenContractVersion: 1,
      skin: "tension",
      requestedAppearance: "system",
      resolvedAppearance: "light",
      preferenceSource: "local",
      adapterSource: "web",
      syncStatus: "local-only",
      lastSyncErrorClass: null,
      reducedMotion: false,
      forcedColors: false,
      recoveredFields: [] as string[],
    },
    canRetry: false,
    canCopyDiagnostics: false,
    recoveryNoticePending: false,
  } as AppearanceMockState,
}));
const userRef = vi.hoisted(() => ({
  current: null as { id: string; timezone?: string | null } | null,
}));

vi.mock("@multica/ui/components/common/theme-provider", () => ({
  SKIN_IDS: ["tension", "relay", "field"],
  useTheme: () => ({ theme: "light", setTheme: mockSetTheme }),
  useSkin: () => ({ skin: "tension", setSkin: mockSetSkin }),
}));

vi.mock("../../appearance", async () => {
  const actual = await vi.importActual<typeof import("../../appearance")>(
    "../../appearance",
  );
  return {
    ...actual,
    useAppearancePreferences: () => ({
      ...appearanceRef.current,
      selectSkin: mockSetSkin,
      selectAppearance: mockSetTheme,
      reset: mockResetAppearance,
      undo: mockUndoAppearance,
      retry: mockRetryAppearance,
      acknowledgeRecoveryNotice: mockAcknowledgeRecovery,
    }),
  };
});

vi.mock("@multica/ui/lib/clipboard", () => ({
  copyText: mockCopyText,
}));

vi.mock("@multica/core/i18n/react", async () => {
  const actual = await vi.importActual<
    typeof import("@multica/core/i18n/react")
  >("@multica/core/i18n/react");
  return {
    ...actual,
    useLocaleAdapter: () => ({
      persist: mockPersist,
      getUserChoice: () => null,
      getSystemPreferences: () => [],
    }),
  };
});

// The chat store is registered by the app shell; a callable stand-in with the
// Zustand shape is enough for the one toggle this page owns.
const chatState = vi.hoisted(() => ({
  floatingChatEnabled: true,
  setFloatingChatEnabled: vi.fn(),
}));
vi.mock("@multica/core/chat", () => ({
  useChatStore: Object.assign(
    (selector: (state: typeof chatState) => unknown) => selector(chatState),
    { getState: () => chatState },
  ),
}));

vi.mock("@multica/core/paths", () => ({
  useCurrentWorkspace: () => ({ id: "ws-1", name: "Acme" }),
}));

vi.mock("@multica/core/api", () => ({
  api: { updateMe: mockUpdateMe },
}));

vi.mock("sonner", () => ({
  toast: {
    warning: mockToastWarning,
    error: mockToastError,
    success: mockToastSuccess,
  },
}));

vi.mock("@multica/core/auth", async () => {
  const actual =
    await vi.importActual<typeof import("@multica/core/auth")>(
      "@multica/core/auth",
    );
  type AuthState = {
    user: typeof userRef.current;
    setUser: typeof mockSetUser;
  };
  const state = (): AuthState => ({
    user: userRef.current,
    setUser: mockSetUser,
  });
  const useAuthStore = Object.assign(
    (sel?: (s: AuthState) => unknown) => (sel ? sel(state()) : state()),
    { getState: state },
  );
  return { ...actual, useAuthStore };
});

import { PreferencesTab } from "./preferences-tab";
import { useCommentComposerStore } from "@multica/core/issues/stores";
import { useIssueOpeningStore } from "@multica/core/issues/stores/issue-opening-store";
import {
  DEFAULT_MANUAL_CREATE_FIELDS,
  DEFAULT_QUICK_CREATE_FIELDS,
  useIssueCreateSettingsStore,
} from "@multica/core/issues/stores/issue-create-settings-store";


const TEST_RESOURCES = {
  en: { common: enCommon, auth: enAuth, settings: enSettings },
};

const APPEARANCE_RECEIPT = {
  previous: {
    version: 1 as const,
    tokenContractVersion: 1 as const,
    skin: "tension" as const,
    requestedAppearance: "system" as const,
    resolvedAppearance: "light" as const,
    source: "local" as const,
    updatedAt: "2026-08-26T10:00:00.000Z",
    syncState: { status: "local-only" as const },
  },
  expectedUpdatedAt: "2026-08-26T10:01:00.000Z",
};

beforeEach(() => {
  appearanceRef.current = {
    preferences: {
      skin: "tension",
      requestedAppearance: "system",
      resolvedAppearance: "light",
      source: "local",
      syncState: { status: "local-only" },
    },
    diagnostics: {
      preferenceVersion: 1,
      tokenContractVersion: 1,
      skin: "tension",
      requestedAppearance: "system",
      resolvedAppearance: "light",
      preferenceSource: "local",
      adapterSource: "web",
      syncStatus: "local-only",
      lastSyncErrorClass: null,
      reducedMotion: false,
      forcedColors: false,
      recoveredFields: [],
    },
    canRetry: false,
    canCopyDiagnostics: false,
    recoveryNoticePending: false,
  };
  mockSetSkin.mockReturnValue(APPEARANCE_RECEIPT);
  mockSetTheme.mockReturnValue(APPEARANCE_RECEIPT);
  mockResetAppearance.mockReturnValue(APPEARANCE_RECEIPT);
  mockUndoAppearance.mockResolvedValue("applied");
  mockCopyText.mockResolvedValue(true);
});

function I18nWrapper({ children }: { children: ReactNode }) {
  return (
    <I18nProvider locale="en" resources={TEST_RESOURCES}>
      {children}
    </I18nProvider>
  );
}
describe("PreferencesTab — Language switcher", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    userRef.current = null;
    vi.useFakeTimers({ shouldAdvanceTime: true });
    Object.defineProperty(window, "location", {
      writable: true,
      configurable: true,
      value: { reload: mockReload },
    });
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  async function pickLanguage(
    user: ReturnType<typeof userEvent.setup>,
    name: string,
  ) {
    await user.click(screen.getByRole("combobox", { name: "Language" }));
    await user.click(await screen.findByRole("option", { name }));
  }

  it("keeps stacked skins clear of the chat launcher", () => {
    render(<PreferencesTab />, { wrapper: I18nWrapper });

    expect(screen.getByRole("radiogroup", { name: "Skin" })).toHaveClass(
      "pe-chat-launcher",
      "@xl:pe-0",
    );
  });

  it("keeps the language and region card clear of the chat launcher on compact screens", () => {
    render(<PreferencesTab />, { wrapper: I18nWrapper });

    expect(screen.getByRole("heading", { name: "Language & region" }).closest("section")).toHaveClass(
      "max-md:pe-chat-launcher",
    );
  });

  it("does nothing when clicking the current locale", async () => {
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime });
    render(<PreferencesTab />, { wrapper: I18nWrapper });

    await pickLanguage(user, "English");

    expect(mockPersist).not.toHaveBeenCalled();
    expect(mockUpdateMe).not.toHaveBeenCalled();
    expect(mockReload).not.toHaveBeenCalled();
  });
  it("shows a confirmation toast when the appearance is saved locally", async () => {
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime });
    render(<PreferencesTab />, { wrapper: I18nWrapper });

    await user.click(screen.getByRole("radio", { name: "Dark" }));

    expect(mockSetTheme).toHaveBeenCalledWith("dark");
    expect(mockToastSuccess).toHaveBeenCalledTimes(1);
    expect(mockToastSuccess).toHaveBeenCalledWith(
      "Changes saved",
      expect.objectContaining({
        id: "settings-appearance-save",
        action: expect.objectContaining({ label: "Undo" }),
      }),
    );

    const toastOptions = mockToastSuccess.mock.calls[0]![1] as {
      action: { onClick: () => void };
    };
    toastOptions.action.onClick();

    await waitFor(() =>
      expect(mockUndoAppearance).toHaveBeenCalledWith(APPEARANCE_RECEIPT),
    );
    expect(mockToastSuccess).toHaveBeenLastCalledWith(
      "Previous appearance restored",
      { id: "settings-appearance-undo" },
    );
  });

  it("persists a named skin independently from appearance", async () => {
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime });
    render(<PreferencesTab />, { wrapper: I18nWrapper });

    await user.click(screen.getByRole("radio", { name: /Relay/ }));

    expect(mockSetSkin).toHaveBeenCalledWith("relay");
    expect(mockSetTheme).not.toHaveBeenCalled();
    expect(mockToastSuccess).toHaveBeenCalledTimes(1);
  });

  it("supports arrow-key navigation between named skins", async () => {
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime });
    render(<PreferencesTab />, { wrapper: I18nWrapper });

    screen.getByRole("radio", { name: /Tension/ }).focus();
    await user.keyboard("{ArrowRight}");

    expect(mockSetSkin).toHaveBeenCalledWith("relay");
    expect(screen.getByRole("radio", { name: /Relay/ })).toHaveFocus();
  });

  it("renders the same semantic state fixture for every named skin", () => {
    render(<PreferencesTab />, { wrapper: I18nWrapper });

    const fixtures = Array.from(
      document.querySelectorAll<HTMLElement>("[data-appearance-fixture]"),
    );
    expect(fixtures).toHaveLength(3);
    expect(fixtures.map((fixture) => fixture.dataset.skin)).toEqual([
      "tension",
      "relay",
      "field",
    ]);
    expect(
      fixtures.every(
        (fixture) =>
          fixture.dataset.appearanceMode === "light" &&
          fixture.querySelector('[data-fixture-role="form-control"]') &&
          fixture.querySelector('[data-fixture-role="destructive"]') &&
          fixture.querySelector('[data-fixture-role="code-editor"]'),
      ),
    ).toBe(true);
    expect(fixtures[0]).toHaveTextContent("Review ready");
    expect(fixtures[0]).toHaveTextContent("Selected task");
  });

  it("names the exact defaults before reset and only resets on confirm", async () => {
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime });
    render(<PreferencesTab />, { wrapper: I18nWrapper });

    await user.click(
      screen.getByRole("button", { name: "Reset appearance" }),
    );

    expect(screen.getByRole("alertdialog")).toBeInTheDocument();
    expect(screen.getByText(/skin to Tension/i)).toBeInTheDocument();
    expect(screen.getByText(/color mode to System/i)).toBeInTheDocument();
    expect(mockResetAppearance).not.toHaveBeenCalled();

    await user.click(screen.getByRole("button", { name: "Cancel" }));
    expect(mockResetAppearance).not.toHaveBeenCalled();
    await user.click(
      screen.getByRole("button", { name: "Reset appearance" }),
    );
    await user.click(
      screen.getByRole("button", { name: "Reset to defaults" }),
    );

    expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument();
    expect(mockResetAppearance).toHaveBeenCalledTimes(1);
    expect(mockToastSuccess).toHaveBeenCalledWith(
      "Changes saved",
      expect.objectContaining({ id: "settings-appearance-save" }),
    );
  });

  it("explains an expired Undo without claiming restoration", async () => {
    mockUndoAppearance.mockResolvedValueOnce("expired");
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime });
    render(<PreferencesTab />, { wrapper: I18nWrapper });
    await user.click(screen.getByRole("radio", { name: "Dark" }));
    const toastOptions = mockToastSuccess.mock.calls[0]![1] as {
      action: { onClick: () => void };
    };

    toastOptions.action.onClick();

    await waitFor(() =>
      expect(mockToastWarning).toHaveBeenCalledWith(
        "Undo expired because appearance changed elsewhere",
        { id: "settings-appearance-undo" },
      ),
    );
  });

  it("reveals and copies only the bounded diagnostic snapshot", async () => {
    appearanceRef.current.preferences.syncState = {
      status: "failed",
      errorClass: "network",
    };
    appearanceRef.current.diagnostics.syncStatus = "failed";
    appearanceRef.current.diagnostics.lastSyncErrorClass = "network";
    appearanceRef.current.canCopyDiagnostics = true;
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime });
    render(<PreferencesTab />, { wrapper: I18nWrapper });

    await user.click(
      screen.getByRole("button", { name: "Copy diagnostics" }),
    );

    expect(mockCopyText).toHaveBeenCalledTimes(1);
    const payload = mockCopyText.mock.calls[0]![0] as string;
    expect(JSON.parse(payload)).toEqual(appearanceRef.current.diagnostics);
    expect(payload).not.toMatch(/workspace|route|email|updatedAt/i);
    expect(mockToastSuccess).toHaveBeenCalledWith(
      "Appearance diagnostics copied",
      { id: "settings-appearance-diagnostics" },
    );
  });

  it("shows Retry only when the current failure can make progress", async () => {
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime });
    appearanceRef.current.preferences.syncState = {
      status: "failed",
      errorClass: "conflict",
    };
    const rendered = render(<PreferencesTab />, { wrapper: I18nWrapper });

    expect(
      screen.queryByRole("button", { name: "Retry sync" }),
    ).not.toBeInTheDocument();

    appearanceRef.current.preferences.syncState = {
      status: "failed",
      errorClass: "network",
    };
    appearanceRef.current.canRetry = true;
    rendered.rerender(<PreferencesTab />);
    await user.click(screen.getByRole("button", { name: "Retry sync" }));

    expect(mockRetryAppearance).toHaveBeenCalledTimes(1);
  });

  it("notifies once when an invalid appearance value was recovered", async () => {
    appearanceRef.current.recoveryNoticePending = true;
    appearanceRef.current.diagnostics.recoveredFields = ["skin"];
    const rendered = render(<PreferencesTab />, { wrapper: I18nWrapper });

    await waitFor(() =>
      expect(mockToastWarning).toHaveBeenCalledWith(
        "An invalid appearance value was replaced with a safe default",
        { id: "settings-appearance-recovered" },
      ),
    );
    expect(mockAcknowledgeRecovery).toHaveBeenCalledTimes(1);

    appearanceRef.current.recoveryNoticePending = false;
    rendered.rerender(<PreferencesTab />);
    expect(mockToastWarning).toHaveBeenCalledTimes(1);
  });

  it.each([
    { name: "한국어", locale: "ko" },
    { name: "日本語", locale: "ja" },
  ])("when not logged in: persists $locale and reloads, no PATCH", async ({ name, locale }) => {
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime });
    render(<PreferencesTab />, { wrapper: I18nWrapper });

    await pickLanguage(user, name);

    expect(mockPersist).toHaveBeenCalledWith(locale);
    expect(mockUpdateMe).not.toHaveBeenCalled();
    // The reload is the confirmation; no success toast precedes it.
    expect(mockReload).toHaveBeenCalledTimes(1);
    expect(mockToastSuccess).not.toHaveBeenCalled();
    expect(mockToastWarning).not.toHaveBeenCalled();
  });

  it("cancels the delayed locale reload when Settings unmounts", async () => {
    userRef.current = { id: "user-1" };
    mockUpdateMe.mockRejectedValueOnce(new Error("network"));
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime });
    const rendered = render(<PreferencesTab />, { wrapper: I18nWrapper });

    await pickLanguage(user, "한국어");
    await waitFor(() => expect(mockToastWarning).toHaveBeenCalledTimes(1));
    rendered.unmount();
    act(() => vi.advanceTimersByTime(2500));

    expect(mockReload).not.toHaveBeenCalled();
  });

  it.each([
    { name: "中文", locale: "zh-Hans" },
    { name: "Français", locale: "fr" },
  ])("when logged in: saves $locale before reloading", async ({ name, locale }) => {
    userRef.current = { id: "user-1" };
    mockUpdateMe.mockResolvedValueOnce({});
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime });
    render(<PreferencesTab />, { wrapper: I18nWrapper });

    await pickLanguage(user, name);

    expect(mockPersist).toHaveBeenCalledWith(locale);
    expect(mockUpdateMe).toHaveBeenCalledWith({ language: locale });
    expect(mockToastWarning).not.toHaveBeenCalled();
    await waitFor(() => expect(mockReload).toHaveBeenCalledTimes(1));
  });

  it("when logged in + PATCH fails: shows toast and delays reload by 2.5s", async () => {
    userRef.current = { id: "user-1" };
    mockUpdateMe.mockRejectedValueOnce(new Error("network"));
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime });
    render(<PreferencesTab />, { wrapper: I18nWrapper });

    await pickLanguage(user, "中文");

    // Local persist still happened so the reload below sees the new locale.
    expect(mockPersist).toHaveBeenCalledWith("zh-Hans");
    expect(mockUpdateMe).toHaveBeenCalledWith({ language: "zh-Hans" });
    // Toast surfaced the sync failure.
    expect(mockToastWarning).toHaveBeenCalledTimes(1);
    // Reload deferred so the toast is visible.
    expect(mockReload).not.toHaveBeenCalled();

    act(() => {
      vi.advanceTimersByTime(2500);
    });
    expect(mockReload).toHaveBeenCalledTimes(1);
  });
});

describe("PreferencesTab — Time zone", () => {
  // Shrink the picker to the curated COMMON_TIMEZONES fallback so the list
  // stays small enough for fast userEvent traversal (MUL-4427). Everything
  // these tests pick — Asia/Tokyo and the "(browser)" entry — exists in the
  // fallback list too.
  const intlWithValues = Intl as typeof Intl & {
    supportedValuesOf?: (key: "timeZone") => string[];
  };
  const realSupportedValuesOf = intlWithValues.supportedValuesOf;
  beforeAll(() => {
    intlWithValues.supportedValuesOf = () => [];
  });
  afterAll(() => {
    intlWithValues.supportedValuesOf = realSupportedValuesOf;
  });

  beforeEach(() => {
    vi.clearAllMocks();
    userRef.current = null;
  });

  afterEach(() => {
    cleanup();
  });

  async function pickTimezone(
    user: ReturnType<typeof userEvent.setup>,
    search: string,
    name: RegExp | string,
  ) {
    await user.click(screen.getByRole("button", { name: "Time zone" }));
    await user.type(
      await screen.findByPlaceholderText("Search time zones..."),
      search,
    );
    await user.click(await screen.findByRole("option", { name }));
  }

  it("renders the stored time zone in the trigger", () => {
    userRef.current = { id: "user-1", timezone: "Asia/Shanghai" };
    render(<PreferencesTab />, { wrapper: I18nWrapper });

    expect(
      screen.getByRole("button", { name: "Time zone" }).textContent,
    ).toContain("Asia/Shanghai");
  });

  it("searches the list and saves the chosen zone to the account", async () => {
    userRef.current = { id: "user-1", timezone: "Asia/Shanghai" };
    const updatedUser = { id: "user-1", timezone: "Asia/Tokyo" };
    mockUpdateMe.mockResolvedValueOnce(updatedUser);
    const user = userEvent.setup();
    render(<PreferencesTab />, { wrapper: I18nWrapper });

    await pickTimezone(user, "tokyo", /Asia\/Tokyo/);

    await waitFor(() => {
      expect(mockUpdateMe).toHaveBeenCalledWith({ timezone: "Asia/Tokyo" });
      expect(mockSetUser).toHaveBeenCalledWith(updatedUser);
    });
    expect(mockToastSuccess).not.toHaveBeenCalled();
  });

  it("surfaces a toast when the PATCH fails", async () => {
    userRef.current = { id: "user-1", timezone: "Asia/Shanghai" };
    mockUpdateMe.mockRejectedValueOnce(new Error("network down"));
    const user = userEvent.setup();
    render(<PreferencesTab />, { wrapper: I18nWrapper });

    await pickTimezone(user, "tokyo", /Asia\/Tokyo/);

    await waitFor(() => {
      expect(mockUpdateMe).toHaveBeenCalledWith({ timezone: "Asia/Tokyo" });
      expect(mockToastError).toHaveBeenCalledTimes(1);
    });
    expect(mockSetUser).not.toHaveBeenCalled();
  });

  it("clearing the preference sends an empty-string timezone", async () => {
    userRef.current = { id: "user-1", timezone: "Asia/Shanghai" };
    const clearedUser = { id: "user-1", timezone: null };
    mockUpdateMe.mockResolvedValueOnce(clearedUser);
    const user = userEvent.setup();
    render(<PreferencesTab />, { wrapper: I18nWrapper });

    // The "(browser)" entry resets the preference to NULL; the wire payload
    // is an empty string the backend translates to NULL.
    await pickTimezone(user, "browser", /browser/i);

    await waitFor(() => {
      expect(mockUpdateMe).toHaveBeenCalledWith({ timezone: "" });
      // The PATCH response (timezone: null) is pushed into the auth store
      // so the picker switches back to "(browser)" without a refetch.
      expect(mockSetUser).toHaveBeenCalledWith(clearedUser);
    });
  });
});

describe("PreferencesTab — Comments & chat", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    userRef.current = null;
    useCommentComposerStore.setState({ runningAgentReply: "steer", sticky: true });
    chatState.floatingChatEnabled = true;
  });

  afterEach(() => {
    cleanup();
  });

  it("defaults to adding the reply to the current run and saves starting after it", async () => {
    const user = userEvent.setup();
    render(<PreferencesTab />, { wrapper: I18nWrapper });

    const select = screen.getByRole("combobox", { name: "When replying to a running agent" });
    expect(select).toHaveTextContent("Add to current run");

    await user.click(select);
    await user.click(await screen.findByRole("option", { name: "Start after this run" }));

    expect(useCommentComposerStore.getState().runningAgentReply).toBe("after_run");
    expect(select).toHaveTextContent("Start after this run");
    expect(mockToastSuccess).not.toHaveBeenCalled();
  });

  it("toggles the sticky comment bar and the floating chat silently", async () => {
    const user = userEvent.setup();
    render(<PreferencesTab />, { wrapper: I18nWrapper });

    const sticky = screen.getByRole("switch", { name: "Pin comment bar to bottom" });
    expect(sticky).toHaveAttribute("aria-checked", "true");
    await user.click(sticky);
    expect(useCommentComposerStore.getState().sticky).toBe(false);

    await user.click(screen.getByRole("switch", { name: enSettings.chat.floating_label }));
    expect(chatState.setFloatingChatEnabled).toHaveBeenCalledWith(false);
    expect(mockToastSuccess).not.toHaveBeenCalled();
  });
});

describe("PreferencesTab — Create-issue fields", () => {
  function resetStore() {
    useIssueCreateSettingsStore.setState({
      quickCreateFields: DEFAULT_QUICK_CREATE_FIELDS,
      manualCreateFields: DEFAULT_MANUAL_CREATE_FIELDS,
    });
  }
  beforeEach(resetStore);
  afterEach(() => {
    cleanup();
    resetStore();
  });

  it("shows one row per field with a column per create dialog", () => {
    render(<PreferencesTab />, { wrapper: I18nWrapper });
    const table = screen.getByRole("table");
    // 7 fields; quick create supports 3 of them, manual create all 7.
    expect(within(table).getAllByRole("checkbox")).toHaveLength(10);
    expect(
      within(table).getByRole("checkbox", { name: "Project · Create with agent" }),
    ).toBeChecked();
    expect(
      within(table).getByRole("checkbox", { name: "Due date · Manual create" }),
    ).not.toBeChecked();
    // Unsupported cells explain themselves instead of rendering a dead box.
    expect(
      within(table).getAllByLabelText(enSettings.preferences.issue_fields.unsupported),
    ).toHaveLength(4);
  });

  it("persists each dialog's fields independently", async () => {
    const user = userEvent.setup();
    render(<PreferencesTab />, { wrapper: I18nWrapper });

    await user.click(screen.getByRole("checkbox", { name: "Priority · Create with agent" }));
    await user.click(screen.getByRole("checkbox", { name: "Labels · Manual create" }));

    expect(useIssueCreateSettingsStore.getState().quickCreateFields).toEqual([
      "project",
      "priority",
    ]);
    expect(useIssueCreateSettingsStore.getState().manualCreateFields).toEqual([
      "status",
      "priority",
      "assignee",
      "project",
    ]);
  });
});

describe("PreferencesTab — Scope", () => {
  afterEach(() => {
    cleanup();
  });

  it("labels where each group of settings is stored", () => {
    render(<PreferencesTab />, { wrapper: I18nWrapper });
    expect(screen.getByText("Account · synced")).toBeInTheDocument();
    // Opening issues, comments & chat. Appearance reports its own sync state.
    expect(screen.getAllByText("This device only")).toHaveLength(2);
  });

  it("switches what clicking a card or row opens, and says what Shift does", async () => {
    useIssueOpeningStore.setState({ openMode: "page" });
    const user = userEvent.setup();
    render(<PreferencesTab />, { wrapper: I18nWrapper });
    const group = screen.getByRole("group", { name: "Clicking a card or row opens" });
    const fullPage = within(group).getByRole("button", { name: "Full page" });
    const preview = within(group).getByRole("button", { name: "Side preview" });
    expect(fullPage).toHaveAttribute("aria-pressed", "true");
    expect(screen.getByText("Shift+Click opens the other one.")).toBeInTheDocument();

    await user.click(preview);
    expect(useIssueOpeningStore.getState().openMode).toBe("peek");
    expect(preview).toHaveAttribute("aria-pressed", "true");
  });
});
