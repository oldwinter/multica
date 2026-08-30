"use client";

import { useEffect, useMemo, useState } from "react";
import {
  Check,
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
import {
  SKIN_IDS,
  serializeAppearanceDiagnostics,
  type AppearanceUndoReceipt,
  type SkinId,
} from "@multica/core/appearance";
import { cn } from "@multica/ui/lib/utils";
import { copyText } from "@multica/ui/lib/clipboard";
import {
  DEFAULT_LOCALE,
  SUPPORTED_LOCALES,
  type SupportedLocale,
} from "@multica/core/i18n";
import { useLocaleAdapter } from "@multica/core/i18n/react";
import { useAuthStore } from "@multica/core/auth";
import { useCommentComposerStore } from "@multica/core/issues/stores";
import { api } from "@multica/core/api";
import { browserTimezone, timezoneOptions } from "../../common/timezone-select";
import { useT } from "../../i18n";
import { useAppearancePreferences } from "../../appearance";
import {
  SettingsCard,
  SettingsRow,
  SettingsSection,
  SettingsTab,
} from "./settings-layout";

export function PreferencesTab() {
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
  const { t, i18n } = useT("settings");
  const localeAdapter = useLocaleAdapter();
  const user = useAuthStore((s) => s.user);

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

  // i18next.language can be a region-tagged BCP-47 string (e.g. "en-US",
  // "zh-Hans-CN") returned by intl-localematcher. Normalize to a supported
  // locale before comparing — otherwise the radio shows neither option active.
  const currentLocale: SupportedLocale = SUPPORTED_LOCALES.includes(
    i18n.language as SupportedLocale,
  )
    ? (i18n.language as SupportedLocale)
    : DEFAULT_LOCALE;

  const themeOptions = [
    {
      value: "system" as const,
      label: t(($) => $.preferences.theme.system),
      icon: Monitor,
    },
    {
      value: "light" as const,
      label: t(($) => $.preferences.theme.light),
      icon: Sun,
    },
    {
      value: "dark" as const,
      label: t(($) => $.preferences.theme.dark),
      icon: Moon,
    },
  ];

  const skinOptions: Array<{ value: SkinId; label: string; description: string }> =
    SKIN_IDS.map((value) => ({
      value,
      label: t(($) => $.preferences.skin[value].name),
      description: t(($) => $.preferences.skin[value].description),
    }));

  const languageOptions: { value: SupportedLocale; label: string }[] = [
    { value: "en", label: t(($) => $.preferences.language.english) },
    { value: "zh-Hans", label: t(($) => $.preferences.language.chinese) },
    { value: "ko", label: t(($) => $.preferences.language.korean) },
    { value: "ja", label: t(($) => $.preferences.language.japanese) },
  ];

  // Persist locally → sync to user.language → reload. Reload (vs in-place
  // changeLanguage) avoids hydration mismatch and is the i18next-recommended
  // pattern for App Router.
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
      setTimeout(() => window.location.reload(), 2500);
      return;
    }
    toast.success(t(($) => $.auto_save.toast_saved), {
      id: "settings-auto-save",
    });
    // Keep the confirmation visible before the locale reload replaces the UI.
    setTimeout(() => window.location.reload(), 900);
  };

  return (
    <SettingsTab title={t(($) => $.page.tabs.preferences)}>
      <SettingsSection
        title={t(($) => $.preferences.appearance_title)}
        description={t(($) => $.preferences.appearance_hint)}
        className="@container"
      >
        <RadioGroup
          aria-label={t(($) => $.preferences.skin.title)}
          value={skin}
          onValueChange={(value) => {
            showAppearanceSaved(selectSkin(value as SkinId));
          }}
          className="grid gap-2 @xl:grid-cols-3"
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
                showAppearanceSaved(
                  selectAppearance(value as "system" | "light" | "dark"),
                );
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
              {preferences.source === "default" &&
                t(($) => $.preferences.appearance_sync.default)}
              {preferences.source !== "default" &&
                preferences.syncState.status === "local-only" &&
                t(($) => $.preferences.appearance_sync.local_only)}
              {preferences.syncState.status === "synced" &&
                t(($) => $.preferences.appearance_sync.synced)}
              {preferences.syncState.status === "pending" &&
                t(($) => $.preferences.appearance_sync.pending)}
              {preferences.syncState.status === "failed" &&
                t(($) => $.preferences.appearance_sync.failed)}
            </span>
            {canRetry && (
              <Button
                variant="ghost"
                size="sm"
                className="h-8 px-2 text-warning"
                onClick={retryAppearanceSync}
              >
                <RefreshCw className="size-3.5" aria-hidden="true" />
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
      </SettingsSection>

      <SettingsSection title={t(($) => $.preferences.general_title)}>
        <SettingsCard>
          <SettingsRow
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

          <TimezoneRow />

          <StickyCommentBarRow />
        </SettingsCard>
      </SettingsSection>
    </SettingsTab>
  );
}

function StickyCommentBarRow() {
  const { t } = useT("settings");
  const sticky = useCommentComposerStore((s) => s.sticky);
  const toggleSticky = useCommentComposerStore((s) => s.toggleSticky);

  return (
    <SettingsRow
      label={t(($) => $.preferences.sticky_comment_bar.title)}
      description={t(($) => $.preferences.sticky_comment_bar.hint)}
    >
      <Switch
        checked={sticky}
        onCheckedChange={() => {
          toggleSticky();
          toast.success(t(($) => $.auto_save.toast_saved), {
            id: "settings-auto-save",
          });
        }}
        aria-label={t(($) => $.preferences.sticky_comment_bar.title)}
      />
    </SettingsRow>
  );
}

// Base UI rejects "" as a SelectItem value, so route the "no preference"
// state through this sentinel and translate at the wire boundary.
const BROWSER_TZ_VALUE = "__browser__";

function TimezoneRow() {
  const { t } = useT("settings");
  const user = useAuthStore((s) => s.user);
  const setUser = useAuthStore((s) => s.setUser);
  const stored = user?.timezone ?? null;
  const browser = browserTimezone();
  const value = stored ?? BROWSER_TZ_VALUE;

  // Full IANA list (from timezoneOptions in common/timezone-select) so a
  // user needing a non-curated zone isn't stuck with ~18 common ones.
  // Memoized — timezoneOptions enumerates ~600 IANA zones per call.
  const options = useMemo(
    () => timezoneOptions(stored ?? browser),
    [stored, browser],
  );

  const handleChange = async (next: string) => {
    if (next === value) return;
    const payload = next === BROWSER_TZ_VALUE ? "" : next;
    try {
      const updated = await api.updateMe({ timezone: payload });
      setUser(updated);
      toast.success(t(($) => $.auto_save.toast_saved), {
        id: "settings-auto-save",
      });
    } catch (err) {
      toast.error(
        err instanceof Error && err.message
          ? err.message
          : t(($) => $.preferences.timezone.sync_failed),
      );
    }
  };

  const formatTZLabel = (tz: string) => {
    if (tz === BROWSER_TZ_VALUE) {
      return `${browser}${t(($) => $.preferences.timezone.browser_suffix)}`;
    }
    return tz;
  };

  return (
    <SettingsRow
      label={t(($) => $.preferences.timezone.title)}
      description={t(($) => $.preferences.timezone.hint)}
      size="select-wide"
    >
      <Select
        items={[
          { value: BROWSER_TZ_VALUE, label: formatTZLabel(BROWSER_TZ_VALUE) },
          ...options.map((timezone) => ({
            value: timezone,
            label: formatTZLabel(timezone),
          })),
        ]}
        value={value}
        onValueChange={(next) => {
          if (next) void handleChange(next);
        }}
      >
        <SelectTrigger
          size="sm"
          className="w-full font-mono text-caption"
          aria-label={t(($) => $.preferences.timezone.title)}
        >
          <SelectValue>{formatTZLabel(value)}</SelectValue>
        </SelectTrigger>
        <SelectContent align="end" className="max-h-72">
          <SelectItem value={BROWSER_TZ_VALUE} className="font-mono text-caption">
            {formatTZLabel(BROWSER_TZ_VALUE)}
          </SelectItem>
          {options.map((timezone) => (
            <SelectItem key={timezone} value={timezone} className="font-mono text-caption">
              {formatTZLabel(timezone)}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    </SettingsRow>
  );
}
