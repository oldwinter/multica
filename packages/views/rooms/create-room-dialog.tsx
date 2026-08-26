"use client";

import { useMemo, useState } from "react";
import {
  Bot,
  Check,
  ChevronRight,
  ClipboardList,
  FlaskConical,
  Loader2,
  RotateCcw,
  Scale,
  ShieldAlert,
  Siren,
  Users,
} from "lucide-react";
import {
  applyRoomTemplateDefaults,
  type CreateRoomInput,
  type RoomDetail,
  type RoomTemplateDraftFields,
  type RoomTemplateId,
  type RoomTemplateTouchedFields,
} from "@multica/core/rooms";
import type { Agent, MemberWithUser, Squad } from "@multica/core/types";
import { Button } from "@multica/ui/components/ui/button";
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from "@multica/ui/components/ui/collapsible";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@multica/ui/components/ui/dialog";
import { Input } from "@multica/ui/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@multica/ui/components/ui/select";
import { Textarea } from "@multica/ui/components/ui/textarea";
import { cn } from "@multica/ui/lib/utils";
import { ActorAvatar } from "../common/actor-avatar";
import { useT } from "../i18n";

type FacilitatorMode = "agent" | "squad";
type CreateRoomTemplate = Exclude<RoomTemplateId, "unknown">;
type RoomsT = ReturnType<typeof useT<"rooms">>["t"];

interface CreateRoomDialogProps {
  readonly open: boolean;
  readonly agents: readonly Agent[];
  readonly squads: readonly Squad[];
  readonly members: readonly MemberWithUser[];
  readonly pending: boolean;
  readonly initialInput?: CreateRoomInput | null;
  readonly mode?: "create" | "duplicate";
  readonly onOpenChange: (open: boolean) => void;
  readonly onCreate: (
    input: CreateRoomInput,
    onSuccess: (detail: RoomDetail) => void,
  ) => void;
}

const SCHEDULE_VALUES = [0, 15, 30, 60, 180, 720, 1440] as const;
const TURN_LIMIT_VALUES = [8, 12, 20] as const;
const TEMPLATE_VALUES = ["research", "planning", "risk", "incident", "decision"] as const;
const TEMPLATE_ICONS = {
  research: FlaskConical,
  planning: ClipboardList,
  risk: ShieldAlert,
  incident: Siren,
  decision: Scale,
} as const;

export function CreateRoomDialog({
  open,
  agents,
  squads,
  members,
  pending,
  initialInput = null,
  mode = "create",
  onOpenChange,
  onCreate,
}: CreateRoomDialogProps) {
  return (
    <Dialog
      open={open}
      onOpenChange={(next) => {
        if (!pending || next) onOpenChange(next);
      }}
    >
      <DialogContent className="max-h-[min(48rem,calc(100dvh-2rem))] overflow-y-auto sm:max-w-2xl">
        {open ? (
          <CreateRoomForm
            agents={agents}
            squads={squads}
            members={members}
            pending={pending}
            initialInput={initialInput}
            mode={mode}
            onCancel={() => onOpenChange(false)}
            onCreate={onCreate}
            onCreated={() => onOpenChange(false)}
          />
        ) : null}
      </DialogContent>
    </Dialog>
  );
}

