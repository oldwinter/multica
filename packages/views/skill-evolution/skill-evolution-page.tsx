"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import {
  skillEvolutionOverviewOptions,
  skillEvolutionProposalOptions,
  useConfigureSkillEvolution,
  useForkSkillForEvolution,
  usePauseSkillEvolution,
  usePublishSkillEvolutionProposal,
  useRejectSkillEvolutionProposal,
  useRequestSkillEvolutionProposal,
  useRollbackSkillEvolutionRelease,
  type ConfigureSkillEvolutionInput,
  type SkillEvolutionOverview,
} from "@multica/core/skill-evolution";
import { ApiError } from "@multica/core/api";
import { paths } from "@multica/core/paths";
import {
  AlertTriangle,
  GitFork,
  Loader2,
  Lock,
  RefreshCw,
  ShieldAlert,
  Sparkles,
} from "lucide-react";
import { toast } from "sonner";
import { Alert, AlertDescription, AlertTitle } from "@multica/ui/components/ui/alert";
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
import { Button } from "@multica/ui/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@multica/ui/components/ui/dialog";
import { Input } from "@multica/ui/components/ui/input";
import { Label } from "@multica/ui/components/ui/label";
import { Skeleton } from "@multica/ui/components/ui/skeleton";
import { Textarea } from "@multica/ui/components/ui/textarea";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@multica/ui/components/ui/tooltip";
import { cn } from "@multica/ui/lib/utils";
import { BreadcrumbHeader } from "../layout/breadcrumb-header";
import { useT } from "../i18n";
import { useNavigation } from "../navigation";
import { LoopConfiguration } from "./loop-configuration";
import { ProposalReview } from "./proposal-review";
import { ReleaseHistory } from "./release-history";
import {
  isProposalPending,
  normalizeLoopMode,
  proposalStatusTone,
} from "./status";
import { EvolutionStatusBadge } from "./status-badge";

type Proposal = SkillEvolutionOverview["proposals"][number];
type Release = SkillEvolutionOverview["releases"][number];
type QueuedRoom = {
  roomId: string;
  queuedAt: number;
  previousProposalIds: string[];
};

const ROOM_PROPOSAL_POLL_MS = 3000;
const ROOM_PROPOSAL_POLL_WINDOW_MS = 120000;
const EMPTY_PROPOSALS: Proposal[] = [];

function createIntentKey(action: string, identity: string): string {
  const nonce = globalThis.crypto?.randomUUID?.() ?? `${Date.now()}-${Math.random().toString(36).slice(2)}`;
  return `skill-evolution-${action}-${identity}-${nonce}`;
}

function formatDate(value: string | null | undefined, locale: string, fallback: string) {
  if (!value) return fallback;
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return fallback;
  return new Intl.DateTimeFormat(locale, {
    dateStyle: "medium",
    timeStyle: "short",
  }).format(date);
}

function shortHash(value: string | null | undefined, fallback: string): string {
  return value ? value.slice(0, 14) : fallback;
}

function MutationFailure({ fallback, error }: { fallback: string; error: Error | null }) {
  if (!error) return null;
  return (
    <p role="alert" className="text-caption text-destructive">
      {error.message || fallback}
    </p>
  );
}

function LoadingPage() {
  return (
    <div className="flex min-h-0 flex-1 flex-col">
      <div className="flex h-12 shrink-0 items-center gap-2 border-b px-4">
        <Skeleton className="h-4 w-20" />
        <Skeleton className="h-4 w-32" />
      </div>
      <div className="border-b px-4 py-4 sm:px-6">
        <Skeleton className="h-5 w-56" />
        <div className="mt-3 flex gap-3">
          <Skeleton className="h-8 w-36" />
          <Skeleton className="h-8 w-36" />
          <Skeleton className="h-8 w-28" />
        </div>
      </div>
      <div className="grid flex-1 gap-px bg-border xl:grid-cols-[20rem_minmax(0,1fr)_20rem]">
        <Skeleton className="h-full min-h-72 rounded-none bg-background" />
        <Skeleton className="h-full min-h-96 rounded-none bg-background" />
        <Skeleton className="h-full min-h-72 rounded-none bg-background" />
      </div>
    </div>
  );
}

