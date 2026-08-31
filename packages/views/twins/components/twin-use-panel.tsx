"use client";

import { useEffect, useMemo, useState } from "react";
import { useQueries, useQuery } from "@tanstack/react-query";
import { Activity, AlertTriangle, BarChart3, Braces, FileSearch, Gauge, Info, Pause, Save, ShieldCheck, Trash2, Wrench } from "lucide-react";
import { issueDetailOptions } from "@multica/core/issues/queries";
import { agentListOptions } from "@multica/core/workspace/queries";
import { projectListOptions } from "@multica/core/projects/queries";
import {
  twinActivationReadinessOptions, twinBindingsOptions, twinExecutionMetricsOptions,
  useDeleteTwinBinding, usePauseTwinExecution, usePreviewTwinBriefing, useUpsertTwinBinding,
  type TwinBinding, type TwinBindingScope, type TwinBindingState, type TwinEffectivenessCohort,
  type TwinBriefingPreviewInput, type TwinEffectivenessMetrics, type TwinMaintenanceItem, type TwinVersion,
} from "@multica/core/twins";
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
import { Badge } from "@multica/ui/components/ui/badge";
import { Button } from "@multica/ui/components/ui/button";
import { Input } from "@multica/ui/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@multica/ui/components/ui/select";
import { Skeleton } from "@multica/ui/components/ui/skeleton";
import { Textarea } from "@multica/ui/components/ui/textarea";
import { Tooltip, TooltipContent, TooltipTrigger } from "@multica/ui/components/ui/tooltip";
import { SegmentedToggle } from "../../common/segmented-toggle";
import { useT } from "../../i18n";
import {
  TwinAgentSelector, TwinIssueSelector, TwinProjectSelector,
  type TwinAgentSelection, type TwinIssueSelection, type TwinProjectSelection,
} from "./twin-entity-selectors";
import type { TwinWorkspaceTab } from "./twin-workspace-tabs";

const LONG_LIVED_SCOPES: readonly TwinBindingScope[] = ["workspace", "agent", "project", "issue"];
const BINDING_STATES: readonly TwinBindingState[] = ["off", "preview", "enabled"];

type PreviewKeyInput = {
  readonly workspaceId: string;
  readonly scopeType: TwinBindingScope;
  readonly scopeAgentId: string;
  readonly scopeProjectId: string;
  readonly scopeIssueId: string;
  readonly state: TwinBindingState;
  readonly versionId: string;
  readonly currentVersionId: string;
  readonly previewInput: TwinBriefingPreviewInput | null;
};

function previewKey(input: PreviewKeyInput): string {
  return JSON.stringify(input);
}

function normalizePreviewInput({
  agentId,
  projectId,
  issueId,
  runId,
  request,
  tags,
}: {
  agentId: string;
  projectId?: string;
  issueId?: string;
  runId: string;
  request: string;
  tags: string;
}): TwinBriefingPreviewInput {
  return {
    agentId,
    projectId,
    issueId,
    runId: runId.trim() || undefined,
    request: request.trim(),
    tags: tags.split(",").map((tag) => tag.trim()).filter(Boolean),
  };
}

function metric(value: number): string { return new Intl.NumberFormat().format(value); }
function percentage(value: number): string {
  return new Intl.NumberFormat(undefined, { style: "percent", maximumFractionDigits: 0 }).format(value);
}

