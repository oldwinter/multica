"use client";

import { useEffect, useMemo, useRef, useState, type ReactNode } from "react";
import { useQuery } from "@tanstack/react-query";
import { toast } from "sonner";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@multica/ui/components/ui/dialog";
import { Button } from "@multica/ui/components/ui/button";
import { Textarea } from "@multica/ui/components/ui/textarea";
import { Spinner } from "@multica/ui/components/ui/spinner";
import type { IssueAssigneeType, IssueStatus, UpdateIssueRequest } from "@multica/core/types";
import { useUpdateIssue, useBatchUpdateIssues } from "@multica/core/issues/mutations";
import { errorCode } from "@multica/core/api";
import { useActorName } from "@multica/core/workspace/hooks";
import { useWorkspaceId } from "@multica/core/hooks";
import { agentListOptions, squadListOptions } from "@multica/core/workspace/queries";
import { runtimeListOptions, readRuntimeCliVersion, handoffSupported } from "@multica/core/runtimes";
import { useShortcut, shortcutMatchesEvent, isPlainShortcut } from "@multica/core/shortcuts";
import { isImeComposing } from "@multica/core/utils";
import {
  usePreviewTwinBriefing,
  type TwinBindingState,
  type TwinBriefingPreview,
} from "@multica/core/twins";
import { Eye, Power, Sparkles } from "lucide-react";
import { ShortcutKeycaps } from "../common/shortcut-keycaps";
import { useStatusLabel } from "../issues/utils/status-label";
import { useT } from "../i18n";

const MAX_HANDOFF_NOTE = 2000;

// i18next inlines {{name}} / {{status}} into the sentence, but their position
// varies by language ("{{name}} 会…" vs "Once assigned, {{name}} will…" vs
// "{{name}}'s leader…"). Fence each one with a sentinel so we can bold just
// those spans at render time without splitting copy into per-language
// prefix/suffix keys. Bolding is also what marks a custom status name as a
// status rather than an ordinary word ("Move this issue to Later.").
const FENCE = "\u0000";

const fenced = (value: string) => `${FENCE}${value}${FENCE}`;

function boldFenced(text: string): ReactNode {
  const parts = text.split(FENCE);
  // Every fenced span contributes one odd-indexed part; an unfenced string is
  // a single part and renders as-is.
  if (parts.length < 3) return text;
  return (
    <>
      {parts.map((part, i) =>
        i % 2 === 1 ? (
          <span key={i} className="font-semibold text-foreground">
            {part}
          </span>
        ) : (
          part
        ),
      )}
    </>
  );
}

interface RunConfirmData {
  issueIds?: string[];
  // The two issue writes that hand work to an agent, and the only two that
  // confirm. `assign` gives the issue an agent/squad owner; `promote` moves an
  // already-owned issue out of the backlog category, which starts the run on
  // its own (RunSourceStatus). Batch status changes still apply directly
  // (MUL-4155) — `promote` is the single-issue picker path only (MUL-6463).
  mode?: "assign" | "promote";
  /** promote only: the status KEY the issue is moving to. */
  status?: IssueStatus;
  assigneeType?: IssueAssigneeType;
  assigneeId?: string;
  assigneeName?: string;
  issueRevision?: number;
  request?: string;
  projectId?: string;
}

/**
 * Handoff confirmation for the issue writes that start agent runs.
 *
 * The rule is "dialog = you are handing this to an agent", NOT "you are
 * confirming N runs" (MUL-5010). The note and actions remain usable on the
 * first frame. A single eligible run compiles its Twin briefing alongside the
 * form. The run write waits for that exact one-off snapshot so it cannot queue
 * a different signed version than the one the user reviewed.
 *
 * Completion is silent: the assignee/status change and any run it starts
 * surface through the issue's normal updates, so the confirm adds no result
 * toast. Whether a run starts stays the server's existing decision at write
 * time. Dismissing the dialog (X / Esc / click-outside) cancels without any
 * write. Shared by single assign (1 id), batch assign (N ids), and the
 * single-issue promotion out of backlog.
 */