function StatePage({
  forbidden,
  error,
  onRetry,
}: {
  forbidden: boolean;
  error: Error | null;
  onRetry: () => void;
}) {
  const { t } = useT("skill-evolution");
  return (
    <div className="flex min-h-0 flex-1 items-center justify-center px-4 py-10 text-center">
      <div className="flex max-w-md flex-col items-center gap-2">
        {forbidden ? (
          <ShieldAlert className="size-8 text-warning" aria-hidden="true" />
        ) : (
          <AlertTriangle className="size-8 text-destructive" aria-hidden="true" />
        )}
        <h1 className="text-title-sm font-medium">
          {forbidden ? t(($) => $.page.forbidden_title) : t(($) => $.page.error_title)}
        </h1>
        <p className="text-body text-muted-foreground">
          {forbidden
            ? t(($) => $.page.forbidden_description)
            : error?.message || t(($) => $.page.error_description)}
        </p>
        {!forbidden ? (
          <Button type="button" variant="outline" size="sm" className="mt-2" onClick={onRetry}>
            {t(($) => $.page.retry)}
          </Button>
        ) : null}
      </div>
    </div>
  );
}

function IdentityStrip({ overview }: { overview: SkillEvolutionOverview }) {
  const { t } = useT("skill-evolution");
  const fallback = t(($) => $.page.not_available);
  const base = overview.revisions.find(
    (revision) => revision.bundleHash === overview.skill.bundleHash,
  );
  const baseHash = base?.bundleHash ?? overview.proposals[0]?.baseHash ?? null;
  const drifted = Boolean(baseHash && baseHash !== overview.skill.bundleHash);
  const loopMode = overview.loop?.enabled === true
    ? normalizeLoopMode(overview.loop.mode)
    : "disabled";

  return (
    <div className="shrink-0 border-b px-4 py-4 sm:px-6">
      <div className="mx-auto flex max-w-[1600px] flex-wrap items-start gap-x-8 gap-y-3">
        <div className="min-w-0 flex-1">
          <div className="flex flex-wrap items-center gap-2">
            <h1 className="min-w-0 truncate font-mono text-title font-semibold" title={overview.skill.name}>
              {overview.skill.name}
            </h1>
            <EvolutionStatusBadge value={overview.skill.ownership} />
            {overview.skill.forkRequired ? (
              <EvolutionStatusBadge value="external" tone="warning" />
            ) : null}
            {drifted ? <EvolutionStatusBadge value="stale" tone="warning" /> : null}
          </div>
          <p className="mt-1 truncate text-caption text-muted-foreground" title={overview.skill.ownershipReason}>
            {t(($) => $.header.ownership_reason, { reason: overview.skill.ownershipReason || fallback })}
          </p>
        </div>
        <dl className="grid min-w-0 grid-cols-2 gap-x-6 gap-y-2 text-caption sm:grid-cols-3">
          <div className="min-w-0">
            <dt className="text-muted-foreground">{t(($) => $.header.live_hash)}</dt>
            <dd className="truncate font-mono" title={overview.skill.bundleHash}>
              {shortHash(overview.skill.bundleHash, fallback)}
            </dd>
          </div>
          <div className="min-w-0">
            <dt className="text-muted-foreground">{t(($) => $.header.base_hash)}</dt>
            <dd className={cn("truncate font-mono", drifted && "text-warning")} title={baseHash ?? undefined}>
              {shortHash(baseHash, fallback)}
            </dd>
          </div>
          <div className="min-w-0">
            <dt className="text-muted-foreground">{t(($) => $.header.loop_state)}</dt>
            <dd className="mt-0.5"><EvolutionStatusBadge value={loopMode} /></dd>
          </div>
        </dl>
      </div>
    </div>
  );
}