export function TwinUsePanel({ wsId, versions, currentVersionId, canManage, onNavigate }: {
  wsId: string;
  versions: readonly TwinVersion[];
  currentVersionId: string;
  canManage: boolean;
  onNavigate: (target: TwinWorkspaceTab) => void;
}) {
  const { t } = useT("twins");
  const activationQuery = useQuery(twinActivationReadinessOptions(wsId));
  const bindingsQuery = useQuery(twinBindingsOptions(wsId));
  const metricsQuery = useQuery(twinExecutionMetricsOptions(wsId));
  const agentsQuery = useQuery(agentListOptions(wsId));
  const projectsQuery = useQuery(projectListOptions(wsId));
  const upsertBinding = useUpsertTwinBinding(wsId);
  const deleteBinding = useDeleteTwinBinding(wsId);
  const pauseExecution = usePauseTwinExecution(wsId);
  const previewBriefing = usePreviewTwinBriefing(wsId);
  const [scopeType, setScopeType] = useState<TwinBindingScope>("workspace");
  const [scopeAgent, setScopeAgent] = useState<TwinAgentSelection | null>(null);
  const [scopeProject, setScopeProject] = useState<TwinProjectSelection | null>(null);
  const [scopeIssue, setScopeIssue] = useState<TwinIssueSelection | null>(null);
  const [state, setState] = useState<TwinBindingState>("off");
  const [versionId, setVersionId] = useState(currentVersionId);
  const [previewAgent, setPreviewAgent] = useState<TwinAgentSelection | null>(null);
  const [previewProject, setPreviewProject] = useState<TwinProjectSelection | null>(null);
  const [previewIssue, setPreviewIssue] = useState<TwinIssueSelection | null>(null);
  const [runId, setRunId] = useState("");
  const [request, setRequest] = useState("");
  const [tags, setTags] = useState("");
  const [confirmPause, setConfirmPause] = useState(false);
  const [deleteBindingId, setDeleteBindingId] = useState<string | null>(null);
  const currentPreviewInput = previewAgent ? normalizePreviewInput({
    agentId: previewAgent.id,
    projectId: previewProject?.id,
    issueId: previewIssue?.id,
    runId,
    request,
    tags,
  }) : null;
  const currentPreviewKey = previewKey({
    workspaceId: wsId,
    scopeType,
    scopeAgentId: scopeAgent?.id ?? "",
    scopeProjectId: scopeProject?.id ?? "",
    scopeIssueId: scopeIssue?.id ?? "",
    state,
    versionId,
    currentVersionId,
    previewInput: currentPreviewInput,
  });
  const [submittedPreviewKey, setSubmittedPreviewKey] = useState<string | null>(null);

  const bindings = useMemo(() => bindingsQuery.data?.bindings ?? [], [bindingsQuery.data?.bindings]);
  const issueBindingIds = useMemo(
    () => [...new Set(bindings.filter((binding) => binding.scopeType === "issue").map((binding) => binding.scopeId))],
    [bindings],
  );
  const issueBindingQueries = useQueries({
    queries: issueBindingIds.map((issueId) => issueDetailOptions(wsId, issueId)),
  });
  const metrics = metricsQuery.data;
  const killSwitch = bindingsQuery.data?.killSwitch ?? metrics?.killSwitch ?? { enabled: false, reason: null };
  const featureEnabled = bindingsQuery.data?.killSwitch.enabled === true && metrics?.killSwitch.enabled === true;
  const stale = (bindingsQuery.isError && !!bindingsQuery.data) || (metricsQuery.isError && !!metricsQuery.data) || (activationQuery.isError && !!activationQuery.data);
  const mayManage = canManage && bindingsQuery.data?.canManage === true && featureEnabled && !stale;
  const mayPause = canManage && bindingsQuery.data?.canManage === true && Boolean(currentVersionId) && !stale && bindings.some((binding) => binding.state !== "off");
  const effectiveScopeId = scopeType === "workspace" ? wsId : scopeType === "agent" ? scopeAgent?.id ?? "" : scopeType === "project" ? scopeProject?.id ?? "" : scopeIssue?.id ?? "";
  const matchingBinding = bindings.find((binding) => binding.scopeType === scopeType && binding.scopeId === effectiveScopeId);

  useEffect(() => {
    setState(matchingBinding?.state ?? "off");
    setVersionId(matchingBinding?.twinVersionId ?? currentVersionId);
  }, [matchingBinding, currentVersionId]);

  const agentsById = useMemo(() => new Map((agentsQuery.data ?? []).map((agent) => [agent.id, agent.name])), [agentsQuery.data]);
  const projectsById = useMemo(() => new Map((projectsQuery.data ?? []).map((project) => [project.id, project.title])), [projectsQuery.data]);
  const issuesById = useMemo(
    () => new Map(issueBindingQueries.flatMap((query) => query.data ? [[query.data.id, `${query.data.identifier} ${query.data.title}`] as const] : [])),
    [issueBindingQueries],
  );
  const versionsById = useMemo(() => new Map(versions.map((version) => [version.id, `v${version.version_number}`])), [versions]);
  const stateDescription = state === "enabled" ? t(($) => $.use.state_enabled_description) : state === "preview" ? t(($) => $.use.state_preview_description) : t(($) => $.use.state_off_description);
  const stateOptions = useMemo(() => BINDING_STATES.map((value) => [value, value === "enabled" ? t(($) => $.use.state_enabled) : value === "preview" ? t(($) => $.use.state_preview) : t(($) => $.use.state_off)] as const), [t]);
  const scopeLabel = (value: TwinBindingScope) => value === "workspace" ? t(($) => $.use.scope_workspace) : value === "agent" ? t(($) => $.use.scope_agent) : value === "project" ? t(($) => $.use.scope_project) : value === "issue" ? t(($) => $.use.scope_issue) : value;
  const stateLabel = (value: TwinBindingState) => value === "enabled" ? t(($) => $.use.state_enabled) : value === "preview" ? t(($) => $.use.state_preview) : t(($) => $.use.state_off);
  const scopeOptions = LONG_LIVED_SCOPES.map((value) => ({ value, label: scopeLabel(value) }));
  const versionOptions = versions.map((version) => ({ value: version.id, label: `v${version.version_number} / ${version.content_digest.slice(0, 20)}` }));
  const loading = bindingsQuery.isPending || metricsQuery.isPending || activationQuery.isPending;
  const failed = (bindingsQuery.isError && !bindingsQuery.data) || (metricsQuery.isError && !metricsQuery.data) || (activationQuery.isError && !activationQuery.data);
  const mutationError = upsertBinding.isError || deleteBinding.isError || pauseExecution.isError;
  const previewIsCurrent = submittedPreviewKey === currentPreviewKey
    && previewBriefing.isSuccess
    && !previewBriefing.isPending;
  const preview = previewIsCurrent ? previewBriefing.data : undefined;
  const previewIsOutOfDate = Boolean(previewBriefing.data) && submittedPreviewKey !== currentPreviewKey;
  const entityLabel = (binding: TwinBinding): string => {
    if (binding.scopeType === "workspace") return t(($) => $.use.workspace_default);
    if (binding.scopeType === "agent") return agentsById.get(binding.scopeId) ?? `${t(($) => $.use.unavailable_agent)} (${binding.scopeId})`;
    if (binding.scopeType === "project") return projectsById.get(binding.scopeId) ?? `${t(($) => $.use.unavailable_project)} (${binding.scopeId})`;
    if (binding.scopeType === "issue") return issuesById.get(binding.scopeId) ?? (scopeIssue?.id === binding.scopeId ? `${scopeIssue.identifier} ${scopeIssue.title}` : `${t(($) => $.use.unavailable_issue)} (${binding.scopeId})`);
    return t(($) => $.use.unavailable_target);
  };
  const deleteBindingTarget = deleteBindingId
    ? bindings.find((binding) => binding.id === deleteBindingId) ?? null
    : null;

  if (loading) return <div className="space-y-4" role="status" aria-label={t(($) => $.use.loading)}><Skeleton className="h-24 w-full" /><Skeleton className="h-64 w-full" /><Skeleton className="h-36 w-full" /></div>;
  if (failed) return (
    <Alert variant="destructive"><AlertTriangle aria-hidden="true" /><AlertTitle>{t(($) => $.use.error_title)}</AlertTitle><AlertDescription className="space-y-3"><p>{t(($) => $.use.error_description)}</p><Button variant="outline" onClick={() => void Promise.all([activationQuery.refetch(), bindingsQuery.refetch(), metricsQuery.refetch()])}>{t(($) => $.use.try_again)}</Button></AlertDescription></Alert>
  );

  return (
    <div className="space-y-7" data-testid="twin-use-panel">
      <header className="space-y-2"><div className="flex flex-wrap items-center gap-2"><ShieldCheck className={featureEnabled ? "size-4 text-success" : "size-4 text-muted-foreground"} aria-hidden="true" /><h2 className="text-title font-medium text-foreground">{t(($) => $.use.title)}</h2><Badge variant="outline">{t(($) => $.use.default_off)}</Badge></div><p className="max-w-3xl text-body text-muted-foreground">{t(($) => $.use.description)}</p></header>
      {stale ? <Alert><AlertTriangle aria-hidden="true" /><AlertTitle>{t(($) => $.use.stale_title)}</AlertTitle><AlertDescription>{t(($) => $.use.stale_description)}</AlertDescription></Alert> : null}
      {!featureEnabled ? <Alert><AlertTriangle aria-hidden="true" /><AlertTitle>{t(($) => $.use.kill_switch_title)}</AlertTitle><AlertDescription>{killSwitch.reason || t(($) => $.use.kill_switch_fallback)}</AlertDescription></Alert> : !canManage || bindingsQuery.data?.canManage !== true ? <Alert><Info aria-hidden="true" /><AlertDescription>{t(($) => $.use.read_only)}</AlertDescription></Alert> : null}

      <section className="space-y-4 border-y border-border/70 py-5" aria-labelledby="twin-binding-title">
        <div className="space-y-1"><h3 id="twin-binding-title" className="text-title font-medium text-foreground">{t(($) => $.use.binding_title)}</h3><p className="text-body text-muted-foreground">{t(($) => $.use.binding_description)}</p></div>
        <div className="grid gap-4 lg:grid-cols-3">
          <label className="grid min-w-0 gap-2 text-label font-medium">{t(($) => $.use.scope)}<Select items={scopeOptions} value={scopeType} onValueChange={(value) => value && setScopeType(value)}><SelectTrigger className="w-full" aria-label={t(($) => $.use.scope)} disabled={!mayManage}><SelectValue /></SelectTrigger><SelectContent>{scopeOptions.map((item) => <SelectItem key={item.value} value={item.value}>{item.label}</SelectItem>)}</SelectContent></Select></label>
          <div className="grid min-w-0 gap-2 text-label font-medium"><span>{t(($) => $.use.target)}</span>{scopeType === "workspace" ? <div className="flex h-9 items-center rounded-md border border-input bg-muted/25 px-3 text-body text-muted-foreground">{t(($) => $.use.workspace_default)}</div> : scopeType === "agent" ? <TwinAgentSelector wsId={wsId} value={scopeAgent} onChange={setScopeAgent} disabled={!mayManage} ariaLabel={t(($) => $.use.select_agent)} /> : scopeType === "project" ? <TwinProjectSelector wsId={wsId} value={scopeProject} onChange={setScopeProject} disabled={!mayManage} ariaLabel={t(($) => $.use.select_project)} /> : <TwinIssueSelector wsId={wsId} value={scopeIssue} onChange={setScopeIssue} disabled={!mayManage} ariaLabel={t(($) => $.use.select_issue)} />}</div>
          <label className="grid min-w-0 gap-2 text-label font-medium">{t(($) => $.use.version)}<Select items={versionOptions} value={versionId} onValueChange={(value) => value && setVersionId(value)}><SelectTrigger className="w-full" aria-label={t(($) => $.use.version)} disabled={!mayManage || versionOptions.length === 0}><SelectValue /></SelectTrigger><SelectContent>{versionOptions.map((item) => <SelectItem key={item.value} value={item.value}>{item.label}</SelectItem>)}</SelectContent></Select></label>
        </div>
        <fieldset className="grid gap-2 disabled:opacity-60" disabled={!mayManage}><legend className="text-label font-medium">{t(($) => $.use.state)}</legend><SegmentedToggle value={state} options={stateOptions} onChange={setState} buttonClassName="min-h-9 px-3 py-2 text-body" /><p className="text-caption text-muted-foreground" aria-live="polite">{stateDescription}</p></fieldset>
        {mutationError ? <p role="alert" className="text-body text-destructive">{t(($) => $.use.mutation_failed)}</p> : null}
        <Button disabled={!mayManage || !effectiveScopeId || !versionId || upsertBinding.isPending} onClick={() => upsertBinding.mutate({ scopeType, scopeId: effectiveScopeId, state, twinVersionId: versionId })}><Save data-icon="inline-start" />{upsertBinding.isPending ? t(($) => $.use.saving_binding) : t(($) => $.use.save_binding)}</Button>
      </section>

      <section className="space-y-3" aria-labelledby="configured-bindings-title"><h3 id="configured-bindings-title" className="text-title font-medium text-foreground">{t(($) => $.use.configured_title)}</h3>{bindings.length === 0 ? <p className="text-body text-muted-foreground">{t(($) => $.use.configured_empty)}</p> : <div className="divide-y divide-border/70">{bindings.map((binding) => <div key={binding.id} className="flex min-w-0 items-center gap-3 py-3 first:pt-0 last:pb-0"><Badge variant="outline">{scopeLabel(binding.scopeType)}</Badge><span className="min-w-0 flex-1 truncate text-body text-foreground">{entityLabel(binding)}</span><span className="text-caption text-muted-foreground">{versionsById.get(binding.twinVersionId) ?? t(($) => $.use.unavailable_version)}</span><Badge variant={binding.state === "enabled" ? "default" : "secondary"}>{stateLabel(binding.state)}</Badge>{mayManage ? <Tooltip><TooltipTrigger render={<Button variant="ghost" size="icon-sm" aria-label={t(($) => $.use.delete_binding)} />} onClick={() => { deleteBinding.reset(); setDeleteBindingId(binding.id); }} disabled={deleteBinding.isPending}><Trash2 aria-hidden="true" /></TooltipTrigger><TooltipContent>{t(($) => $.use.delete_binding)}</TooltipContent></Tooltip> : null}</div>)}</div>}</section>

      <AlertDialog
        open={deleteBindingId !== null}
        onOpenChange={(open) => {
          if (!open && !deleteBinding.isPending) setDeleteBindingId(null);
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t(($) => $.use.delete_binding_title)}</AlertDialogTitle>
            <AlertDialogDescription>
              {t(($) => $.use.delete_binding_description, {
                target: deleteBindingTarget ? entityLabel(deleteBindingTarget) : t(($) => $.use.unavailable_target),
              })}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={deleteBinding.isPending}>{t(($) => $.actions.cancel)}</AlertDialogCancel>
            <AlertDialogAction
              variant="destructive"
              disabled={deleteBinding.isPending || deleteBindingId === null}
              onClick={() => {
                if (!deleteBindingId) return;
                deleteBinding.mutate(deleteBindingId, {
                  onSuccess: () => setDeleteBindingId(null),
                });
              }}
            >
              {deleteBinding.isPending ? t(($) => $.use.deleting_binding) : t(($) => $.use.confirm_delete_binding)}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      <section className="space-y-4 border-y border-border/70 py-5" aria-labelledby="twin-preview-title">
        <div className="space-y-1"><div className="flex items-center gap-2"><FileSearch className="size-4 text-muted-foreground" aria-hidden="true" /><h3 id="twin-preview-title" className="text-title font-medium text-foreground">{t(($) => $.use.preview_title)}</h3></div><p className="text-body text-muted-foreground">{t(($) => $.use.preview_description)}</p></div>
        <div className="grid gap-3 md:grid-cols-2"><SelectorField label={t(($) => $.use.agent)}><TwinAgentSelector wsId={wsId} value={previewAgent} onChange={setPreviewAgent} ariaLabel={t(($) => $.use.agent)} /></SelectorField><SelectorField label={t(($) => $.use.project_optional)}><TwinProjectSelector wsId={wsId} value={previewProject} optional onChange={(project) => { setPreviewProject(project); if (previewIssue && project?.id !== previewIssue.project_id) setPreviewIssue(null); }} ariaLabel={t(($) => $.use.project_optional)} /></SelectorField><SelectorField label={t(($) => $.use.issue_optional)}><TwinIssueSelector wsId={wsId} value={previewIssue} optional onChange={(issue) => { setPreviewIssue(issue); if (issue?.project_id) { const project = (projectsQuery.data ?? []).find((candidate) => candidate.id === issue.project_id); if (project) setPreviewProject({ id: project.id, title: project.title, status: project.status, icon: project.icon }); } }} ariaLabel={t(($) => $.use.issue_optional)} /></SelectorField></div>
        <label className="grid gap-2 text-label font-medium">{t(($) => $.use.request)}<Textarea value={request} maxLength={4000} rows={3} placeholder={t(($) => $.use.request_placeholder)} onChange={(event) => setRequest(event.target.value)} /></label>
        <details className="group text-body"><summary className="cursor-pointer text-label font-medium text-muted-foreground">{t(($) => $.use.advanced_context)}</summary><div className="mt-3 grid gap-3 md:grid-cols-2"><LabeledInput label={t(($) => $.use.run_id)} value={runId} onChange={setRunId} /><LabeledInput label={t(($) => $.use.tags)} value={tags} onChange={setTags} /></div></details>
        {previewBriefing.isError ? <p role="alert" className="text-body text-destructive">{t(($) => $.use.preview_failed)}</p> : previewIsOutOfDate ? <p role="alert" className="text-body text-destructive">{t(($) => $.use.preview_stale)}</p> : null}
        <Button variant="outline" disabled={!previewAgent || !request.trim() || previewBriefing.isPending} onClick={() => {
          if (!currentPreviewInput) return;
          const input = currentPreviewInput;
          setSubmittedPreviewKey(currentPreviewKey);
          previewBriefing.mutate(input);
        }}><Braces data-icon="inline-start" />{previewBriefing.isPending ? t(($) => $.use.previewing) : t(($) => $.use.preview_action)}</Button>
        {preview ? <div className="space-y-4 rounded-lg bg-muted/35 p-4" aria-live="polite"><div className="flex flex-wrap items-center gap-2"><Badge variant={preview.policy.state === "enabled" ? "default" : "secondary"}>{stateLabel(preview.policy.state)}</Badge><span className="text-body text-muted-foreground">{t(($) => $.use.effective_source)}:</span><span className="truncate text-body text-foreground">{previewSourceLabel(preview.policy.scopeType, preview.policy.scopeId, { workspace: t(($) => $.use.workspace_default), agent: previewAgent, project: previewProject, issue: previewIssue, unavailable: t(($) => $.use.unavailable_target), none: t(($) => $.use.no_effective_source) })}</span></div><dl className="grid gap-3 text-caption sm:grid-cols-2"><div><dt className="text-muted-foreground">{t(($) => $.use.compiler)}</dt><dd className="break-all font-mono text-foreground">{preview.compilerVersion || "-"}</dd></div><div><dt className="text-muted-foreground">{t(($) => $.use.version)}</dt><dd className="text-foreground">{preview.twinVersion ? `v${preview.twinVersion.versionNumber}` : "-"}</dd></div><div><dt className="text-muted-foreground">{t(($) => $.use.effective_policy)}</dt><dd className="text-foreground">{preview.policy.reason}</dd></div><div><dt className="text-muted-foreground">{t(($) => $.use.exclusions)}</dt><dd className="break-words text-foreground">{[...preview.exclusionReasons, ...preview.policy.exclusions.map((item) => item.code)].join(", ") || "-"}</dd></div></dl><div className="space-y-2"><div className="flex flex-wrap items-center justify-between gap-2"><span className="text-label font-medium">{t(($) => $.use.exact_briefing)}</span><span className="text-caption text-muted-foreground">{t(($) => $.use.budget, { bytes: preview.byteCount, tokens: preview.tokenCount })}</span></div>{preview.briefing ? <pre className="max-h-64 overflow-auto whitespace-pre-wrap break-words rounded-md bg-background p-3 font-mono text-caption text-foreground">{preview.briefing}</pre> : <p className="text-body text-muted-foreground">{t(($) => $.use.empty_briefing)}</p>}</div></div> : null}
      </section>

      <MaintenanceQueue items={activationQuery.data?.maintenance ?? []} onNavigate={onNavigate} />
      {metrics ? <Effectiveness metrics={metrics.effectiveness} mayPause={mayPause} confirmPause={confirmPause} pausePending={pauseExecution.isPending} pauseFailed={pauseExecution.isError} onStartPause={() => setConfirmPause(true)} onCancelPause={() => setConfirmPause(false)} onConfirmPause={() => pauseExecution.mutate(undefined, { onSuccess: () => setConfirmPause(false) })} /> : null}
      {metrics ? <section className="space-y-3" aria-labelledby="twin-metrics-title"><div className="flex items-center gap-2"><Gauge className="size-4 text-muted-foreground" aria-hidden="true" /><h3 id="twin-metrics-title" className="text-title font-medium text-foreground">{t(($) => $.use.metrics_title)}</h3></div><div className="grid gap-2 sm:grid-cols-2 lg:grid-cols-4"><Metric label={t(($) => $.use.attributed_runs)} value={metric(metrics.attributedRuns)} /><Metric label={t(($) => $.use.helpfulness)} value={metrics.helpfulnessRate === null ? t(($) => $.use.not_available) : percentage(metrics.helpfulnessRate)} /><Metric label={t(($) => $.use.feedback)} value={t(($) => $.use.feedback_summary, { total: metric(metrics.feedback.total), helped: metric(metrics.feedback.helped) })} /><Metric label={t(($) => $.use.depositions)} value={t(($) => $.use.deposition_summary, { total: metric(metrics.depositions.total), pending: metric(metrics.depositions.pending) })} /></div></section> : null}
    </div>
  );
}

function SelectorField({ label, children }: { label: string; children: React.ReactNode }) { return <div className="grid min-w-0 gap-2 text-label font-medium"><span>{label}</span>{children}</div>; }
function LabeledInput({ label, value, onChange }: { label: string; value: string; onChange: (value: string) => void }) { return <label className="grid min-w-0 gap-2 text-label font-medium">{label}<Input value={value} onChange={(event) => onChange(event.target.value)} /></label>; }
function Metric({ label, value }: { label: string; value: string }) { return <div className="min-w-0 rounded-lg bg-muted/35 p-3"><div className="flex items-center gap-1.5 text-caption text-muted-foreground"><Activity className="size-3.5" aria-hidden="true" />{label}</div><p className="mt-1 break-words text-title-sm font-medium text-foreground">{value}</p></div>; }

function MaintenanceQueue({ items, onNavigate }: { items: readonly TwinMaintenanceItem[]; onNavigate: (target: TwinWorkspaceTab) => void }) {
  const { t } = useT("twins");
  return <section className="space-y-3" aria-labelledby="twin-maintenance-title"><div className="flex items-center gap-2"><Wrench className="size-4 text-muted-foreground" aria-hidden="true" /><h3 id="twin-maintenance-title" className="text-title font-medium text-foreground">{t(($) => $.use.maintenance_title)}</h3></div>{items.length === 0 ? <p className="text-body text-muted-foreground">{t(($) => $.use.maintenance_empty)}</p> : <div className="divide-y divide-border/70">{items.map((item) => <div key={item.id} className="flex min-w-0 flex-col gap-2 py-3 first:pt-0 last:pb-0 sm:flex-row sm:items-center"><Badge variant={item.severity === "high" ? "destructive" : "outline"}>{maintenanceSeverity(item, t)}</Badge><div className="min-w-0 flex-1"><p className="text-body font-medium text-foreground">{maintenanceTitle(item, t)}</p><p className="text-caption text-muted-foreground">{item.count > 0 ? t(($) => $.use.maintenance_count, { count: item.count }) : t(($) => $.use.maintenance_owner)}</p></div><Button variant="outline" size="sm" onClick={() => onNavigate(item.action === "review_twin" || item.action === "generate_twin" || item.action === "review_deposition" ? "twin" : "use")}>{t(($) => $.use.maintenance_review)}</Button></div>)}</div>}</section>;
}

function Effectiveness({
  metrics,
  mayPause,
  confirmPause,
  pausePending,
  pauseFailed,
  onStartPause,
  onCancelPause,
  onConfirmPause,
}: {
  metrics: TwinEffectivenessMetrics;
  mayPause: boolean;
  confirmPause: boolean;
  pausePending: boolean;
  pauseFailed: boolean;
  onStartPause: () => void;
  onCancelPause: () => void;
  onConfirmPause: () => void;
}) {
  const { t } = useT("twins");
  return (
    <section className="space-y-4 border-y border-border/70 py-5" aria-labelledby="twin-effectiveness-title">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="space-y-1">
          <div className="flex items-center gap-2">
            <BarChart3 className="size-4 text-muted-foreground" aria-hidden="true" />
            <h3 id="twin-effectiveness-title" className="text-title font-medium text-foreground">
              {t(($) => $.use.effectiveness_title)}
            </h3>
          </div>
          <p className="max-w-3xl text-body text-muted-foreground">{t(($) => $.use.effectiveness_description)}</p>
        </div>
        <div className="flex flex-wrap items-center justify-end gap-2">
          <Badge variant="outline">{t(($) => $.use.window_days, { count: metrics.windowDays })}</Badge>
          <Badge variant="outline">{t(($) => $.use.minimum_sample, { count: metrics.minimumSample })}</Badge>
          {mayPause && !confirmPause ? (
            <Button variant="outline" onClick={onStartPause}>
              <Pause data-icon="inline-start" />
              {t(($) => $.use.pause_action)}
            </Button>
          ) : null}
        </div>
      </div>
      {confirmPause ? (
        <Alert>
          <Pause aria-hidden="true" />
          <AlertTitle>{t(($) => $.use.pause_confirm_title)}</AlertTitle>
          <AlertDescription className="space-y-3">
            <p>{t(($) => $.use.pause_confirm_description)}</p>
            <div className="flex flex-wrap gap-2">
              <Button variant="destructive" disabled={pausePending} onClick={onConfirmPause}>
                {pausePending ? t(($) => $.use.pausing) : t(($) => $.use.pause_confirm)}
              </Button>
              <Button variant="outline" disabled={pausePending} onClick={onCancelPause}>
                {t(($) => $.actions.cancel)}
              </Button>
            </div>
          </AlertDescription>
        </Alert>
      ) : null}
      {pauseFailed ? <p role="alert" className="text-body text-destructive">{t(($) => $.use.mutation_failed)}</p> : null}
      <p className="text-caption text-muted-foreground">
        {t(($) => $.use.cohort_definition, { definition: metrics.cohortDefinition })}
      </p>
      <p className="text-caption text-muted-foreground">
        {metrics.comparison.eligible
          ? t(($) => $.use.comparison_ready, { control: metrics.comparison.controlState ?? "off" })
          : t(($) => $.use.comparison_waiting)}
      </p>
      <div
        role="region"
        aria-label={t(($) => $.use.cohort)}
        tabIndex={0}
        onKeyDown={(event) => {
          if (event.key !== "ArrowRight" && event.key !== "ArrowLeft") return;
          event.preventDefault();
          event.currentTarget.scrollLeft += event.key === "ArrowRight" ? 240 : -240;
        }}
        className="overflow-x-auto focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring"
      >
        <table className="w-full min-w-[1040px] border-collapse text-left text-caption">
          <thead>
            <tr className="border-b border-border text-muted-foreground">
              <th className="py-2 pr-3 font-medium">{t(($) => $.use.cohort)}</th>
              <th className="px-3 py-2 font-medium">{t(($) => $.use.sample)}</th>
              <th className="px-3 py-2 font-medium">{t(($) => $.use.feedback_coverage)}</th>
              <th className="px-3 py-2 font-medium">{t(($) => $.use.feedback_distribution)}</th>
              <th className="px-3 py-2 font-medium">{t(($) => $.use.helped_rate)}</th>
              <th className="px-3 py-2 font-medium">{t(($) => $.use.revision_rate)}</th>
              <th className="px-3 py-2 font-medium">{t(($) => $.use.deposition_acceptance)}</th>
              <th className="px-3 py-2 font-medium">{t(($) => $.use.latency)}</th>
              <th className="px-3 py-2 font-medium">{t(($) => $.use.briefing_overhead)}</th>
              <th className="py-2 pl-3 font-medium">{t(($) => $.use.bounded_cost)}</th>
            </tr>
          </thead>
          <tbody>{metrics.cohorts.map((cohort) => <CohortRow key={cohort.policyState} cohort={cohort} />)}</tbody>
        </table>
      </div>
      <p className="text-caption text-muted-foreground">{metrics.revisionMeasure}. {metrics.costMeasure}.</p>
    </section>
  );
}

function CohortRow({ cohort }: { cohort: TwinEffectivenessCohort }) {
  const { t } = useT("twins");
  const suppressed = t(($) => $.use.suppressed);
  const feedbackDistribution = cohort.feedbackHelped === null || cohort.feedbackIrrelevant === null || cohort.feedbackMismatch === null
    ? suppressed
    : `${metric(cohort.feedbackHelped)} / ${metric(cohort.feedbackIrrelevant)} / ${metric(cohort.feedbackMismatch)}`;
  const depositionAcceptance = cohort.depositionAcceptanceRate === null || cohort.depositionAccepted === null || cohort.depositionTotal === null
    ? suppressed
    : `${percentage(cohort.depositionAcceptanceRate)} (${metric(cohort.depositionAccepted)}/${metric(cohort.depositionTotal)})`;
  return (
    <tr className="border-b border-border/70 last:border-0">
      <td className="py-3 pr-3"><Badge variant={cohort.policyState === "enabled" ? "default" : "outline"}>{cohort.policyState}</Badge></td>
      <td className="px-3 py-3">{metric(cohort.sampleSize)}</td>
      <td className="px-3 py-3">{percentage(cohort.feedbackCoverage)} ({metric(cohort.feedbackTotal)})</td>
      <td className="px-3 py-3">{feedbackDistribution}</td>
      <td className="px-3 py-3">{cohort.helpedRate === null ? suppressed : percentage(cohort.helpedRate)}</td>
      <td className="px-3 py-3">{cohort.revisionRate === null ? suppressed : percentage(cohort.revisionRate)}</td>
      <td className="px-3 py-3">{depositionAcceptance}</td>
      <td className="px-3 py-3">{cohort.averageLatencyMs === null ? suppressed : formatDuration(cohort.averageLatencyMs)}</td>
      <td className="px-3 py-3">{cohort.averageBriefingTokens === null ? suppressed : t(($) => $.use.tokens, { count: cohort.averageBriefingTokens })}</td>
      <td className="py-3 pl-3">{cohort.costUsdTicks === null ? suppressed : `${formatCost(cohort.costUsdTicks)} / ${percentage(cohort.costCoverage)}`}</td>
    </tr>
  );
}

function previewSourceLabel(scopeType: TwinBindingScope | null, scopeId: string | null, labels: { workspace: string; agent: TwinAgentSelection | null; project: TwinProjectSelection | null; issue: TwinIssueSelection | null; unavailable: string; none: string }): string {
  if (!scopeType || !scopeId) return labels.none;
  if (scopeType === "workspace") return labels.workspace;
  if (scopeType === "agent") return labels.agent?.id === scopeId ? labels.agent.name : labels.unavailable;
  if (scopeType === "project") return labels.project?.id === scopeId ? labels.project.title : labels.unavailable;
  if (scopeType === "issue") return labels.issue?.id === scopeId ? `${labels.issue.identifier} ${labels.issue.title}` : labels.unavailable;
  return labels.unavailable;
}

type Translate = ReturnType<typeof useT<"twins">>["t"];
function maintenanceTitle(item: TwinMaintenanceItem, t: Translate): string {
  switch (item.kind) {
    case "pending_proposal": return t(($) => $.use.maintenance_pending_proposal);
    case "stale_signed_version": return t(($) => $.use.maintenance_stale_version, { version: item.versionNumber ?? 0 });
    case "repeated_mismatch": return t(($) => $.use.maintenance_repeated_mismatch);
    case "low_confidence": return t(($) => $.use.maintenance_low_confidence);
    case "pending_deposition": return t(($) => $.use.maintenance_pending_deposition);
  }
}
function maintenanceSeverity(item: TwinMaintenanceItem, t: Translate): string { return item.severity === "high" ? t(($) => $.use.severity_high) : item.severity === "medium" ? t(($) => $.use.severity_medium) : t(($) => $.use.severity_low); }
function formatDuration(milliseconds: number): string { return milliseconds < 60_000 ? `${Math.round(milliseconds / 1000)}s` : `${Math.round(milliseconds / 60_000)}m`; }
function formatCost(ticks: number): string { return new Intl.NumberFormat(undefined, { style: "currency", currency: "USD", maximumFractionDigits: 4 }).format(ticks / 10_000_000_000); }