export function RunConfirmModal({
  onClose,
  data,
}: {
  onClose: () => void;
  data: Record<string, unknown> | null;
}) {
  const { t } = useT("modals");
  const { t: tIssues } = useT("issues");
  const { getActorName } = useActorName();
  const sendShortcut = useShortcut("send");
  const d = (data ?? {}) as RunConfirmData;
  const issueIds = d.issueIds ?? [];

  const [note, setNote] = useState("");
  // Which footer action is in flight, so only the clicked button shows a
  // spinner (the request runs an agent on the server for note assigns, so it is
  // not instant — the disabled-only state read as frozen).
  const [pendingAction, setPendingAction] = useState<"go" | "suppress" | null>(null);
  const [twinUseState, setTwinUseState] = useState<TwinBindingState>("off");
  const [previewRevision, setPreviewRevision] = useState(0);
  const [previewSnapshot, setPreviewSnapshot] = useState<{
    readonly requestedState: TwinBindingState;
    readonly data: TwinBriefingPreview;
  } | null>(null);
  const [previewPending, setPreviewPending] = useState(false);
  const [previewError, setPreviewError] = useState(false);
  const previewSequence = useRef(0);
  const previewIntent = useRef("");
  const previewedVersionId = useRef<string | null>(null);
  const submitting = pendingAction !== null;

  const updateIssue = useUpdateIssue();
  const batchUpdate = useBatchUpdateIssues();

  // Handoff-support verdict, resolved entirely from warm client caches
  // (useWorkspacePresencePrefetch keeps agents / squads / runtimes hot), so the
  // note box settles on the first frame with no round-trip — the same shape as
  // the quick-create version gate. An agent assignee targets its own runtime; a
  // squad targets its leader's, which the squad list gives us directly, so both
  // are knowable locally. `null` means "cannot tell" (assignee not in cache
  // yet, or no runtime bound) and leaves the box enabled: the note is a soft
  // gate, and a spurious warning is worse than a note an old daemon drops.
  const wsId = useWorkspaceId();
  // Built-ins resolve through i18n, custom statuses through the catalog, so the
  // promotion headline reads the same way the picker the user just used does.
  const statusLabel = useStatusLabel(wsId);
  const { data: agents = [] } = useQuery({ ...agentListOptions(wsId), enabled: !!wsId });
  const { data: runtimes = [] } = useQuery({ ...runtimeListOptions(wsId), enabled: !!wsId });
  const { data: squads = [] } = useQuery({ ...squadListOptions(wsId), enabled: !!wsId });
  const targetAgentId = useMemo(() => {
    if (d.assigneeType === "agent") return d.assigneeId;
    if (d.assigneeType === "squad") {
      return squads.find((s) => s.id === d.assigneeId)?.leader_id;
    }
    return undefined;
  }, [d.assigneeId, d.assigneeType, squads]);
  const localHandoff = useMemo<boolean | null>(() => {
    if (!targetAgentId) return null;
    const agent = agents.find((a) => a.id === targetAgentId);
    if (!agent?.runtime_id) return null;
    const runtime = runtimes.find((r) => r.id === agent.runtime_id);
    if (!runtime) return null;
    return handoffSupported(readRuntimeCliVersion(runtime.metadata));
  }, [targetAgentId, agents, runtimes]);

  const twinPreview = usePreviewTwinBriefing();
  const canPreviewTwin = issueIds.length === 1 && !!targetAgentId && !!d.request;
  const previewIntentKey = [targetAgentId, d.projectId, issueIds[0], d.request].join("\u0000");
  useEffect(() => {
    if (!canPreviewTwin || !targetAgentId || !d.request) return;
    if (previewIntent.current !== previewIntentKey) {
      previewIntent.current = previewIntentKey;
      previewedVersionId.current = null;
    }
    const sequence = ++previewSequence.current;
    const requestedState = twinUseState;
    const pinnedVersionId = requestedState === "off" ? null : previewedVersionId.current;
    setPreviewPending(true);
    setPreviewError(false);
    setPreviewSnapshot(null);
    void twinPreview.mutateAsync({
      agentId: targetAgentId,
      ...(d.projectId ? { projectId: d.projectId } : {}),
      issueId: issueIds[0],
      request: d.request,
      oneOffState: requestedState,
      ...(pinnedVersionId ? { twinVersionId: pinnedVersionId } : {}),
    }).then((result) => {
      if (previewSequence.current !== sequence) return;
      if (result.twinVersion) previewedVersionId.current = result.twinVersion.id;
      else if (requestedState !== "off") previewedVersionId.current = null;
      setPreviewSnapshot({ requestedState, data: result });
      setPreviewPending(false);
    }).catch(() => {
      if (previewSequence.current !== sequence) return;
      setPreviewError(true);
      setPreviewPending(false);
    });
    return () => {
      if (previewSequence.current === sequence) previewSequence.current += 1;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [canPreviewTwin, previewIntentKey, previewRevision, twinUseState]);

  const activePreview = previewSnapshot?.requestedState === twinUseState
    ? previewSnapshot.data
    : null;
  const previewNeedsVersion = twinUseState === "preview" || twinUseState === "enabled";
  const previewReady = !canPreviewTwin || (
    !previewPending &&
    !previewError &&
    activePreview !== null &&
    (!previewNeedsVersion || activePreview.twinVersion !== null)
  );

  // Soft gate: an old runtime can't render the note. Disable the box but let
  // the assignment proceed (MUL-3375 §6.3).
  const noteDisabled = localHandoff === false;

  // A promotion carries the status and nothing else: the owner is already on
  // the issue, and re-sending the same assignee would turn a status write into
  // an assignee write on the server's side of the trigger predicate.
  const isPromote = d.mode === "promote" && !!d.status;

  const applyTo = (extra: Partial<UpdateIssueRequest>) => {
    const base: UpdateIssueRequest = isPromote
      ? { status: d.status }
      : {
          assignee_type: d.assigneeType ?? null,
          assignee_id: d.assigneeId ?? null,
        };
    const twinVersionId = activePreview?.twinVersion?.id;
    const twinUse =
      issueIds.length === 1 &&
      activePreview &&
      (twinUseState !== "enabled" || twinVersionId)
        ? {
            state: twinUseState,
            ...((twinUseState === "enabled" || twinUseState === "preview") && twinVersionId
              ? { twin_version_id: twinVersionId }
              : {}),
          }
        : undefined;
    return { ...base, ...extra, ...(twinUse ? { twin_use: twinUse } : {}) };
  };

  // The copy names whoever the issue is handed to; for a squad that is the
  // squad itself, since its leader deciding who works is an internal detail.
  const assigneeName =
    d.assigneeName ??
    getActorName(d.assigneeType === "squad" ? "squad" : "agent", d.assigneeId ?? "");

  const submit = async (suppressRun: boolean) => {
    if (issueIds.length === 0 || submitting || !previewReady) return;
    setPendingAction(suppressRun ? "suppress" : "go");
    const payload = applyTo({
      ...(suppressRun ? { suppress_run: true } : {}),
      ...(!suppressRun && !noteDisabled && note.trim() ? { handoff_note: note.trim() } : {}),
    });
    if (suppressRun) delete payload.twin_use;
    try {
      // Completion is silent, exactly as before: the assignee and any run show
      // up through the issue's normal assignee / run-status updates, so there is
      // no result toast to add here. Whether a run started is the server's
      // existing decision at write time, not something this dialog reports.
      if (issueIds.length === 1) {
        await updateIssue.mutateAsync({
          id: issueIds[0]!,
          ...payload,
        });
      } else {
        await batchUpdate.mutateAsync({ ids: issueIds, updates: payload });
      }
      onClose();
    } catch (err) {
      toast.error(
        errorCode(err) === "revision_conflict"
          ? tIssues(($) => $.revision.conflict)
          : err instanceof Error && err.message
            ? err.message
            : t(($) => $.run_confirm.toast_failed),
      );
      setPendingAction(null);
    }
  };

  /**
   * The configured `send` chord confirms the assignment, the same chord that
   * creates from the issue composer (MUL-5694).
   *
   * Bound on the dialog, not on the note box, because the chord means "run the
   * primary action" no matter which control has focus — and the note box is
   * not always where focus is. An old runtime disables it, which hands initial
   * focus to the footer instead, and that is precisely where the keycap on the
   * confirm button would otherwise be advertising a dead key.
   */
  const onDialogKeyDown = (e: React.KeyboardEvent) => {
    // A held chord submits once, and the Enter that commits an IME
    // composition is the user picking a candidate, never a confirmation.
    if (e.defaultPrevented || e.repeat || isImeComposing(e)) return;
    if (!shortcutMatchesEvent(sendShortcut, e.nativeEvent)) return;
    // Only a BARE Enter activates a focused button (Chromium fires no click
    // for ⌘/Ctrl+Enter), so a `send` remapped to plain Enter is the one case
    // where confirming here too would double-write — and on "Don't start yet"
    // the two writes would disagree about suppress_run. Every chord form
    // reaches the footer as a dead key without us, so it must not be skipped.
    const activatesFocusedButton =
      isPlainShortcut(sendShortcut, "Enter") &&
      e.target instanceof HTMLElement &&
      e.target.closest("button") !== null;
    if (activatesFocusedButton) return;
    e.preventDefault();
    void submit(false);
  };

  // States the action, not a prediction: the write is certain, the run is
  // conditional, so the copy names no run count. The promotion names the
  // status it is moving to by its workspace label — a custom status is only
  // recognisable by the name its admin gave it.
  const headline: ReactNode = boldFenced(
    isPromote
      ? t(($) => $.run_confirm.promote_single, {
          name: fenced(assigneeName),
          status: fenced(statusLabel(d.status ?? "")),
        })
      : issueIds.length > 1
        ? t(($) => $.run_confirm.assign_batch, {
            name: fenced(assigneeName),
            count: issueIds.length,
          })
        : t(($) => $.run_confirm.assign_single, { name: fenced(assigneeName) }),
  );

  return (
    <Dialog open onOpenChange={(v) => { if (!v && !submitting) onClose(); }}>
      <DialogContent onKeyDown={onDialogKeyDown}>
        <DialogHeader>
          <DialogTitle>
            {isPromote
              ? t(($) => $.run_confirm.title_promote)
              : t(($) => $.run_confirm.title_assign)}
          </DialogTitle>
          <DialogDescription>{headline}</DialogDescription>
        </DialogHeader>

        {/* The note remains editable while the independent Twin preview settles. */}
        <div className="grid gap-1.5">
          <label className="text-body font-medium" htmlFor="handoff-note">
            {t(($) => $.run_confirm.note_label)}
          </label>
          <Textarea
            id="handoff-note"
            value={note}
            maxLength={MAX_HANDOFF_NOTE}
            disabled={submitting || noteDisabled}
            placeholder={t(($) => $.run_confirm.note_placeholder)}
            onChange={(e) => setNote(e.target.value)}
            rows={3}
          />
          {noteDisabled ? (
            <p className="text-caption text-muted-foreground">{t(($) => $.run_confirm.note_unsupported)}</p>
          ) : null}
        </div>

        {canPreviewTwin ? (
          <section className="grid gap-2 border-y py-3" aria-labelledby="run-twin-use-title">
            <div className="flex min-w-0 items-start justify-between gap-3">
              <div className="min-w-0">
                <h3 id="run-twin-use-title" className="text-body font-medium">
                  {t(($) => $.run_confirm.twin_title)}
                </h3>
                <p className="text-caption text-muted-foreground">
                  {t(($) => $.run_confirm.twin_description)}
                </p>
              </div>
              {activePreview ? (
                <span className="shrink-0 text-caption tabular-nums text-muted-foreground">
                  {t(($) => $.run_confirm.twin_budget, {
                    bytes: activePreview.byteCount,
                    tokens: activePreview.tokenCount,
                  })}
                </span>
              ) : null}
            </div>

            <div className="grid grid-cols-3 gap-1" role="group" aria-label={t(($) => $.run_confirm.twin_title)}>
              {([
                ["off", Power, t(($) => $.run_confirm.twin_off)],
                ["preview", Eye, t(($) => $.run_confirm.twin_preview)],
                ["enabled", Sparkles, t(($) => $.run_confirm.twin_enabled)],
              ] as const).map(([state, Icon, label]) => {
                const noVersion = state === twinUseState && state !== "off" && activePreview?.twinVersion === null;
                return (
                  <Button
                    key={state}
                    type="button"
                    size="sm"
                    variant={twinUseState === state ? "secondary" : "ghost"}
                    aria-pressed={twinUseState === state}
                    disabled={submitting}
                    title={noVersion ? t(($) => $.run_confirm.twin_no_version) : label}
                    onClick={() => {
                      setTwinUseState(state);
                      setPreviewRevision((revision) => revision + 1);
                    }}
                  >
                    <Icon className="size-4" />
                    {label}
                  </Button>
                );
              })}
            </div>

            {previewPending || (!activePreview && !previewError) ? (
              <div className="flex items-center gap-2 text-caption text-muted-foreground" role="status">
                <Spinner className="size-4" />
                {t(($) => $.run_confirm.twin_loading)}
              </div>
            ) : previewError ? (
              <p className="text-caption text-destructive" role="alert">
                {t(($) => $.run_confirm.twin_error)}
              </p>
            ) : activePreview ? (
              <>
                <p className="break-words text-caption text-muted-foreground">
                  {t(($) => $.run_confirm.twin_effective, {
                    state: activePreview.policy.state,
                    version: activePreview.twinVersion?.versionNumber ?? "-",
                  })}{" "}
                  {activePreview.policy.reason}
                </p>
                {activePreview.twinVersion ? (
                  <dl className="grid min-w-0 gap-2 text-caption sm:grid-cols-2">
                    <div className="min-w-0">
                      <dt className="text-muted-foreground">{t(($) => $.run_confirm.twin_version_id)}</dt>
                      <dd className="break-all font-mono text-foreground">{activePreview.twinVersion.id}</dd>
                    </div>
                    <div className="min-w-0">
                      <dt className="text-muted-foreground">{t(($) => $.run_confirm.twin_version_digest)}</dt>
                      <dd className="break-all font-mono text-foreground">{activePreview.twinVersion.contentDigest}</dd>
                    </div>
                  </dl>
                ) : null}
                {activePreview.briefing ? (
                  <pre className="max-h-32 overflow-auto whitespace-pre-wrap break-words border-l-2 pl-3 text-caption text-foreground">
                    {activePreview.briefing}
                  </pre>
                ) : null}
              </>
            ) : null}
          </section>
        ) : null}

        {/* A single run can only queue the exact snapshot shown above. */}
        <DialogFooter>
          <Button type="button" variant="outline" disabled={submitting || !previewReady} onClick={() => submit(true)}>
            {pendingAction === "suppress" ? <Spinner className="size-4" /> : t(($) => $.run_confirm.dont_start)}
          </Button>
          <Button type="button" disabled={submitting || !previewReady} onClick={() => submit(false)}>
            {pendingAction === "go" ? (
              <Spinner className="size-4" />
            ) : (
              <>
                {isPromote
                  ? t(($) => $.run_confirm.confirm_promote)
                  : t(($) => $.run_confirm.confirm_assign)}
                {/* Decorative: the accessible name stays the button's own copy,
                    not "Confirm assignment Command Enter". Absent when `send`
                    is unbound. */}
                {sendShortcut ? (
                  <ShortcutKeycaps
                    shortcut={sendShortcut}
                    decorative
                    className="ml-1"
                    keyClassName="border-background/30 bg-background/15 text-primary-foreground shadow-none"
                  />
                ) : null}
              </>
            )}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