function ProposalPicker({
  proposals,
  selectedId,
  onSelect,
}: {
  proposals: readonly Proposal[];
  selectedId: string;
  onSelect: (id: string) => void;
}) {
  const { t, i18n } = useT("skill-evolution");
  if (proposals.length === 0) {
    return (
      <div className="px-4 py-10 text-center sm:px-6">
        <Sparkles className="mx-auto size-6 text-faint-foreground" aria-hidden="true" />
        <div className="mt-2 text-body font-medium">{t(($) => $.states.no_proposals_title)}</div>
        <div className="mt-0.5 text-caption text-muted-foreground">
          {t(($) => $.states.no_proposals_description)}
        </div>
      </div>
    );
  }
  return (
    <div className="overflow-x-auto border-b">
      <div className="flex min-w-max gap-2 p-3">
        {proposals.map((proposal) => {
          const active = proposal.id === selectedId;
          return (
            <button
              key={proposal.id}
              type="button"
              aria-pressed={active}
              onClick={() => onSelect(proposal.id)}
              className={cn(
                "grid w-52 shrink-0 gap-1 rounded-md border px-3 py-2 text-left transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring",
                active
                  ? "border-foreground/20 bg-muted/60 text-foreground hover:bg-muted/70"
                  : "border-border bg-background text-muted-foreground hover:bg-muted/40 hover:text-foreground",
              )}
            >
              <span className="flex min-w-0 items-center justify-between gap-2">
                <EvolutionStatusBadge
                  value={proposal.state}
                  tone={proposalStatusTone(proposal.state)}
                />
                <span className="truncate font-mono text-micro" title={proposal.candidateHash ?? proposal.baseHash}>
                  {shortHash(proposal.candidateHash ?? proposal.baseHash, t(($) => $.page.not_available))}
                </span>
              </span>
              <span className={cn("truncate text-caption", active && "font-medium")}>
                {t(($) => $.proposal.created, {
                  time: formatDate(proposal.createdAt, i18n.language, t(($) => $.page.not_available)),
                })}
              </span>
            </button>
          );
        })}
      </div>
    </div>
  );
}

export interface SkillEvolutionPageProps {
  workspaceId: string;
  workspaceSlug: string;
  skillId: string;
}

