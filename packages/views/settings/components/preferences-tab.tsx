"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import {
  Check,
  ChevronsUpDown,
  Copy,
  Monitor,
  Moon,
  RefreshCw,
  RotateCcw,
  Sun,
} from "lucide-react";
import { toast } from "sonner";
import { SemanticAppearanceFixture } from "@multica/ui/components/common/semantic-appearance-fixture";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@multica/ui/components/ui/alert-dialog";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@multica/ui/components/ui/select";
import { Switch } from "@multica/ui/components/ui/switch";
import { Button } from "@multica/ui/components/ui/button";
import {
  RadioGroup,
  RadioGroupItem,
} from "@multica/ui/components/ui/radio-group";
import { Checkbox } from "@multica/ui/components/ui/checkbox";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@multica/ui/components/ui/popover";
import {
  Command,
  CommandEmpty,
  CommandInput,
  CommandItem,
  CommandList,
} from "@multica/ui/components/ui/command";
import {
  serializeAppearanceDiagnostics,
  type AppearanceUndoReceipt,
} from "@multica/core/appearance";
import { cn } from "@multica/ui/lib/utils";
import { copyText } from "@multica/ui/lib/clipboard";
import { type SupportedLocale } from "@multica/core/i18n";
import { useLocaleAdapter } from "@multica/core/i18n/react";
import { useAuthStore } from "@multica/core/auth";
import { useChatStore } from "@multica/core/chat";
import {
  useCommentComposerStore,
  type RunningAgentReply,
} from "@multica/core/issues/stores";
import {
  useIssueOpeningStore,
  type IssueOpenMode,
} from "@multica/core/issues/stores/issue-opening-store";
import {
  MANUAL_CREATE_FIELDS,
  QUICK_CREATE_FIELDS,
  useIssueCreateSettingsStore,
  type ManualCreateField,
  type QuickCreateField,
} from "@multica/core/issues/stores/issue-create-settings-store";
import { api } from "@multica/core/api";
import { browserTimezone, timezoneOptions } from "../../common/timezone-select";
import { SegmentedToggle } from "../../common/segmented-toggle";
import { useT } from "../../i18n";
import {
  APPEARANCE_OPTIONS,
  SKIN_OPTIONS,
  getAppearanceSyncMessage,
  isRequestedAppearance,
  isSkinId,
  useAppearancePreferences,
} from "../../appearance";
import {
  SettingsCard,
  SettingsRow,
  SettingsSection,
  SettingsTab,
} from "./settings-layout";
import { resolveSettingsLocale } from "./preferences-locale";

/**
 * Preferences on one page. The settings here are stored in three different
 * places — the account (language, timezone), this device (theme, composer,
 * chat) and this device for the current workspace (create-dialog fields) —
 * so each section carries a scope badge instead of splitting them into tabs.
 * Changes apply immediately; only failures are announced.
 */
export function PreferencesTab() {
  const { t } = useT("settings");
  return (
    <SettingsTab title={t(($) => $.page.tabs.preferences)}>
      <SettingsSection
        title={t(($) => $.preferences.region_title)}
        scope="account"
        anchor="region"
        className="max-md:pe-chat-launcher"
      >
        <SettingsCard>
          <LanguageRow />
          <TimezoneRow />
        </SettingsCard>
      </SettingsSection>

      <SettingsSection
        title={t(($) => $.preferences.appearance_title)}
        description={t(($) => $.preferences.appearance_hint)}
        anchor="appearance"
        className="@container"
      >
        <AppearancePreferences />
      </SettingsSection>

      <SettingsSection
        title={t(($) => $.preferences.comments_title)}
        scope="device"
        anchor="comments"
      >
        <SettingsCard>
          <StickyCommentBarRow />
          <RunningAgentReplyRow />
          <FloatingChatRow />
        </SettingsCard>
      </SettingsSection>

      <SettingsSection
        title={t(($) => $.issue.opening.title)}
        scope="device"
        anchor="issue-opening"
      >
        <SettingsCard>
          <IssueOpeningRow />
        </SettingsCard>
      </SettingsSection>

      <SettingsSection
        title={t(($) => $.preferences.issue_fields_title)}
        description={t(($) => $.issue.description)}
        scope="device-workspace"
        anchor="issue"
      >
        <IssueFieldsMatrix />
      </SettingsSection>
    </SettingsTab>
  );
}

