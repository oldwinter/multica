"use client";

import { useEffect, useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import {
  Activity,
  AlertTriangle,
  Braces,
  FileSearch,
  Gauge,
  Info,
  Save,
  ShieldCheck,
  Trash2,
} from "lucide-react";
import {
  twinBindingsOptions,
  twinExecutionMetricsOptions,
  useDeleteTwinBinding,
  usePreviewTwinBriefing,
  useUpsertTwinBinding,
  type TwinBindingScope,
  type TwinBindingState,
  type TwinVersion,
} from "@multica/core/twins";
import { Alert, AlertDescription, AlertTitle } from "@multica/ui/components/ui/alert";
import { Badge } from "@multica/ui/components/ui/badge";
import { Button } from "@multica/ui/components/ui/button";
import { Input } from "@multica/ui/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@multica/ui/components/ui/select";
import { Skeleton } from "@multica/ui/components/ui/skeleton";
import { Textarea } from "@multica/ui/components/ui/textarea";
import { Tooltip, TooltipContent, TooltipTrigger } from "@multica/ui/components/ui/tooltip";
import { SegmentedToggle } from "../../common/segmented-toggle";
import { useT } from "../../i18n";

const LONG_LIVED_SCOPES: readonly TwinBindingScope[] = ["workspace", "agent", "project", "issue"];
const BINDING_STATES: readonly TwinBindingState[] = ["off", "preview", "enabled"];

function metric(value: number): string {
  return new Intl.NumberFormat().format(value);
}

export function TwinUsePanel({
  wsId,
  versions,
  currentVersionId,
  canManage,
}: {
  wsId: string;
  versions: readonly TwinVersion[];
  currentVersionId: string;
  canManage: boolean;
}) {
  const { t } = useT("twins");
  const bindingsQuery = useQuery(twinBindingsOptions(wsId));
  const metricsQuery = useQuery(twinExecutionMetricsOptions(wsId));
  const upsertBinding = useUpsertTwinBinding(wsId);
  const deleteBinding = useDeleteTwinBinding(wsId);
  const previewBriefing = usePreviewTwinBriefing();
  const [scopeType, setScopeType] = useState<TwinBindingScope>("workspace");
  const [scopeId, setScopeId] = useState(wsId);
  const [state, setState] = useState<TwinBindingState>("off");
  const [versionId, setVersionId] = useState(currentVersionId);
  const [agentId, setAgentId] = useState("");
  const [projectId, setProjectId] = useState("");
  const [issueId, setIssueId] = useState("");
  const [runId, setRunId] = useState("");
  const [request, setRequest] = useState("");
  const [tags, setTags] = useState("");

  const bindings = bindingsQuery.data?.bindings ?? [];
  const metrics = metricsQuery.data;
  const killSwitch = bindingsQuery.data?.killSwitch ?? metrics?.killSwitch ?? { enabled: false, reason: null };
  const featureEnabled = bindingsQuery.data?.killSwitch.enabled === true && metrics?.killSwitch.enabled === true;
  const stale = (bindingsQuery.isError && !!bindingsQuery.data) ||
    (metricsQuery.isError && !!metricsQuery.data);
  const mayManage = canManage && bindingsQuery.data?.canManage === true && featureEnabled && !stale;
  const effectiveScopeId = scopeType === "workspace" ? wsId : scopeId.trim();
  const matchingBinding = bindings.find((binding) =>
    binding.scopeType === scopeType && binding.scopeId === effectiveScopeId,
  );

  useEffect(() => {
    if (scopeType === "workspace") setScopeId(wsId);
  }, [scopeType, wsId]);

  useEffect(() => {
    setState(matchingBinding?.state ?? "off");
    setVersionId(matchingBinding?.twinVersionId ?? currentVersionId);
  }, [matchingBinding, currentVersionId]);

  const stateDescription = state === "enabled"
    ? t(($) => $.use.state_enabled_description)
    : state === "preview"
      ? t(($) => $.use.state_preview_description)
      : t(($) => $.use.state_off_description);
  const stateOptions = useMemo(() => BINDING_STATES.map((value) => [
    value,
    value === "enabled"
      ? t(($) => $.use.state_enabled)
      : value === "preview"
        ? t(($) => $.use.state_preview)
        : t(($) => $.use.state_off),
  ] as const), [t]);
  const scopeLabel = (value: TwinBindingScope) => value === "workspace"
    ? t(($) => $.use.scope_workspace)
    : value === "agent"
      ? t(($) => $.use.scope_agent)
      : value === "project"
        ? t(($) => $.use.scope_project)
        : value === "issue"
          ? t(($) => $.use.scope_issue)
          : value;
  const stateLabel = (value: TwinBindingState) => value === "enabled"
    ? t(($) => $.use.state_enabled)
    : value === "preview"
      ? t(($) => $.use.state_preview)
      : t(($) => $.use.state_off);
  const scopeOptions = LONG_LIVED_SCOPES.map((value) => ({ value, label: scopeLabel(value) }));
  const versionOptions = versions.map((version) => ({
    value: version.id,
    label: `v${version.version_number} / ${version.content_digest.slice(0, 20)}`,
  }));
  const loading = bindingsQuery.isPending || metricsQuery.isPending;
  const failed = (bindingsQuery.isError && !bindingsQuery.data) ||
    (metricsQuery.isError && !metricsQuery.data);
  const mutationError = upsertBinding.isError || deleteBinding.isError;
  const preview = previewBriefing.data;

  if (loading) {
    return (
      <div className="space-y-4" role="status" aria-label={t(($) => $.use.loading)}>
        <Skeleton className="h-24 w-full" />
        <Skeleton className="h-64 w-full" />
        <Skeleton className="h-36 w-full" />
      </div>
    );
  }

  if (failed) {
    return (
      <Alert variant="destructive">
        <AlertTriangle aria-hidden="true" />
        <AlertTitle>{t(($) => $.use.error_title)}</AlertTitle>
        <AlertDescription className="space-y-3">
          <p>{t(($) => $.use.error_description)}</p>
          <Button variant="outline" onClick={() => void Promise.all([bindingsQuery.refetch(), metricsQuery.refetch()])}>
            {t(($) => $.use.try_again)}
          </Button>
        </AlertDescription>
      </Alert>
    );
  }

  return (
    <div className="space-y-6" data-testid="twin-use-panel">
      <header className="space-y-2">
        <div className="flex flex-wrap items-center gap-2">
          <ShieldCheck className={featureEnabled ? "size-4 text-success" : "size-4 text-muted-foreground"} aria-hidden="true" />
          <h2 className="text-title font-medium text-foreground">{t(($) => $.use.title)}</h2>
          <Badge variant="outline">{t(($) => $.use.default_off)}</Badge>
        </div>
        <p className="max-w-3xl text-body text-muted-foreground">{t(($) => $.use.description)}</p>
      </header>

      {stale ? (
        <Alert>
          <AlertTriangle aria-hidden="true" />
          <AlertTitle>{t(($) => $.use.stale_title)}</AlertTitle>
          <AlertDescription>{t(($) => $.use.stale_description)}</AlertDescription>
        </Alert>
      ) : null}

      {!featureEnabled ? (
        <Alert>
          <AlertTriangle aria-hidden="true" />
          <AlertTitle>{t(($) => $.use.kill_switch_title)}</AlertTitle>
          <AlertDescription>{killSwitch.reason || t(($) => $.use.kill_switch_fallback)}</AlertDescription>
        </Alert>
      ) : !canManage || bindingsQuery.data?.canManage !== true ? (
        <Alert>
          <Info aria-hidden="true" />
          <AlertDescription>{t(($) => $.use.read_only)}</AlertDescription>
        </Alert>
      ) : null}

      <section className="space-y-4 border-y border-border/70 py-5" aria-labelledby="twin-binding-title">
        <div className="space-y-1">
          <h3 id="twin-binding-title" className="text-title font-medium text-foreground">{t(($) => $.use.binding_title)}</h3>
          <p className="text-body text-muted-foreground">{t(($) => $.use.binding_description)}</p>
        </div>
        <div className="grid gap-4 lg:grid-cols-3">
          <label className="grid min-w-0 gap-2 text-label font-medium">
            {t(($) => $.use.scope)}
            <Select items={scopeOptions} value={scopeType} onValueChange={(value) => value && setScopeType(value)}>
              <SelectTrigger className="w-full" aria-label={t(($) => $.use.scope)} disabled={!mayManage}><SelectValue /></SelectTrigger>
              <SelectContent>{scopeOptions.map((item) => <SelectItem key={item.value} value={item.value}>{item.label}</SelectItem>)}</SelectContent>
            </Select>
          </label>
          <label className="grid min-w-0 gap-2 text-label font-medium">
            {t(($) => $.use.scope_id)}
            <Input value={effectiveScopeId} readOnly={scopeType === "workspace"} disabled={!mayManage} placeholder={t(($) => $.use.scope_id_placeholder)} onChange={(event) => setScopeId(event.target.value)} />
          </label>
          <label className="grid min-w-0 gap-2 text-label font-medium">
            {t(($) => $.use.version)}
            <Select items={versionOptions} value={versionId} onValueChange={(value) => value && setVersionId(value)}>
              <SelectTrigger className="w-full" aria-label={t(($) => $.use.version)} disabled={!mayManage || versionOptions.length === 0}><SelectValue /></SelectTrigger>
              <SelectContent>{versionOptions.map((item) => <SelectItem key={item.value} value={item.value}>{item.label}</SelectItem>)}</SelectContent>
            </Select>
          </label>
        </div>
        <fieldset className="grid gap-2 disabled:opacity-60" disabled={!mayManage}>
          <legend className="text-label font-medium">{t(($) => $.use.state)}</legend>
          <SegmentedToggle value={state} options={stateOptions} onChange={setState} buttonClassName="min-h-9 px-3 py-2 text-body" />
          <p className="text-caption text-muted-foreground" aria-live="polite">{stateDescription}</p>
        </fieldset>
        {mutationError ? <p role="alert" className="text-body text-destructive">{t(($) => $.use.mutation_failed)}</p> : null}
        <Button
          disabled={!mayManage || !effectiveScopeId || !versionId || upsertBinding.isPending}
          onClick={() => upsertBinding.mutate({ scopeType, scopeId: effectiveScopeId, state, twinVersionId: versionId })}
        >
          <Save data-icon="inline-start" />
          {upsertBinding.isPending ? t(($) => $.use.saving_binding) : t(($) => $.use.save_binding)}
        </Button>
      </section>

      <section className="space-y-3" aria-labelledby="configured-bindings-title">
        <h3 id="configured-bindings-title" className="text-title font-medium text-foreground">{t(($) => $.use.configured_title)}</h3>
        {bindings.length === 0 ? (
          <p className="text-body text-muted-foreground">{t(($) => $.use.configured_empty)}</p>
        ) : (
          <div className="divide-y divide-border/70">
            {bindings.map((binding) => (
              <div key={binding.id} className="flex min-w-0 items-center gap-3 py-3 first:pt-0 last:pb-0">
                <Badge variant="outline">{scopeLabel(binding.scopeType)}</Badge>
                <span className="min-w-0 flex-1 break-all font-mono text-caption text-foreground">{binding.scopeId}</span>
                <Badge variant={binding.state === "enabled" ? "default" : "secondary"}>{stateLabel(binding.state)}</Badge>
                {mayManage ? (
                  <Tooltip>
                    <TooltipTrigger render={<Button variant="ghost" size="icon-sm" aria-label={t(($) => $.use.delete_binding)} />} onClick={() => deleteBinding.mutate(binding.id)} disabled={deleteBinding.isPending}>
                      <Trash2 aria-hidden="true" />
                    </TooltipTrigger>
                    <TooltipContent>{t(($) => $.use.delete_binding)}</TooltipContent>
                  </Tooltip>
                ) : null}
              </div>
            ))}
          </div>
        )}
      </section>

      <section className="space-y-4 border-y border-border/70 py-5" aria-labelledby="twin-preview-title">
        <div className="space-y-1">
          <div className="flex items-center gap-2"><FileSearch className="size-4 text-muted-foreground" aria-hidden="true" /><h3 id="twin-preview-title" className="text-title font-medium text-foreground">{t(($) => $.use.preview_title)}</h3></div>
          <p className="text-body text-muted-foreground">{t(($) => $.use.preview_description)}</p>
        </div>
        <div className="grid gap-3 md:grid-cols-2">
          <LabeledInput label={t(($) => $.use.agent_id)} value={agentId} onChange={setAgentId} required />
          <LabeledInput label={t(($) => $.use.project_id)} value={projectId} onChange={setProjectId} />
          <LabeledInput label={t(($) => $.use.issue_id)} value={issueId} onChange={setIssueId} />
          <LabeledInput label={t(($) => $.use.run_id)} value={runId} onChange={setRunId} />
        </div>
        <label className="grid gap-2 text-label font-medium">
          {t(($) => $.use.request)}
          <Textarea value={request} maxLength={4000} rows={3} placeholder={t(($) => $.use.request_placeholder)} onChange={(event) => setRequest(event.target.value)} />
        </label>
        <LabeledInput label={t(($) => $.use.tags)} value={tags} onChange={setTags} />
        {previewBriefing.isError ? <p role="alert" className="text-body text-destructive">{t(($) => $.use.preview_failed)}</p> : null}
        <Button
          variant="outline"
          disabled={!agentId.trim() || !request.trim() || previewBriefing.isPending}
          onClick={() => previewBriefing.mutate({
            agentId: agentId.trim(),
            projectId: projectId.trim() || undefined,
            issueId: issueId.trim() || undefined,
            runId: runId.trim() || undefined,
            request: request.trim(),
            tags: tags.split(",").map((tag) => tag.trim()).filter(Boolean),
          })}
        >
          <Braces data-icon="inline-start" />
          {previewBriefing.isPending ? t(($) => $.use.previewing) : t(($) => $.use.preview_action)}
        </Button>
        {preview ? (
          <div className="space-y-4 rounded-lg bg-muted/35 p-4" aria-live="polite">
            <div className="flex flex-wrap items-center gap-2">
              <Badge variant={preview.policy.state === "enabled" ? "default" : "secondary"}>{preview.policy.state}</Badge>
              <span className="text-body text-muted-foreground">{t(($) => $.use.effective_source)}:</span>
              <span className="break-all font-mono text-caption text-foreground">{preview.policy.scopeType ? `${preview.policy.scopeType}:${preview.policy.scopeId}` : t(($) => $.use.no_effective_source)}</span>
            </div>
            <dl className="grid gap-3 text-caption sm:grid-cols-2">
              <div><dt className="text-muted-foreground">{t(($) => $.use.compiler)}</dt><dd className="break-all font-mono text-foreground">{preview.compilerVersion || "-"}</dd></div>
              <div><dt className="text-muted-foreground">{t(($) => $.use.version)}</dt><dd className="break-all font-mono text-foreground">{preview.twinVersion ? `v${preview.twinVersion.versionNumber} / ${preview.twinVersion.contentDigest}` : "-"}</dd></div>
              <div><dt className="text-muted-foreground">{t(($) => $.use.effective_policy)}</dt><dd className="text-foreground">{preview.policy.reason}</dd></div>
              <div><dt className="text-muted-foreground">{t(($) => $.use.exclusions)}</dt><dd className="break-words text-foreground">{[...preview.exclusionReasons, ...preview.policy.exclusions.map((item) => item.code)].join(", ") || "-"}</dd></div>
            </dl>
            <div className="space-y-2">
              <div className="flex flex-wrap items-center justify-between gap-2"><span className="text-label font-medium">{t(($) => $.use.exact_briefing)}</span><span className="text-caption text-muted-foreground">{t(($) => $.use.budget, { bytes: preview.byteCount, tokens: preview.tokenCount })}</span></div>
              {preview.briefing ? <pre className="max-h-64 overflow-auto whitespace-pre-wrap break-words rounded-md bg-background p-3 font-mono text-caption text-foreground">{preview.briefing}</pre> : <p className="text-body text-muted-foreground">{t(($) => $.use.empty_briefing)}</p>}
            </div>
          </div>
        ) : null}
      </section>

      {metrics ? (
        <section className="space-y-3" aria-labelledby="twin-metrics-title">
          <div className="flex items-center gap-2"><Gauge className="size-4 text-muted-foreground" aria-hidden="true" /><h3 id="twin-metrics-title" className="text-title font-medium text-foreground">{t(($) => $.use.metrics_title)}</h3></div>
          <div className="grid gap-2 sm:grid-cols-2 lg:grid-cols-4">
            <Metric label={t(($) => $.use.attributed_runs)} value={metric(metrics.attributedRuns)} />
            <Metric label={t(($) => $.use.helpfulness)} value={metrics.helpfulnessRate === null ? t(($) => $.use.not_available) : `${Math.round(metrics.helpfulnessRate * 100)}%`} />
            <Metric label={t(($) => $.use.feedback)} value={t(($) => $.use.feedback_summary, { total: metric(metrics.feedback.total), helped: metric(metrics.feedback.helped) })} />
            <Metric label={t(($) => $.use.depositions)} value={t(($) => $.use.deposition_summary, { total: metric(metrics.depositions.total), pending: metric(metrics.depositions.pending) })} />
          </div>
        </section>
      ) : null}
    </div>
  );
}

function LabeledInput({ label, value, onChange, required = false }: { label: string; value: string; onChange: (value: string) => void; required?: boolean }) {
  return <label className="grid min-w-0 gap-2 text-label font-medium">{label}<Input value={value} required={required} onChange={(event) => onChange(event.target.value)} /></label>;
}

function Metric({ label, value }: { label: string; value: string }) {
  return (
    <div className="min-w-0 rounded-lg bg-muted/35 p-3">
      <div className="flex items-center gap-1.5 text-caption text-muted-foreground"><Activity className="size-3.5" aria-hidden="true" />{label}</div>
      <p className="mt-1 break-words text-title-sm font-medium text-foreground">{value}</p>
    </div>
  );
}