export function SkillEvolutionPage({
  workspaceId,
  workspaceSlug,
  skillId,
}: SkillEvolutionPageProps) {
  const { t } = useT("skill-evolution");
  const navigation = useNavigation();
  const workspacePaths = useMemo(() => paths.workspace(workspaceSlug), [workspaceSlug]);
  const overviewQuery = useQuery(skillEvolutionOverviewOptions(workspaceId, skillId));
  const { refetch: refetchOverview } = overviewQuery;
  const overview = overviewQuery.data;
  const proposals = overview?.proposals ?? EMPTY_PROPOSALS;
  const [selectedProposalId, setSelectedProposalId] = useState("");
  const [queuedRoom, setQueuedRoom] = useState<QueuedRoom | null>(null);
  const [rollbackRelease, setRollbackRelease] = useState<Release | null>(null);

  useEffect(() => {
    if (proposals.some((proposal) => proposal.id === selectedProposalId)) return;
    const preferred = proposals.find((proposal) =>
      isProposalPending(proposal.state) || proposal.state === "ready" || proposal.state === "publication_unknown",
    );
    setSelectedProposalId(preferred?.id ?? proposals[0]?.id ?? "");
  }, [proposals, selectedProposalId]);

  useEffect(() => {
    if (!queuedRoom) return;
    const newProposal = proposals.find(
      (proposal) => !queuedRoom.previousProposalIds.includes(proposal.id),
    );
    if (newProposal) {
      setSelectedProposalId(newProposal.id);
      setQueuedRoom(null);
      return;
    }

    const expiresAt = queuedRoom.queuedAt + ROOM_PROPOSAL_POLL_WINDOW_MS;
    const poll = () => {
      if (Date.now() >= expiresAt) {
        setQueuedRoom(null);
        return;
      }
      void refetchOverview();
    };
    poll();
    const interval = window.setInterval(poll, ROOM_PROPOSAL_POLL_MS);
    return () => window.clearInterval(interval);
  }, [proposals, queuedRoom, refetchOverview]);

  const proposalQuery = useQuery(
    skillEvolutionProposalOptions(workspaceId, selectedProposalId),
  );
  const configure = useConfigureSkillEvolution(workspaceId, skillId);
  const pause = usePauseSkillEvolution(workspaceId, skillId);
  const requestProposal = useRequestSkillEvolutionProposal(workspaceId, skillId);
  const reject = useRejectSkillEvolutionProposal(workspaceId, skillId, selectedProposalId);
  const publish = usePublishSkillEvolutionProposal(workspaceId, skillId, selectedProposalId);
  const rollback = useRollbackSkillEvolutionRelease(workspaceId, skillId);
  const fork = useForkSkillForEvolution(workspaceId, skillId);

  const [rejectOpen, setRejectOpen] = useState(false);
  const [rejectReason, setRejectReason] = useState("");
  const [rejectKey, setRejectKey] = useState("");
  const [publishOpen, setPublishOpen] = useState(false);
  const [publishKey, setPublishKey] = useState("");
  const [rollbackKey, setRollbackKey] = useState("");
  const [forkOpen, setForkOpen] = useState(false);
  const [forkName, setForkName] = useState("");
  const [forkKey, setForkKey] = useState("");
  const pauseKeyRef = useRef("");
  const requestKeyRef = useRef("");

  const reportError = (error: Error | null) => {
    toast.error(error?.message || t(($) => $.toast.failed));
  };

  const saveConfiguration = (input: ConfigureSkillEvolutionInput) => {
    configure.mutate(input, {
      onSuccess: () => toast.success(t(($) => $.toast.configured)),
      onError: reportError,
    });
  };

  const pauseNow = () => {
    if (!pauseKeyRef.current) {
      pauseKeyRef.current = createIntentKey("pause", skillId);
    }
    pause.mutate({ idempotencyKey: pauseKeyRef.current }, {
      onSuccess: () => {
        pauseKeyRef.current = "";
        toast.success(t(($) => $.toast.paused));
      },
      onError: reportError,
    });
  };

  const requestNow = () => {
    if (!requestKeyRef.current) {
      requestKeyRef.current = createIntentKey("proposal", skillId);
    }
    requestProposal.mutate(
      { idempotencyKey: requestKeyRef.current },
      {
        onSuccess: (result) => {
          requestKeyRef.current = "";
          if (result.proposal) {
            setSelectedProposalId(result.proposal.id);
            toast.success(t(($) => $.toast.requested));
          } else if (result.state === "improvement_room_queued" && result.roomId) {
            toast.success(t(($) => $.toast.room_queued));
            setQueuedRoom({
              roomId: result.roomId,
              queuedAt: Date.now(),
              previousProposalIds: proposals.map((proposal) => proposal.id),
            });
          } else {
            toast.success(t(($) => $.toast.requested));
            overviewQuery.refetch();
          }
        },
        onError: reportError,
      },
    );
  };

  const openReject = () => {
    setRejectReason("");
    setRejectKey(createIntentKey("reject", selectedProposalId));
    setRejectOpen(true);
  };

  const submitReject = () => {
    const reason = rejectReason.trim();
    if (!reason) return;
    reject.mutate(
      { reason, idempotencyKey: rejectKey },
      {
        onSuccess: () => {
          setRejectOpen(false);
          toast.success(t(($) => $.toast.rejected));
        },
        onError: reportError,
      },
    );
  };

  const openPublish = () => {
    setPublishKey(createIntentKey("publish", selectedProposalId));
    setPublishOpen(true);
  };

  const submitPublish = () => {
    publish.mutate(
      { idempotencyKey: publishKey },
      {
        onSuccess: () => {
          setPublishOpen(false);
          toast.success(t(($) => $.toast.published));
        },
        onError: reportError,
      },
    );
  };

  const openRollback = (release: Release) => {
    setRollbackRelease(release);
    setRollbackKey(createIntentKey("rollback", release.id));
  };

  const submitRollback = () => {
    if (!rollbackRelease) return;
    rollback.mutate(
      { releaseId: rollbackRelease.id, idempotencyKey: rollbackKey },
      {
        onSuccess: () => {
          setRollbackRelease(null);
          toast.success(t(($) => $.toast.rolled_back));
        },
        onError: reportError,
      },
    );
  };

  const openFork = () => {
    setForkName(overview?.skill.name ? `${overview.skill.name}-workspace` : "");
    setForkKey(createIntentKey("fork", skillId));
    setForkOpen(true);
  };

  const submitFork = () => {
    const name = forkName.trim();
    if (!name) return;
    fork.mutate(
      { name, idempotencyKey: forkKey },
      {
        onSuccess: (created) => {
          setForkOpen(false);
          toast.success(t(($) => $.toast.forked));
          navigation.push(`${workspacePaths.skillDetail(created.id)}/evolution`);
        },
        onError: reportError,
      },
    );
  };

  if (overviewQuery.isPending) return <LoadingPage />;
  if (overviewQuery.isError || !overview) {
    const forbidden = overviewQuery.error instanceof ApiError &&
      (overviewQuery.error.status === 401 || overviewQuery.error.status === 403);
    return (
      <StatePage
        forbidden={forbidden}
        error={overviewQuery.error instanceof Error ? overviewQuery.error : null}
        onRetry={() => overviewQuery.refetch()}
      />
    );
  }

  const pending = queuedRoom !== null || proposals.some((proposal) => isProposalPending(proposal.state));
  const canConfigure = overview.permissions.canConfigure === true;
  const canPublish = overview.permissions.canPublish === true;
  const canFork = overview.permissions.canFork === true;

  return (
    <div className="flex min-h-0 flex-1 flex-col bg-page-canvas">
      <BreadcrumbHeader
        segments={[
          { href: workspacePaths.skills(), label: t(($) => $.page.back) },
          {
            href: workspacePaths.skillDetail(skillId),
            label: <span className="max-w-44 truncate font-mono">{overview.skill.name}</span>,
            className: "flex min-w-0 max-w-44 items-center",
          },
        ]}
        leaf={<span className="truncate text-caption text-foreground">{t(($) => $.page.title)}</span>}
        actions={
          <>
            {pending ? (
              <span
                role="status"
                className="hidden items-center gap-1 text-caption text-muted-foreground sm:inline-flex"
              >
                <Loader2 className="size-3.5 animate-spin" aria-hidden="true" />
                {t(($) => $.page.polling)}
              </span>
            ) : null}
            <Tooltip>
              <TooltipTrigger
                render={
                  <Button
                    type="button"
                    variant="ghost"
                    size="icon-xs"
                    aria-label={overviewQuery.isFetching ? t(($) => $.page.refreshing) : t(($) => $.page.refresh)}
                    onClick={() => overviewQuery.refetch()}
                    disabled={overviewQuery.isFetching}
                  />
                }
              >
                <RefreshCw className={cn(overviewQuery.isFetching && "animate-spin")} aria-hidden="true" />
              </TooltipTrigger>
              <TooltipContent>
                {overviewQuery.isFetching ? t(($) => $.page.refreshing) : t(($) => $.page.refresh)}
              </TooltipContent>
            </Tooltip>
            {overview.skill.forkRequired ? (
              <Button
                type="button"
                size="xs"
                variant="outline"
                disabled={!canFork || fork.isPending}
                onClick={openFork}
                title={!canFork ? t(($) => $.permissions.publish_required) : undefined}
              >
                <GitFork aria-hidden="true" />
                {fork.isPending ? t(($) => $.actions.forking) : t(($) => $.actions.fork)}
              </Button>
            ) : (
              <Button
                type="button"
                size="xs"
                disabled={!canConfigure || !overview.loop || overview.loop.mode === "paused" || requestProposal.isPending}
                onClick={requestNow}
                title={!canConfigure ? t(($) => $.permissions.configure_required) : undefined}
              >
                <Sparkles aria-hidden="true" />
                {requestProposal.isPending ? t(($) => $.actions.requesting) : t(($) => $.actions.request)}
              </Button>
            )}
          </>
        }
      />

      <IdentityStrip overview={overview} />

      {overview.skill.forkRequired ? (
        <Alert className="mx-4 mt-4 w-auto shrink-0 border-warning/40 bg-warning/5 sm:mx-6">
          <ShieldAlert aria-hidden="true" />
          <AlertTitle>{t(($) => $.header.fork_required)}</AlertTitle>
          <AlertDescription>{t(($) => $.permissions.fork_required)}</AlertDescription>
        </Alert>
      ) : !canConfigure && !canPublish ? (
        <div className="mx-4 mt-4 flex shrink-0 items-center gap-1.5 text-caption text-muted-foreground sm:mx-6">
          <Lock className="size-3.5" aria-hidden="true" />
          {t(($) => $.permissions.read_only)}
        </div>
      ) : null}

      {queuedRoom ? (
        <Alert className="mx-4 mt-4 w-auto shrink-0 border-brand/25 bg-brand/7 sm:mx-6">
          <Loader2 className="animate-spin" aria-hidden="true" />
          <AlertTitle>{t(($) => $.states.room_queued)}</AlertTitle>
          <AlertDescription>
            <Button
              type="button"
              variant="link"
              size="xs"
              className="h-auto p-0"
              onClick={() => navigation.push(workspacePaths.roomDetail(queuedRoom.roomId))}
            >
              {t(($) => $.actions.open_room)}
            </Button>
          </AlertDescription>
        </Alert>
      ) : null}

      <main className="min-h-0 flex-1 overflow-y-auto xl:grid xl:grid-cols-[20rem_minmax(0,1fr)_20rem] xl:overflow-hidden">
        <aside className="border-b bg-background xl:min-h-0 xl:overflow-y-auto xl:border-b-0 xl:border-r">
          <LoopConfiguration
            loop={overview.loop}
            canConfigure={canConfigure}
            forkRequired={overview.skill.forkRequired === true}
            saving={configure.isPending}
            pausing={pause.isPending}
            onSave={saveConfiguration}
            onPause={pauseNow}
          />
        </aside>

        <section aria-labelledby="evolution-proposals" className="min-w-0 border-b bg-background xl:min-h-0 xl:overflow-y-auto xl:border-b-0 xl:border-r">
          <div className="flex min-h-12 items-center gap-2 border-b px-4 sm:px-6">
            <Sparkles className="size-4 text-muted-foreground" aria-hidden="true" />
            <h2 id="evolution-proposals" className="text-title-sm font-medium">
              {t(($) => $.proposal.title)}
            </h2>
            {pending ? <Loader2 className="ms-auto size-3.5 animate-spin text-muted-foreground" aria-hidden="true" /> : null}
          </div>
          <ProposalPicker
            proposals={proposals}
            selectedId={selectedProposalId}
            onSelect={setSelectedProposalId}
          />
          {selectedProposalId ? (
            <ProposalReview
              detail={proposalQuery.data}
              loading={proposalQuery.isPending}
              error={proposalQuery.error instanceof Error ? proposalQuery.error : null}
              canReject={canConfigure}
              canPublish={canPublish}
              rejecting={reject.isPending}
              publishing={publish.isPending}
              onRetry={() => proposalQuery.refetch()}
              onReject={openReject}
              onPublish={openPublish}
            />
          ) : null}
        </section>

        <aside className="bg-background xl:min-h-0 xl:overflow-y-auto">
          <ReleaseHistory
            releases={overview.releases}
            revisions={overview.revisions}
            canRollback={canPublish}
            rollbackPending={rollback.isPending}
            onRollback={openRollback}
          />
        </aside>
      </main>

      <Dialog open={rejectOpen} onOpenChange={(open) => !reject.isPending && setRejectOpen(open)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t(($) => $.dialogs.reject_title)}</DialogTitle>
            <DialogDescription>{t(($) => $.dialogs.reject_description)}</DialogDescription>
          </DialogHeader>
          <div className="space-y-1.5">
            <Label htmlFor="evolution-reject-reason">{t(($) => $.dialogs.reject_reason)}</Label>
            <Textarea
              id="evolution-reject-reason"
              autoFocus
              rows={4}
              maxLength={1000}
              value={rejectReason}
              disabled={reject.isPending}
              placeholder={t(($) => $.dialogs.reject_placeholder)}
              onChange={(event) => setRejectReason(event.currentTarget.value)}
            />
          </div>
          <MutationFailure fallback={t(($) => $.toast.failed)} error={reject.error} />
          <DialogFooter>
            <Button type="button" variant="outline" disabled={reject.isPending} onClick={() => setRejectOpen(false)}>
              {t(($) => $.actions.cancel)}
            </Button>
            <Button type="button" variant="destructive" disabled={reject.isPending || !rejectReason.trim()} onClick={submitReject}>
              {reject.isPending ? t(($) => $.actions.rejecting) : t(($) => $.actions.reject)}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <AlertDialog open={publishOpen} onOpenChange={(open) => !publish.isPending && setPublishOpen(open)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t(($) => $.dialogs.publish_title)}</AlertDialogTitle>
            <AlertDialogDescription>{t(($) => $.dialogs.publish_description)}</AlertDialogDescription>
          </AlertDialogHeader>
          <MutationFailure fallback={t(($) => $.toast.failed)} error={publish.error} />
          <AlertDialogFooter>
            <AlertDialogCancel disabled={publish.isPending}>{t(($) => $.actions.cancel)}</AlertDialogCancel>
            <AlertDialogAction
              disabled={publish.isPending}
              onClick={(event) => {
                event.preventDefault();
                submitPublish();
              }}
            >
              {publish.isPending ? t(($) => $.actions.publishing) : t(($) => $.actions.publish)}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      <AlertDialog
        open={rollbackRelease !== null}
        onOpenChange={(open) => !open && !rollback.isPending && setRollbackRelease(null)}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t(($) => $.dialogs.rollback_title)}</AlertDialogTitle>
            <AlertDialogDescription>{t(($) => $.dialogs.rollback_description)}</AlertDialogDescription>
          </AlertDialogHeader>
          <MutationFailure fallback={t(($) => $.toast.failed)} error={rollback.error} />
          <AlertDialogFooter>
            <AlertDialogCancel disabled={rollback.isPending}>{t(($) => $.actions.cancel)}</AlertDialogCancel>
            <AlertDialogAction
              disabled={rollback.isPending}
              onClick={(event) => {
                event.preventDefault();
                submitRollback();
              }}
            >
              {rollback.isPending ? t(($) => $.actions.rolling_back) : t(($) => $.actions.rollback)}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      <Dialog open={forkOpen} onOpenChange={(open) => !fork.isPending && setForkOpen(open)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t(($) => $.dialogs.fork_title)}</DialogTitle>
            <DialogDescription>{t(($) => $.dialogs.fork_description)}</DialogDescription>
          </DialogHeader>
          <div className="space-y-1.5">
            <Label htmlFor="evolution-fork-name">{t(($) => $.dialogs.fork_name)}</Label>
            <Input
              id="evolution-fork-name"
              autoFocus
              maxLength={200}
              value={forkName}
              disabled={fork.isPending}
              placeholder={t(($) => $.dialogs.fork_placeholder)}
              onChange={(event) => setForkName(event.currentTarget.value)}
            />
          </div>
          <MutationFailure fallback={t(($) => $.toast.failed)} error={fork.error} />
          <DialogFooter>
            <Button type="button" variant="outline" disabled={fork.isPending} onClick={() => setForkOpen(false)}>
              {t(($) => $.actions.cancel)}
            </Button>
            <Button type="button" disabled={fork.isPending || !forkName.trim()} onClick={submitFork}>
              <GitFork aria-hidden="true" />
              {fork.isPending ? t(($) => $.actions.forking) : t(($) => $.actions.fork)}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