function AppearancePreferences() {
  const {
    preferences,
    diagnostics,
    canRetry,
    canCopyDiagnostics,
    recoveryNoticePending,
    selectSkin,
    selectAppearance,
    reset: resetAppearance,
    undo: undoAppearance,
    retry: retryAppearanceSync,
    acknowledgeRecoveryNotice,
  } = useAppearancePreferences();
  const [resetDialogOpen, setResetDialogOpen] = useState(false);
  const skin = preferences.skin;
  const theme = preferences.requestedAppearance;
  const { t } = useT("settings");

  useEffect(() => {
    if (!recoveryNoticePending) return;
    toast.warning(t(($) => $.preferences.appearance_sync.recovered), {
      id: "settings-appearance-recovered",
    });
    acknowledgeRecoveryNotice();
  }, [acknowledgeRecoveryNotice, recoveryNoticePending, t]);

  const showAppearanceSaved = (receipt: AppearanceUndoReceipt | null) => {
    if (!receipt) return;
    toast.success(t(($) => $.auto_save.toast_saved), {
      id: "settings-appearance-save",
      action: {
        label: t(($) => $.preferences.appearance_sync.undo),
        onClick: () => {
          void undoAppearance(receipt).then((outcome) => {
            if (outcome === "expired") {
              toast.warning(
                t(($) => $.preferences.appearance_sync.undo_expired),
                { id: "settings-appearance-undo" },
              );
              return;
            }
            toast.success(
              t(($) => $.preferences.appearance_sync.undo_applied),
              { id: "settings-appearance-undo" },
            );
          });
        },
      },
    });
  };

  const handleCopyDiagnostics = async () => {
    const copied = await copyText(serializeAppearanceDiagnostics(diagnostics));
    toast[copied ? "success" : "error"](
      copied
        ? t(($) => $.preferences.appearance_sync.diagnostics_copied)
        : t(($) => $.preferences.appearance_sync.diagnostics_copy_failed),
      { id: "settings-appearance-diagnostics" },
    );
  };

  const themeIcons = { system: Monitor, light: Sun, dark: Moon } as const;
  const themeOptions = APPEARANCE_OPTIONS.map(({ value }) => ({
    value,
    label: t(($) => $.preferences.theme[value]),
    icon: themeIcons[value],
  }));
  const skinOptions = SKIN_OPTIONS.map(({ value }) => ({
    value,
    label: t(($) => $.preferences.skin[value].name),
    description: t(($) => $.preferences.skin[value].description),
  }));
  const syncMessage = getAppearanceSyncMessage(preferences);

  return (
    <>
      <RadioGroup
        aria-label={t(($) => $.preferences.skin.title)}
        value={skin}
        onValueChange={(value) => {
          if (!isSkinId(value)) return;
          showAppearanceSaved(selectSkin(value));
        }}
        className="grid gap-2 pe-chat-launcher @xl:grid-cols-3 @xl:pe-0"
      >
        {skinOptions.map((option) => {
          const selected = option.value === skin;
          return (
            <RadioGroupItem
              key={option.value}
              value={option.value}
              aria-label={`${option.label}. ${option.description}`}
              className={cn(
                "group cursor-pointer overflow-hidden rounded-lg border bg-surface text-left outline-none transition-colors",
                "hover:border-faint-foreground focus-visible:ring-3 focus-visible:ring-ring/40",
                selected
                  ? "border-primary ring-1 ring-primary"
                  : "border-surface-border",
              )}
            >
              <SemanticAppearanceFixture
                skin={option.value}
                mode={preferences.resolvedAppearance}
                compact
                className="border-b border-inherit"
                aria-hidden="true"
                labels={{
                  reviewReady: t(($) => $.preferences.appearance_fixture.review_ready),
                  updatedMomentsAgo: t(
                    ($) => $.preferences.appearance_fixture.updated_moments_ago,
                  ),
                  selectedTask: t(($) => $.preferences.appearance_fixture.selected_task),
                  assignee: t(($) => $.preferences.appearance_fixture.assignee),
                  done: t(($) => $.preferences.appearance_fixture.done),
                  watch: t(($) => $.preferences.appearance_fixture.watch),
                  remove: t(($) => $.preferences.appearance_fixture.remove),
                  summary: t(($) => $.preferences.appearance_fixture.summary),
                  linkedTask: t(($) => $.preferences.appearance_fixture.linked_task),
                  commandMenu: t(($) => $.preferences.appearance_fixture.command_menu),
                }}
              />
              <span className="flex min-h-16 items-start gap-2 px-3 py-2.5">
                <span className="min-w-0 flex-1">
                  <span className="block text-body font-semibold text-foreground">
                    {option.label}
                  </span>
                  <span className="mt-0.5 block text-caption leading-4 text-muted-foreground">
                    {option.description}
                  </span>
                </span>
                <Check
                  className={cn(
                    "mt-0.5 size-4 shrink-0 text-primary",
                    !selected && "invisible",
                  )}
                  aria-hidden="true"
                />
              </span>
            </RadioGroupItem>
          );
        })}
      </RadioGroup>

      <SettingsCard>
        <SettingsRow
          label={t(($) => $.preferences.theme.title)}
          description={t(($) => $.preferences.theme.hint)}
          size="none"
          className="sm:flex-col sm:items-stretch sm:gap-3 @xl:flex-row @xl:items-center @xl:gap-8"
        >
          <RadioGroup
            aria-label={t(($) => $.preferences.theme.title)}
            value={theme}
            onValueChange={(value) => {
              if (!isRequestedAppearance(value)) return;
              showAppearanceSaved(selectAppearance(value));
            }}
            className="grid grid-cols-3 gap-1 rounded-lg bg-secondary p-1"
          >
            {themeOptions.map((option) => {
              const Icon = option.icon;
              const selected = option.value === theme;
              return (
                <RadioGroupItem
                  key={option.value}
                  value={option.value}
                  aria-label={option.label}
                  className={cn(
                    "flex h-8 min-w-20 items-center justify-center gap-1.5 rounded-md px-2 text-caption font-medium outline-none transition-colors",
                    "focus-visible:ring-2 focus-visible:ring-ring",
                    selected
                      ? "bg-surface text-foreground shadow-[var(--surface-shadow)]"
                      : "text-muted-foreground hover:text-foreground",
                  )}
                >
                  <Icon className="size-3.5" aria-hidden="true" />
                  <span>{option.label}</span>
                </RadioGroupItem>
              );
            })}
          </RadioGroup>
        </SettingsRow>
      </SettingsCard>

      <div className="flex min-h-9 flex-wrap items-center justify-between gap-2">
        <div className="flex min-h-8 flex-wrap items-center gap-1">
          <span
            className="text-caption text-muted-foreground"
            role="status"
            aria-live="polite"
          >
            {t(($) => $.preferences.appearance_sync[syncMessage])}
          </span>
          {canRetry && (
            <Button
              variant="ghost"
              size="sm"
              className="h-8 px-2 text-foreground"
              onClick={retryAppearanceSync}
            >
              <RefreshCw className="size-3.5 text-warning" aria-hidden="true" />
              {t(($) => $.preferences.appearance_sync.retry)}
            </Button>
          )}
          {canCopyDiagnostics && (
            <Button
              variant="ghost"
              size="sm"
              className="h-8 px-2"
              onClick={() => void handleCopyDiagnostics()}
            >
              <Copy className="size-3.5" aria-hidden="true" />
              {t(($) => $.preferences.appearance_sync.copy_diagnostics)}
            </Button>
          )}
        </div>
        <Button
          variant="ghost"
          size="sm"
          className="h-8 px-2 text-muted-foreground"
          onClick={() => setResetDialogOpen(true)}
        >
          <RotateCcw className="size-3.5" aria-hidden="true" />
          {t(($) => $.preferences.appearance_sync.reset)}
        </Button>
      </div>

      <AlertDialog open={resetDialogOpen} onOpenChange={setResetDialogOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>
              {t(($) => $.preferences.appearance_sync.reset_title)}
            </AlertDialogTitle>
            <AlertDialogDescription>
              {t(($) => $.preferences.appearance_sync.reset_description)}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>
              {t(($) => $.preferences.appearance_sync.cancel)}
            </AlertDialogCancel>
            <AlertDialogAction
              onClick={() => {
                const receipt = resetAppearance();
                setResetDialogOpen(false);
                showAppearanceSaved(receipt);
              }}
            >
              {t(($) => $.preferences.appearance_sync.reset_confirm)}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  );
}

function LanguageRow() {
  const { t, i18n } = useT("settings");
  const localeAdapter = useLocaleAdapter();
  const user = useAuthStore((s) => s.user);
  const reloadTimerRef = useRef<number | null>(null);

  useEffect(() => () => {
    if (reloadTimerRef.current !== null) {
      window.clearTimeout(reloadTimerRef.current);
    }
  }, []);

  // i18next.language can be a region-tagged BCP-47 string (e.g. "en-US",
  // "zh-Hans-CN") returned by intl-localematcher. Normalize it before
  // comparing so the picker always exposes the actual selected option.
  const currentLocale: SupportedLocale = resolveSettingsLocale(i18n.language);

  const languageOptions: { value: SupportedLocale; label: string }[] = [
    { value: "en", label: t(($) => $.preferences.language.english) },
    { value: "zh-Hans", label: t(($) => $.preferences.language.chinese) },
    { value: "ko", label: t(($) => $.preferences.language.korean) },
    { value: "ja", label: t(($) => $.preferences.language.japanese) },
    { value: "fr", label: t(($) => $.preferences.language.french) },
  ];

  // Persist locally → sync to user.language → reload. Reload (vs in-place
  // changeLanguage) avoids hydration mismatch and is the i18next-recommended
  // pattern for App Router. The reload itself is the confirmation.
  //
  // If the cross-device sync (PATCH /api/me) fails, the local cookie is
  // already written so the new locale will take effect after reload — but
  // the user's other devices won't see the change. Surface that explicitly
  // via a toast and delay the reload long enough for the toast to be read,
  // otherwise the failure would be invisible.
  const handleLanguageChange = async (next: SupportedLocale) => {
    if (next === currentLocale) return;
    localeAdapter.persist(next);

    let syncFailed = false;
    if (user) {
      try {
        await api.updateMe({ language: next });
      } catch {
        syncFailed = true;
      }
    }

    if (syncFailed) {
      toast.warning(t(($) => $.preferences.language.sync_failed));
      // Give the toast 2.5s of visible time before navigating away.
      reloadTimerRef.current = window.setTimeout(() => {
        reloadTimerRef.current = null;
        window.location.reload();
      }, 2500);
      return;
    }
    window.location.reload();
  };

  return (
    <SettingsRow
      anchor="language"
      label={t(($) => $.preferences.language.title)}
      size="select"
    >
      <Select
        items={languageOptions}
        value={currentLocale}
        onValueChange={(next) => {
          if (next) void handleLanguageChange(next as SupportedLocale);
        }}
      >
        <SelectTrigger
          size="sm"
          className="w-full"
          aria-label={t(($) => $.preferences.language.title)}
        >
          <SelectValue>
            {languageOptions.find((option) => option.value === currentLocale)?.label}
          </SelectValue>
        </SelectTrigger>
        <SelectContent align="end">
          {languageOptions.map((option) => (
            <SelectItem key={option.value} value={option.value}>
              {option.label}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    </SettingsRow>
  );
}

function StickyCommentBarRow() {
  const { t } = useT("settings");
  const sticky = useCommentComposerStore((s) => s.sticky);
  const toggleSticky = useCommentComposerStore((s) => s.toggleSticky);

  return (
    <SettingsRow
      anchor="sticky-comment-bar"
      label={t(($) => $.preferences.sticky_comment_bar.title)}
    >
      <Switch
        checked={sticky}
        onCheckedChange={() => toggleSticky()}
        aria-label={t(($) => $.preferences.sticky_comment_bar.title)}
      />
    </SettingsRow>
  );
}

function RunningAgentReplyRow() {
  const { t } = useT("settings");
  const value = useCommentComposerStore((s) => s.runningAgentReply);
  const setValue = useCommentComposerStore((s) => s.setRunningAgentReply);
  const options: { value: RunningAgentReply; label: string }[] = [
    { value: "steer", label: t(($) => $.preferences.running_agent_reply.steer) },
    { value: "after_run", label: t(($) => $.preferences.running_agent_reply.after_run) },
  ];

  return (
    <SettingsRow
      anchor="running-agent-reply"
      label={t(($) => $.preferences.running_agent_reply.title)}
      description={t(($) => $.preferences.running_agent_reply.hint)}
      size="select"
    >
      <Select
        items={options}
        value={value}
        onValueChange={(next) => {
          if (next && next !== value) setValue(next as RunningAgentReply);
        }}
      >
        <SelectTrigger
          size="sm"
          className="w-full"
          aria-label={t(($) => $.preferences.running_agent_reply.title)}
        >
          <SelectValue>
            {options.find((option) => option.value === value)?.label}
          </SelectValue>
        </SelectTrigger>
        <SelectContent align="end">
          {options.map((option) => (
            <SelectItem key={option.value} value={option.value}>
              {option.label}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    </SettingsRow>
  );
}

/**
 * What a plain click on an issue card or row opens across the issue views.
 * The hint carries the one non-obvious part: Shift+Click opens the other one,
 * so both stay one click away whichever is chosen.
 */
function IssueOpeningRow() {
  const { t } = useT("settings");
  const value = useIssueOpeningStore((s) => s.openMode);
  const setValue = useIssueOpeningStore((s) => s.setOpenMode);
  const label = t(($) => $.issue.opening.click);
  return (
    <SettingsRow
      anchor="issue-opening"
      label={label}
      description={t(($) => $.issue.opening.hint)}
    >
      <div role="group" aria-label={label}>
        <SegmentedToggle<IssueOpenMode>
          value={value}
          onChange={setValue}
          buttonClassName="px-3 py-1 text-label"
          options={[
            ["page", t(($) => $.issue.opening.page)],
            ["peek", t(($) => $.issue.opening.peek)],
          ]}
        />
      </div>
    </SettingsRow>
  );
}

/**
 * When off, the FAB / overlay never mount and Chat is reachable only from its
 * dedicated tab. A persisted client setting, so it applies immediately.
 */
function FloatingChatRow() {
  const { t } = useT("settings");
  const enabled = useChatStore((s) => s.floatingChatEnabled);
  const setEnabled = useChatStore((s) => s.setFloatingChatEnabled);
  return (
    <SettingsRow
      anchor="chat"
      label={t(($) => $.chat.floating_label)}
      description={t(($) => $.chat.floating_hint)}
    >
      <Switch
        checked={enabled}
        onCheckedChange={(checked) => setEnabled(checked)}
        aria-label={t(($) => $.chat.floating_label)}
      />
    </SettingsRow>
  );
}

/**
 * Which fields each create-issue dialog keeps on its toolbar, as one field ×
 * dialog grid. A field toggled off stays reachable from the dialog's ⋯
 * overflow and re-surfaces while it holds a value, so hiding is never
 * destructive. Quick create supports fewer fields; those cells show a dash.
 */
function IssueFieldsMatrix() {
  const { t } = useT("settings");
  const quickFields = useIssueCreateSettingsStore((s) => s.quickCreateFields);
  const setQuickVisible = useIssueCreateSettingsStore(
    (s) => s.setQuickCreateFieldVisible,
  );
  const manualFields = useIssueCreateSettingsStore((s) => s.manualCreateFields);
  const setManualVisible = useIssueCreateSettingsStore(
    (s) => s.setManualCreateFieldVisible,
  );
  const quickLabel = t(($) => $.preferences.issue_fields.quick);
  const manualLabel = t(($) => $.preferences.issue_fields.manual);

  return (
    <SettingsCard>
      <table className="w-full text-body">
        <thead>
          <tr className="border-b border-surface-border bg-muted/20 text-caption font-medium text-muted-foreground">
            <th scope="col" className="px-4 py-2 text-left font-medium">
              {t(($) => $.preferences.issue_fields.field)}
            </th>
            <th scope="col" className="w-32 px-4 py-2 text-center font-medium">
              {quickLabel}
            </th>
            <th scope="col" className="w-32 px-4 py-2 text-center font-medium">
              {manualLabel}
            </th>
          </tr>
        </thead>
        <tbody className="divide-y divide-surface-border">
          {MANUAL_CREATE_FIELDS.map((field: ManualCreateField) => {
            const fieldLabel = t(($) => $.issue.fields[field]);
            const quickSupported = (QUICK_CREATE_FIELDS as readonly string[]).includes(field);
            return (
              <tr key={field}>
                <th scope="row" className="px-4 py-2.5 text-left font-normal">
                  {fieldLabel}
                </th>
                <td className="px-4 py-2.5 text-center">
                  {quickSupported ? (
                    <Checkbox
                      checked={quickFields.includes(field as QuickCreateField)}
                      onCheckedChange={(checked) =>
                        setQuickVisible(field as QuickCreateField, checked === true)
                      }
                      aria-label={`${fieldLabel} · ${quickLabel}`}
                    />
                  ) : (
                    <span
                      className="text-faint-foreground"
                      aria-label={t(($) => $.preferences.issue_fields.unsupported)}
                      title={t(($) => $.preferences.issue_fields.unsupported)}
                    >
                      —
                    </span>
                  )}
                </td>
                <td className="px-4 py-2.5 text-center">
                  <Checkbox
                    checked={manualFields.includes(field)}
                    onCheckedChange={(checked) =>
                      setManualVisible(field, checked === true)
                    }
                    aria-label={`${fieldLabel} · ${manualLabel}`}
                  />
                </td>
              </tr>
            );
          })}
        </tbody>
      </table>
    </SettingsCard>
  );
}

// Base UI rejects "" as an item value, so route the "no preference" state
// through this sentinel and translate at the wire boundary.
const BROWSER_TZ_VALUE = "__browser__";

/** "UTC+8", "UTC-3:30" — the offset right now, for scanning the list. */
function utcOffset(tz: string): string {
  try {
    const part = new Intl.DateTimeFormat("en-US", {
      timeZone: tz,
      timeZoneName: "shortOffset",
    })
      .formatToParts(new Date())
      .find((p) => p.type === "timeZoneName")?.value;
    return part ? part.replace(/^GMT/, "UTC").replace(/^UTC$/, "UTC+0") : "";
  } catch {
    return "";
  }
}

function TimezoneRow() {
  const { t } = useT("settings");
  const user = useAuthStore((s) => s.user);
  const setUser = useAuthStore((s) => s.setUser);
  const [open, setOpen] = useState(false);
  const stored = user?.timezone ?? null;
  const browser = browserTimezone();
  const value = stored ?? BROWSER_TZ_VALUE;

  // The full IANA list (~600 zones) so a user needing a non-curated zone is
  // not stuck with the common ones — which is why this is searchable.
  const options = useMemo(
    () =>
      timezoneOptions(stored ?? browser).map((tz) => ({
        tz,
        offset: utcOffset(tz),
      })),
    [stored, browser],
  );

  const handleChange = async (next: string) => {
    setOpen(false);
    if (next === value) return;
    const payload = next === BROWSER_TZ_VALUE ? "" : next;
    try {
      const updated = await api.updateMe({ timezone: payload });
      setUser(updated);
    } catch (err) {
      toast.error(
        err instanceof Error && err.message
          ? err.message
          : t(($) => $.preferences.timezone.sync_failed),
      );
    }
  };

  const current = stored ?? browser;
  const suffix = stored ? "" : t(($) => $.preferences.timezone.browser_suffix);

  return (
    <SettingsRow
      anchor="timezone"
      label={t(($) => $.preferences.timezone.title)}
      description={t(($) => $.preferences.timezone.hint)}
      size="select-wide"
    >
      <Popover open={open} onOpenChange={setOpen}>
        <PopoverTrigger
          render={
            <button
              type="button"
              aria-label={t(($) => $.preferences.timezone.title)}
              className="flex h-7 w-full items-center gap-1.5 rounded-md border border-input bg-transparent pl-2.5 pr-2 text-left text-body outline-none transition-colors focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50 dark:bg-input/30 dark:hover:bg-input/50"
            />
          }
        >
          <span className="min-w-0 flex-1 truncate font-mono text-caption">
            {current}
            {suffix}
          </span>
          <span className="shrink-0 text-caption text-muted-foreground">
            {utcOffset(current)}
          </span>
          <ChevronsUpDown aria-hidden="true" className="size-4 shrink-0 text-muted-foreground" />
        </PopoverTrigger>
        <PopoverContent align="end" className="w-80 p-0">
          {/* Plain substring matching: fuzzy ranking puts "Antarctica/Vostok"
              next to "Asia/Tokyo" for "tok", which reads as a wrong result. */}
          <Command
            filter={(itemValue, search) =>
              itemValue.toLocaleLowerCase().includes(search.trim().toLocaleLowerCase()) ? 1 : 0
            }
          >
            <CommandInput placeholder={t(($) => $.preferences.timezone.search)} />
            <CommandList>
              <CommandEmpty>{t(($) => $.preferences.timezone.empty)}</CommandEmpty>
              <CommandItem
                value={`${browser} ${t(($) => $.preferences.timezone.browser_suffix)}`}
                data-checked={value === BROWSER_TZ_VALUE}
                onSelect={() => void handleChange(BROWSER_TZ_VALUE)}
              >
                <span className="min-w-0 flex-1 truncate font-mono text-caption">
                  {browser}
                  {t(($) => $.preferences.timezone.browser_suffix)}
                </span>
                <span className="text-caption text-muted-foreground">{utcOffset(browser)}</span>
              </CommandItem>
              {options.map(({ tz, offset }) => (
                <CommandItem
                  key={tz}
                  value={`${tz} ${offset}`}
                  data-checked={value === tz}
                  onSelect={() => void handleChange(tz)}
                >
                  <span className="min-w-0 flex-1 truncate font-mono text-caption">{tz}</span>
                  <span className="text-caption text-muted-foreground">{offset}</span>
                </CommandItem>
              ))}
            </CommandList>
          </Command>
        </PopoverContent>
      </Popover>
    </SettingsRow>
  );
}