function CreateRoomForm({
  agents,
  squads,
  members,
  pending,
  initialInput,
  mode: formMode,
  onCancel,
  onCreate,
  onCreated,
}: Omit<CreateRoomDialogProps, "open" | "onOpenChange"> & {
  readonly initialInput: CreateRoomInput | null;
  readonly mode: "create" | "duplicate";
  readonly onCancel: () => void;
  readonly onCreated: () => void;
}) {
  const { t } = useT("rooms");
  const activeAgents = useMemo(
    () => agents.filter((agent) => !agent.archived_at),
    [agents],
  );
  const activeSquads = useMemo(
    () => squads.filter((squad) => !squad.archived_at),
    [squads],
  );
  const initialTemplate = knownTemplate(initialInput?.template_id) ?? "research";
  const defaults = templateDefaults(t, initialTemplate);
  const initialFacilitatorMode: FacilitatorMode = initialInput?.facilitator_squad_id
    ? "squad"
    : "agent";
  const [facilitatorMode, setFacilitatorMode] = useState<FacilitatorMode>(
    initialFacilitatorMode,
  );
  const [title, setTitle] = useState(() => {
    if (!initialInput) return "";
    const suffix = formMode === "duplicate" ? t(($) => $.create.copy_name_suffix) : "";
    return `${initialInput.title.slice(0, 160 - suffix.length)}${suffix}`;
  });
  const [templateId, setTemplateId] = useState<CreateRoomTemplate>(initialTemplate);
  const [fields, setFields] = useState<RoomTemplateDraftFields>(() => ({
    objective: initialInput?.objective ?? defaults.objective,
    successCriteria:
      initialInput?.success_criteria?.join("\n") ?? defaults.successCriteria,
    stopConditions:
      initialInput?.stop_conditions?.join("\n") ?? defaults.stopConditions,
    instructions: initialInput?.instructions ?? defaults.instructions,
    dailyTurnLimit:
      initialInput?.daily_turn_limit?.toString() ?? defaults.dailyTurnLimit,
    maxCostTicks:
      initialInput?.max_cost_ticks?.toString() ?? defaults.maxCostTicks,
    scheduleMinutes:
      initialInput?.schedule_interval_minutes?.toString() ?? defaults.scheduleMinutes,
  }));
  const [touched, setTouched] = useState<RoomTemplateTouchedFields>(() =>
    initialInput
      ? {
          objective: true,
          successCriteria: true,
          stopConditions: true,
          instructions: true,
          dailyTurnLimit: true,
          maxCostTicks: true,
          scheduleMinutes: true,
        }
      : {},
  );
  const [facilitatorId, setFacilitatorId] = useState(
    initialInput?.facilitator_squad_id ?? initialInput?.facilitator_agent_id ?? "",
  );
  const [participantAgentIds, setParticipantAgentIds] = useState<Set<string>>(
    () =>
      new Set(
        initialInput?.participants
          ?.filter((participant) => participant.type === "agent")
          .map((participant) => participant.id) ?? [],
      ),
  );
  const [participantMemberIds, setParticipantMemberIds] = useState<Set<string>>(
    () =>
      new Set(
        initialInput?.participants
          ?.filter((participant) => participant.type === "member")
          .map((participant) => participant.id) ?? [],
      ),
  );
  const participantRoles = useMemo(
    () =>
      new Map(
        initialInput?.participants?.map((participant) => [
          `${participant.type}:${participant.id}`,
          participant.role,
        ]) ?? [],
      ),
    [initialInput],
  );

  const setField = <K extends keyof RoomTemplateDraftFields>(
    field: K,
    value: RoomTemplateDraftFields[K],
  ) => {
    setFields((current) => ({ ...current, [field]: value }));
    setTouched((current) => ({ ...current, [field]: true }));
  };

  const applyTemplate = (value: CreateRoomTemplate) => {
    setTemplateId(value);
    setFields((current) =>
      applyRoomTemplateDefaults(current, templateDefaults(t, value), touched),
    );
  };

  const resetTemplateDefaults = () => {
    setFields(templateDefaults(t, templateId));
    setTouched({});
  };

  const toggle = (
    selected: Set<string>,
    id: string,
    update: (next: Set<string>) => void,
  ) => {
    const next = new Set(selected);
    if (next.has(id)) next.delete(id);
    else next.add(id);
    update(next);
  };

  const submit = () => {
    const participants = [
      ...Array.from(participantAgentIds, (id) => ({
        type: "agent" as const,
        id,
        role: participantRoles.get(`agent:${id}`) ?? "participant",
      })),
      ...Array.from(participantMemberIds, (id) => ({
        type: "member" as const,
        id,
        role: participantRoles.get(`member:${id}`) ?? "participant",
      })),
    ];
    const scheduled = fields.scheduleMinutes !== "0";
    const shared = {
      title: title.trim(),
      template_id: templateId,
      objective: fields.objective.trim(),
      success_criteria: lines(fields.successCriteria),
      stop_conditions: lines(fields.stopConditions),
      instructions: fields.instructions.trim() || undefined,
      participants,
      daily_turn_limit: fields.dailyTurnLimit
        ? Number(fields.dailyTurnLimit)
        : undefined,
      max_cost_ticks: fields.maxCostTicks
        ? Number(fields.maxCostTicks)
        : undefined,
      schedule_interval_minutes: scheduled
        ? Number(fields.scheduleMinutes)
        : undefined,
      start_paused: formMode === "duplicate" && scheduled,
    };
    const input: CreateRoomInput =
      facilitatorMode === "agent"
        ? { ...shared, facilitator_agent_id: facilitatorId }
        : { ...shared, facilitator_squad_id: facilitatorId };
    onCreate(input, onCreated);
  };

  const facilitatorOptions =
    facilitatorMode === "agent" ? activeAgents : activeSquads;
  const targetAgentIds = new Set(participantAgentIds);
  if (facilitatorMode === "agent" && facilitatorId) targetAgentIds.add(facilitatorId);
  const synthesisRequired = facilitatorMode === "squad" || targetAgentIds.size > 1;
  const facilitator = facilitatorOptions.find((option) => option.id === facilitatorId);
  const participantCount = participantAgentIds.size + participantMemberIds.size;
  const scheduled = fields.scheduleMinutes !== "0";
  const canSubmit =
    title.trim().length > 0 &&
    fields.objective.trim().length > 0 &&
    facilitatorId.length > 0 &&
    !pending;

  return (
    <>
      <DialogHeader>
        <DialogTitle>
          {formMode === "duplicate"
            ? t(($) => $.create.duplicate_title)
            : t(($) => $.create.title)}
        </DialogTitle>
        <DialogDescription>
          {formMode === "duplicate"
            ? t(($) => $.create.duplicate_description)
            : t(($) => $.create.description)}
        </DialogDescription>
      </DialogHeader>

      <div className="space-y-5">
        <section aria-labelledby="room-create-template-label">
          <div className="mb-2 flex items-center justify-between gap-2">
            <h2
              id="room-create-template-label"
              className="text-caption font-medium text-foreground"
            >
              {t(($) => $.create.choose_template)}
            </h2>
            <Button type="button" size="xs" variant="ghost" onClick={resetTemplateDefaults}>
              <RotateCcw aria-hidden="true" />
              {t(($) => $.create.reset_defaults)}
            </Button>
          </div>
          <div className="grid gap-2 sm:grid-cols-2">
            {TEMPLATE_VALUES.map((value) => {
              const Icon = TEMPLATE_ICONS[value];
              const selected = templateId === value;
              return (
                <button
                  key={value}
                  type="button"
                  autoFocus={value === templateId}
                  aria-pressed={selected}
                  data-testid={`room-template-${value}`}
                  className={cn(
                    "min-w-0 rounded-md border px-3 py-2.5 text-left outline-none transition-colors focus-visible:ring-2 focus-visible:ring-ring",
                    selected
                      ? "border-brand bg-surface-selected"
                      : "border-surface-border hover:bg-surface-hover",
                  )}
                  onClick={() => applyTemplate(value)}
                >
                  <span className="flex items-center gap-2 text-body font-medium text-foreground">
                    <Icon className="size-4 shrink-0" aria-hidden="true" />
                    {t(($) => $.create.templates[value].label)}
                  </span>
                  <span className="mt-1 block text-caption leading-5 text-muted-foreground">
                    {t(($) => $.create.templates[value].outcome)}
                  </span>
                  <span className="mt-1 block truncate text-caption text-muted-foreground">
                    {firstLine(t(($) => $.create.templates[value].success_criteria))}
                  </span>
                  <span className="mt-1 block truncate text-caption text-foreground">
                    {t(($) => $.create.templates[value].example)}
                  </span>
                </button>
              );
            })}
          </div>
        </section>

        <div className="grid gap-3 sm:grid-cols-[minmax(10rem,0.65fr)_minmax(0,1.35fr)]">
          <Field label={t(($) => $.create.fields.title)}>
            <Input
              value={title}
              maxLength={160}
              aria-label={t(($) => $.create.fields.title)}
              placeholder={t(($) => $.create.placeholders.title)}
              onChange={(event) => setTitle(event.target.value)}
            />
          </Field>
          <Field label={t(($) => $.create.fields.objective)}>
            <Textarea
              className="min-h-20 resize-y"
              value={fields.objective}
              maxLength={1200}
              aria-label={t(($) => $.create.fields.objective)}
              placeholder={t(($) => $.create.placeholders.objective)}
              onChange={(event) => setField("objective", event.target.value)}
            />
          </Field>
        </div>

        <div className="grid gap-4 sm:grid-cols-2">
          <Field label={t(($) => $.create.fields.facilitator)}>
            <div className="grid grid-cols-2 rounded-md bg-muted p-1">
              {(["agent", "squad"] as const).map((value) => {
                const Icon = value === "agent" ? Bot : Users;
                return (
                  <button
                    key={value}
                    type="button"
                    aria-pressed={facilitatorMode === value}
                    className={cn(
                      "flex h-7 items-center justify-center gap-1.5 rounded-md text-caption font-medium transition-colors",
                      facilitatorMode === value
                        ? "bg-surface text-foreground shadow-[var(--surface-shadow)]"
                        : "text-muted-foreground hover:text-foreground",
                    )}
                    onClick={() => {
                      setFacilitatorMode(value);
                      setFacilitatorId("");
                    }}
                  >
                    <Icon className="size-3.5" aria-hidden="true" />
                    {value === "agent"
                      ? t(($) => $.create.facilitator_modes.agent)
                      : t(($) => $.create.facilitator_modes.squad)}
                  </button>
                );
              })}
            </div>
            <Select
              items={facilitatorOptions.map((option) => ({
                value: option.id,
                label: option.name,
              }))}
              value={facilitatorId || null}
              onValueChange={(value) => setFacilitatorId(value ?? "")}
            >
              <SelectTrigger className="mt-2 w-full" aria-label={t(($) => $.create.fields.facilitator)}>
                <SelectValue
                  placeholder={
                    facilitatorMode === "agent"
                      ? t(($) => $.create.placeholders.select_agent)
                      : t(($) => $.create.placeholders.select_squad)
                  }
                />
              </SelectTrigger>
              <SelectContent>
                {facilitatorOptions.map((option) => (
                  <SelectItem key={option.id} value={option.id}>
                    {option.name}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </Field>

          <div
            className="space-y-1.5 text-caption font-medium text-muted-foreground"
            role="group"
            aria-labelledby="room-create-participants-label"
          >
            <span id="room-create-participants-label">
              {t(($) => $.create.fields.participants)}
            </span>
            <div className="max-h-36 overflow-y-auto rounded-md border border-surface-border p-1">
              {activeAgents.map((agent) => (
                <ParticipantOption
                  key={agent.id}
                  type="agent"
                  id={agent.id}
                  name={agent.name}
                  checked={participantAgentIds.has(agent.id)}
                  onToggle={() =>
                    toggle(participantAgentIds, agent.id, setParticipantAgentIds)
                  }
                />
              ))}
              {members.map((member) => (
                <ParticipantOption
                  key={member.user_id}
                  type="member"
                  id={member.user_id}
                  name={member.name || member.email}
                  checked={participantMemberIds.has(member.user_id)}
                  onToggle={() =>
                    toggle(participantMemberIds, member.user_id, setParticipantMemberIds)
                  }
                />
              ))}
            </div>
          </div>
        </div>

        <section aria-labelledby="room-create-execution-label">
          <h2 id="room-create-execution-label" className="mb-2 text-caption font-medium text-foreground">
            {t(($) => $.create.bounded_execution)}
          </h2>
          <div className="grid gap-3 sm:grid-cols-2">
            <Field label={t(($) => $.create.fields.daily_turn_limit)}>
              <Select
                items={turnLimitItems(fields.dailyTurnLimit, t)}
                value={fields.dailyTurnLimit || "unlimited"}
                onValueChange={(value) =>
                  setField("dailyTurnLimit", value === "unlimited" ? "" : (value ?? ""))
                }
              >
                <SelectTrigger className="w-full" aria-label={t(($) => $.create.fields.daily_turn_limit)}>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {turnLimitItems(fields.dailyTurnLimit, t).map((item) => (
                    <SelectItem key={item.value} value={item.value}>
                      {item.label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </Field>
            <Field label={t(($) => $.create.fields.schedule_interval)}>
              <Select
                items={SCHEDULE_VALUES.map((minutes) => ({
                  value: String(minutes),
                  label: scheduleLabel(t, minutes),
                }))}
                value={fields.scheduleMinutes}
                onValueChange={(value) => setField("scheduleMinutes", value ?? "0")}
              >
                <SelectTrigger className="w-full" aria-label={t(($) => $.create.fields.schedule_interval)}>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {SCHEDULE_VALUES.map((minutes) => (
                    <SelectItem key={minutes} value={String(minutes)}>
                      {scheduleLabel(t, minutes)}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </Field>
          </div>
        </section>

        <Collapsible>
          <CollapsibleTrigger className="group flex items-center gap-1 rounded-md px-1 py-1 text-caption font-medium text-foreground outline-none hover:bg-surface-hover focus-visible:ring-2 focus-visible:ring-ring">
            <ChevronRight className="size-3.5 transition-transform group-data-[panel-open]:rotate-90" />
            {t(($) => $.create.advanced)}
          </CollapsibleTrigger>
          <CollapsibleContent className="h-(--collapsible-panel-height) overflow-hidden transition-[height] duration-200 ease-out data-starting-style:h-0 data-ending-style:h-0">
            <div className="grid gap-3 pt-3 sm:grid-cols-2">
              <Field label={t(($) => $.create.fields.success_criteria)}>
                <Textarea
                  className="min-h-24 resize-y"
                  value={fields.successCriteria}
                  aria-label={t(($) => $.create.fields.success_criteria)}
                  placeholder={t(($) => $.create.placeholders.one_per_line)}
                  onChange={(event) => setField("successCriteria", event.target.value)}
                />
              </Field>
              <Field label={t(($) => $.create.fields.stop_conditions)}>
                <Textarea
                  className="min-h-24 resize-y"
                  value={fields.stopConditions}
                  aria-label={t(($) => $.create.fields.stop_conditions)}
                  placeholder={t(($) => $.create.placeholders.one_per_line)}
                  onChange={(event) => setField("stopConditions", event.target.value)}
                />
              </Field>
              <Field label={t(($) => $.create.fields.instructions)}>
                <Textarea
                  className="min-h-20 resize-y"
                  value={fields.instructions}
                  aria-label={t(($) => $.create.fields.instructions)}
                  placeholder={t(($) => $.create.placeholders.instructions)}
                  onChange={(event) => setField("instructions", event.target.value)}
                />
              </Field>
              <Field label={t(($) => $.create.fields.max_cost_ticks)}>
                <Input
                  type="number"
                  min={1}
                  inputMode="numeric"
                  value={fields.maxCostTicks}
                  aria-label={t(($) => $.create.fields.max_cost_ticks)}
                  placeholder={t(($) => $.detail.unlimited)}
                  onChange={(event) => setField("maxCostTicks", event.target.value)}
                />
              </Field>
            </div>
          </CollapsibleContent>
        </Collapsible>

        <section
          className="border-y border-surface-border py-3"
          aria-labelledby="room-create-summary-label"
          data-testid="room-create-summary"
        >
          <h2 id="room-create-summary-label" className="mb-2 text-caption font-medium text-foreground">
            {t(($) => $.create.summary)}
          </h2>
          <dl className="grid gap-x-4 gap-y-1 text-caption sm:grid-cols-2">
            <SummaryValue
              label={t(($) => $.create.summary_runner)}
              value={
                facilitator
                  ? t(($) => $.create.summary_people, {
                      facilitator: facilitator.name,
                      count: participantCount,
                    })
                  : t(($) => $.create.summary_not_selected)
              }
            />
            <SummaryValue
              label={t(($) => $.create.summary_synthesis)}
              value={t(($) =>
                synthesisRequired ? $.create.summary_required : $.create.summary_direct,
              )}
            />
            <SummaryValue
              label={t(($) => $.create.summary_turns)}
              value={fields.dailyTurnLimit || t(($) => $.detail.unlimited)}
            />
            <SummaryValue
              label={t(($) => $.create.summary_cost)}
              value={fields.maxCostTicks || t(($) => $.detail.unlimited)}
            />
            <SummaryValue
              label={t(($) => $.create.summary_execution)}
              value={scheduled
                ? scheduleLabel(t, Number(fields.scheduleMinutes))
                : t(($) => $.create.manual)}
            />
          </dl>
          {scheduled && formMode === "duplicate" ? (
            <p className="mt-2 text-caption font-medium text-warning">
              {t(($) => $.create.scheduled_copy_paused)}
            </p>
          ) : null}
        </section>
      </div>

      <DialogFooter>
        <Button type="button" variant="outline" disabled={pending} onClick={onCancel}>
          {t(($) => $.actions.cancel)}
        </Button>
        <Button type="button" disabled={!canSubmit} data-testid="room-create-submit" onClick={submit}>
          {pending ? <Loader2 className="animate-spin" aria-hidden="true" /> : null}
          {formMode === "duplicate"
            ? t(($) => $.actions.duplicate)
            : t(($) => $.actions.create)}
        </Button>
      </DialogFooter>
    </>
  );
}

function templateDefaults(t: RoomsT, template: CreateRoomTemplate): RoomTemplateDraftFields {
  const dailyTurnLimit = {
    research: "12",
    planning: "8",
    risk: "12",
    incident: "20",
    decision: "8",
  }[template];
  return {
    objective: t(($) => $.create.templates[template].objective),
    successCriteria: t(($) => $.create.templates[template].success_criteria),
    stopConditions: t(($) => $.create.templates[template].stop_conditions),
    instructions: "",
    dailyTurnLimit,
    maxCostTicks: "",
    scheduleMinutes: "0",
  };
}

function knownTemplate(value: RoomTemplateId | undefined): CreateRoomTemplate | null {
  return value && value !== "unknown" ? value : null;
}

function lines(value: string): string[] {
  return value.split("\n").map((line) => line.trim()).filter(Boolean);
}

function firstLine(value: string): string {
  return value.split("\n", 1)[0] ?? value;
}

function scheduleLabel(t: RoomsT, minutes: number): string {
  return minutes === 0
    ? t(($) => $.create.manual)
    : t(($) => $.detail.minutes, { count: minutes });
}

function turnLimitItems(value: string, t: RoomsT) {
  const standard = TURN_LIMIT_VALUES.map((limit) => ({
    value: String(limit),
    label: t(($) => $.create.turn_limit, { count: limit }),
  }));
  if (value && !TURN_LIMIT_VALUES.includes(Number(value) as (typeof TURN_LIMIT_VALUES)[number])) {
    standard.push({
      value,
      label: t(($) => $.create.turn_limit, { count: Number(value) }),
    });
  }
  return [
    ...standard,
    { value: "unlimited", label: t(($) => $.detail.unlimited) },
  ];
}

function Field({ label, children }: { readonly label: string; readonly children: React.ReactNode }) {
  return (
    <div className="space-y-1.5 text-caption font-medium text-muted-foreground">
      <span>{label}</span>
      {children}
    </div>
  );
}

function SummaryValue({ label, value }: { readonly label: string; readonly value: string }) {
  return (
    <div className="flex min-w-0 justify-between gap-3 sm:block">
      <dt className="text-muted-foreground">{label}</dt>
      <dd className="min-w-0 truncate font-medium text-foreground">{value}</dd>
    </div>
  );
}

function ParticipantOption({
  type,
  id,
  name,
  checked,
  onToggle,
}: {
  readonly type: "agent" | "member";
  readonly id: string;
  readonly name: string;
  readonly checked: boolean;
  readonly onToggle: () => void;
}) {
  return (
    <button
      type="button"
      role="checkbox"
      aria-checked={checked}
      aria-label={name}
      className="flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-left text-body text-foreground outline-none hover:bg-surface-hover focus-visible:ring-2 focus-visible:ring-ring"
      onClick={onToggle}
    >
      <ActorAvatar actorType={type} actorId={id} size="sm" profileLink={false} />
      <span className="min-w-0 flex-1 truncate">{name}</span>
      <span
        className={cn(
          "flex size-4 items-center justify-center rounded border",
          checked ? "border-brand bg-brand text-brand-foreground" : "border-input",
        )}
      >
        {checked ? <Check className="size-3" aria-hidden="true" /> : null}
      </span>
    </button>
  );
}
