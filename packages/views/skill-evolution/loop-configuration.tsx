"use client";

import { useEffect, useMemo, useState } from "react";
import type {
  ConfigureSkillEvolutionInput,
  SkillEvolutionLoop,
} from "@multica/core/skill-evolution";
import { Clock3, Pause, Save, Settings2 } from "lucide-react";
import { Button } from "@multica/ui/components/ui/button";
import { Input } from "@multica/ui/components/ui/input";
import { Label } from "@multica/ui/components/ui/label";
import { Switch } from "@multica/ui/components/ui/switch";
import { SegmentedToggle } from "../common/segmented-toggle";
import { useT } from "../i18n";

type EditableLoopMode = "observe" | "propose" | "paused";
type LoopDraft = ConfigureSkillEvolutionInput & { mode: EditableLoopMode };

const DEFAULT_LOOP: LoopDraft = {
  enabled: false,
  mode: "observe",
  cooldownSeconds: 3600,
  minimumSignals: 2,
  maxEvidenceRefs: 20,
  maxReplaySamples: 8,
  maxCostUsdTicks: 10000,
  policyVersion: "v1",
};

function draftFromLoop(loop: SkillEvolutionLoop | null): LoopDraft {
  if (!loop) return DEFAULT_LOOP;
  return {
    enabled: loop.enabled === true,
    mode:
      loop.mode === "observe" || loop.mode === "propose" || loop.mode === "paused"
        ? loop.mode
        : "observe",
    cooldownSeconds: loop.cooldownSeconds,
    minimumSignals: loop.minimumSignals,
    maxEvidenceRefs: loop.maxEvidenceRefs,
    maxReplaySamples: loop.maxReplaySamples,
    maxCostUsdTicks: loop.maxCostUsdTicks,
    policyVersion: loop.policyVersion,
  };
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

function validLoopDraft(draft: LoopDraft): boolean {
  return Number.isInteger(draft.cooldownSeconds) && draft.cooldownSeconds >= 60 && draft.cooldownSeconds <= 2_592_000 &&
    Number.isInteger(draft.minimumSignals) && draft.minimumSignals >= 1 && draft.minimumSignals <= 100 &&
    Number.isInteger(draft.maxEvidenceRefs) && draft.maxEvidenceRefs >= draft.minimumSignals && draft.maxEvidenceRefs <= 100 &&
    Number.isInteger(draft.maxReplaySamples) && draft.maxReplaySamples >= 1 && draft.maxReplaySamples <= 32 &&
    Number.isInteger(draft.maxCostUsdTicks) && draft.maxCostUsdTicks >= 0 && draft.maxCostUsdTicks <= 1_000_000_000 &&
    draft.policyVersion.trim().length > 0 && draft.policyVersion.trim().length <= 80;
}

function NumberField({
  id,
  label,
  value,
  min,
  max,
  disabled,
  onChange,
}: {
  id: string;
  label: string;
  value: number;
  min: number;
  max: number;
  disabled: boolean;
  onChange: (value: number) => void;
}) {
  return (
    <div className="space-y-1.5">
      <Label htmlFor={id} className="text-caption text-muted-foreground">
        {label}
      </Label>
      <Input
        id={id}
        type="number"
        inputMode="numeric"
        min={min}
        max={max}
        value={value}
        disabled={disabled}
        onChange={(event) => {
          const next = event.currentTarget.valueAsNumber;
          if (Number.isFinite(next)) onChange(next);
        }}
        className="font-mono tabular-nums"
      />
    </div>
  );
}

export function LoopConfiguration({
  loop,
  canConfigure,
  forkRequired,
  saving,
  pausing,
  onSave,
  onPause,
}: {
  loop: SkillEvolutionLoop | null;
  canConfigure: boolean;
  forkRequired: boolean;
  saving: boolean;
  pausing: boolean;
  onSave: (input: ConfigureSkillEvolutionInput) => void;
  onPause: () => void;
}) {
  const { t, i18n } = useT("skill-evolution");
  const [draft, setDraft] = useState<LoopDraft>(() => draftFromLoop(loop));

  useEffect(() => {
    setDraft(draftFromLoop(loop));
  }, [loop]);

  const persisted = useMemo(() => draftFromLoop(loop), [loop]);
  const dirty = JSON.stringify(draft) !== JSON.stringify(persisted);
  const valid = validLoopDraft(draft);
  const disabled = !canConfigure || forkRequired || saving || pausing;
  const update = <K extends keyof LoopDraft>(key: K, value: LoopDraft[K]) => {
    setDraft((current) => ({ ...current, [key]: value }));
  };

  return (
    <section aria-labelledby="evolution-loop-title" className="border-b px-4 py-5 sm:px-6">
      <div className="flex flex-wrap items-center gap-2">
        <Settings2 className="size-4 text-muted-foreground" aria-hidden="true" />
        <h2 id="evolution-loop-title" className="text-title-sm font-medium">
          {t(($) => $.loop.title)}
        </h2>
        {dirty ? (
          <span className="ms-auto text-caption text-warning">{t(($) => $.loop.dirty)}</span>
        ) : null}
      </div>

      {!loop ? (
        <div className="mt-4 rounded-md border border-dashed px-3 py-3">
          <div className="text-body font-medium">{t(($) => $.states.no_loop_title)}</div>
          <div className="mt-0.5 text-caption text-muted-foreground">
            {t(($) => $.states.no_loop_description)}
          </div>
        </div>
      ) : null}

      <fieldset disabled={disabled} className="mt-4 space-y-4 disabled:opacity-70">
        <div className="flex min-h-8 items-center justify-between gap-4">
          <Label htmlFor="evolution-enabled" className="text-body font-medium">
            {t(($) => $.loop.enabled)}
          </Label>
          <Switch
            id="evolution-enabled"
            checked={draft.enabled}
            disabled={disabled}
            onCheckedChange={(checked) => update("enabled", checked)}
          />
        </div>

        <div className="space-y-1.5">
          <Label className="text-caption text-muted-foreground">
            {t(($) => $.loop.mode)}
          </Label>
          <SegmentedToggle
            value={draft.mode}
            options={[
              ["observe", t(($) => $.status.observe)],
              ["propose", t(($) => $.status.propose)],
              ["paused", t(($) => $.status.paused)],
            ]}
            onChange={(mode) => update("mode", mode)}
            buttonClassName="min-h-8 px-2 text-caption"
          />
        </div>

        <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-1 2xl:grid-cols-2">
          <NumberField
            id="evolution-cooldown"
            label={t(($) => $.loop.cooldown)}
            value={draft.cooldownSeconds}
            min={60}
            max={2592000}
            disabled={disabled}
            onChange={(value) => update("cooldownSeconds", value)}
          />
          <NumberField
            id="evolution-signals"
            label={t(($) => $.loop.minimum_signals)}
            value={draft.minimumSignals}
            min={1}
            max={100}
            disabled={disabled}
            onChange={(value) => update("minimumSignals", value)}
          />
          <NumberField
            id="evolution-evidence"
            label={t(($) => $.loop.evidence_limit)}
            value={draft.maxEvidenceRefs}
            min={1}
            max={100}
            disabled={disabled}
            onChange={(value) => update("maxEvidenceRefs", value)}
          />
          <NumberField
            id="evolution-replay"
            label={t(($) => $.loop.replay_limit)}
            value={draft.maxReplaySamples}
            min={1}
            max={32}
            disabled={disabled}
            onChange={(value) => update("maxReplaySamples", value)}
          />
          <NumberField
            id="evolution-cost"
            label={t(($) => $.loop.cost_limit)}
            value={draft.maxCostUsdTicks}
            min={0}
            max={1000000000}
            disabled={disabled}
            onChange={(value) => update("maxCostUsdTicks", value)}
          />
          <div className="space-y-1.5">
            <Label htmlFor="evolution-policy" className="text-caption text-muted-foreground">
              {t(($) => $.loop.policy_version)}
            </Label>
            <Input
              id="evolution-policy"
              value={draft.policyVersion}
              disabled={disabled}
              maxLength={80}
              onChange={(event) => update("policyVersion", event.currentTarget.value)}
              className="font-mono"
            />
          </div>
        </div>
      </fieldset>

      {loop ? (
        <dl className="mt-5 grid grid-cols-2 gap-x-4 gap-y-3 border-t pt-4 text-caption">
          {[
            [t(($) => $.loop.last_observed), loop.lastObservedAt],
            [t(($) => $.loop.last_proposal), loop.lastProposalAt],
            [t(($) => $.loop.next_eligible), loop.nextEligibleAt],
            [t(($) => $.loop.updated), loop.updatedAt],
          ].map(([label, value]) => (
            <div key={label} className="min-w-0">
              <dt className="text-muted-foreground">{label}</dt>
              <dd className="mt-0.5 truncate" title={value ?? undefined}>
                {formatDate(value, i18n.language, t(($) => $.page.not_available))}
              </dd>
            </div>
          ))}
        </dl>
      ) : null}

      <div className="mt-5 flex flex-wrap items-center gap-2">
        <Button
          type="button"
          size="sm"
          disabled={disabled || !dirty || !valid}
          onClick={() => onSave({ ...draft, policyVersion: draft.policyVersion.trim() })}
          title={!canConfigure ? t(($) => $.permissions.configure_required) : undefined}
        >
          <Save aria-hidden="true" />
          {saving ? t(($) => $.actions.saving) : t(($) => $.actions.save)}
        </Button>
        <Button
          type="button"
          size="sm"
          variant="outline"
          disabled={disabled || !loop || loop.mode === "paused"}
          onClick={onPause}
          title={!canConfigure ? t(($) => $.permissions.configure_required) : undefined}
        >
          <Pause aria-hidden="true" />
          {pausing ? t(($) => $.actions.pausing) : t(($) => $.actions.pause)}
        </Button>
        {!canConfigure || forkRequired ? (
          <span className="inline-flex min-w-0 items-center gap-1 text-caption text-muted-foreground">
            <Clock3 className="size-3.5 shrink-0" aria-hidden="true" />
            {forkRequired
              ? t(($) => $.permissions.fork_required)
              : t(($) => $.permissions.configure_required)}
          </span>
        ) : null}
      </div>
    </section>
  );
}
