"use client";

import { useMemo, useState } from "react";
import { Bot, Check, Loader2, Users } from "lucide-react";
import type { Agent, MemberWithUser, Squad } from "@multica/core/types";
import type { CreateRoomInput, RoomDetail, RoomTemplateId } from "@multica/core/rooms";
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
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@multica/ui/components/ui/select";
import { Textarea } from "@multica/ui/components/ui/textarea";
import { cn } from "@multica/ui/lib/utils";
import { useT } from "../i18n";
import { ActorAvatar } from "../common/actor-avatar";

type FacilitatorMode = "agent" | "squad";

interface CreateRoomDialogProps {
  readonly open: boolean;
  readonly agents: readonly Agent[];
  readonly squads: readonly Squad[];
  readonly members: readonly MemberWithUser[];
  readonly pending: boolean;
  readonly onOpenChange: (open: boolean) => void;
  readonly onCreate: (
    input: CreateRoomInput,
    onSuccess: (detail: RoomDetail) => void,
  ) => void;
}

const SCHEDULE_VALUES = [0, 15, 30, 60, 180, 720, 1440] as const;
const TEMPLATE_VALUES = ["research", "planning", "risk", "incident", "decision"] as const;
type CreateRoomTemplate = Exclude<RoomTemplateId, "unknown">;

export function CreateRoomDialog({
  open,
  agents,
  squads,
  members,
  pending,
  onOpenChange,
  onCreate,
}: CreateRoomDialogProps) {
  const { t } = useT("rooms");
  const activeAgents = useMemo(
    () => agents.filter((agent) => !agent.archived_at),
    [agents],
  );
  const activeSquads = useMemo(
    () => squads.filter((squad) => !squad.archived_at),
    [squads],
  );
  const [mode, setMode] = useState<FacilitatorMode>("agent");
  const [title, setTitle] = useState("");
  const [templateId, setTemplateId] = useState<CreateRoomTemplate>("research");
  const [objective, setObjective] = useState(() => t(($) => $.create.templates.research.objective));
  const [successCriteria, setSuccessCriteria] = useState(() => t(($) => $.create.templates.research.success_criteria));
  const [stopConditions, setStopConditions] = useState(() => t(($) => $.create.templates.research.stop_conditions));
  const [instructions, setInstructions] = useState("");
  const [facilitatorId, setFacilitatorId] = useState("");
  const [participantAgentIds, setParticipantAgentIds] = useState<Set<string>>(
    new Set(),
  );
  const [participantMemberIds, setParticipantMemberIds] = useState<Set<string>>(
    new Set(),
  );
  const [dailyLimit, setDailyLimit] = useState("");
  const [maxCostTicks, setMaxCostTicks] = useState("");
  const [scheduleMinutes, setScheduleMinutes] = useState("0");

  const reset = () => {
    setMode("agent");
    setTitle("");
    setTemplateId("research");
    setObjective(t(($) => $.create.templates.research.objective));
    setSuccessCriteria(t(($) => $.create.templates.research.success_criteria));
    setStopConditions(t(($) => $.create.templates.research.stop_conditions));
    setInstructions("");
    setFacilitatorId("");
    setParticipantAgentIds(new Set());
    setParticipantMemberIds(new Set());
    setDailyLimit("");
    setMaxCostTicks("");
    setScheduleMinutes("0");
  };

  const cancel = () => {
    reset();
    onOpenChange(false);
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
      ...Array.from(participantAgentIds, (id) => ({ type: "agent", id }) as const),
      ...Array.from(participantMemberIds, (id) => ({ type: "member", id }) as const),
    ];
    const shared = {
      title: title.trim(),
      template_id: templateId,
      objective: objective.trim(),
      success_criteria: lines(successCriteria),
      stop_conditions: lines(stopConditions),
      instructions: instructions.trim() || undefined,
      participants,
      daily_turn_limit: dailyLimit ? Number(dailyLimit) : undefined,
      max_cost_ticks: maxCostTicks ? Number(maxCostTicks) : undefined,
      schedule_interval_minutes:
        scheduleMinutes === "0" ? undefined : Number(scheduleMinutes),
    };
    const input: CreateRoomInput =
      mode === "agent"
        ? { ...shared, facilitator_agent_id: facilitatorId }
        : { ...shared, facilitator_squad_id: facilitatorId };
    onCreate(input, () => {
      reset();
      onOpenChange(false);
    });
  };

  const facilitatorOptions = mode === "agent" ? activeAgents : activeSquads;
  const canSubmit =
    title.trim().length > 0 && objective.trim().length > 0 && facilitatorId.length > 0 && !pending;

  const applyTemplate = (value: CreateRoomTemplate) => {
    setTemplateId(value);
    setObjective(t(($) => $.create.templates[value].objective));
    setSuccessCriteria(t(($) => $.create.templates[value].success_criteria));
    setStopConditions(t(($) => $.create.templates[value].stop_conditions));
  };

  return (
    <Dialog
      open={open}
      onOpenChange={(next) => {
        if (!next && !pending) reset();
        onOpenChange(next);
      }}
    >
      <DialogContent className="max-h-[min(42rem,calc(100dvh-2rem))] overflow-y-auto sm:max-w-xl">
        <DialogHeader>
          <DialogTitle>{t(($) => $.create.title)}</DialogTitle>
          <DialogDescription className="sr-only">
            {t(($) => $.create.description)}
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-4">
          <Field label={t(($) => $.create.fields.title)}>
            <Input
              autoFocus
              value={title}
              maxLength={160}
              placeholder={t(($) => $.create.placeholders.title)}
              onChange={(event) => setTitle(event.target.value)}
            />
          </Field>
          <Field label={t(($) => $.create.fields.template)}>
            <Select
              items={TEMPLATE_VALUES.map((value) => ({ value, label: t(($) => $.create.templates[value].label) }))}
              value={templateId}
              onValueChange={(value) => {
                if (value && TEMPLATE_VALUES.includes(value as CreateRoomTemplate)) applyTemplate(value as CreateRoomTemplate);
              }}
            >
              <SelectTrigger className="w-full"><SelectValue /></SelectTrigger>
              <SelectContent>
                {TEMPLATE_VALUES.map((value) => <SelectItem key={value} value={value}>{t(($) => $.create.templates[value].label)}</SelectItem>)}
              </SelectContent>
            </Select>
          </Field>
          <Field label={t(($) => $.create.fields.objective)}>
            <Textarea
              className="min-h-20 resize-y"
              value={objective}
              maxLength={1200}
              placeholder={t(($) => $.create.placeholders.objective)}
              onChange={(event) => setObjective(event.target.value)}
            />
          </Field>
          <div className="grid gap-3 sm:grid-cols-2">
            <Field label={t(($) => $.create.fields.success_criteria)}>
              <Textarea
                className="min-h-24 resize-y"
                value={successCriteria}
                placeholder={t(($) => $.create.placeholders.one_per_line)}
                onChange={(event) => setSuccessCriteria(event.target.value)}
              />
            </Field>
            <Field label={t(($) => $.create.fields.stop_conditions)}>
              <Textarea
                className="min-h-24 resize-y"
                value={stopConditions}
                placeholder={t(($) => $.create.placeholders.one_per_line)}
                onChange={(event) => setStopConditions(event.target.value)}
              />
            </Field>
          </div>
          <Field label={t(($) => $.create.fields.instructions)}>
            <Textarea
              className="min-h-20 resize-y"
              value={instructions}
              placeholder={t(($) => $.create.placeholders.instructions)}
              onChange={(event) => setInstructions(event.target.value)}
            />
          </Field>

          <Field label={t(($) => $.create.fields.facilitator)}>
            <div className="grid grid-cols-2 rounded-lg bg-muted p-1">
              {(["agent", "squad"] as const).map((value) => {
                const Icon = value === "agent" ? Bot : Users;
                return (
                  <button
                    key={value}
                    type="button"
                    className={cn(
                      "flex h-7 items-center justify-center gap-1.5 rounded-md text-caption font-medium transition-colors",
                      mode === value
                        ? "bg-surface text-foreground shadow-[var(--surface-shadow)]"
                        : "text-muted-foreground hover:text-foreground",
                    )}
                    onClick={() => {
                      setMode(value);
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
              <SelectTrigger className="mt-2 w-full">
                <SelectValue
                  placeholder={
                    mode === "agent"
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
            <div className="max-h-44 overflow-y-auto rounded-lg border border-surface-border p-1">
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

          <div className="grid gap-3 sm:grid-cols-3">
            <Field label={t(($) => $.create.fields.daily_turn_limit)}>
              <Input
                type="number"
                min={1}
                max={10000}
                inputMode="numeric"
                value={dailyLimit}
                placeholder={t(($) => $.detail.unlimited)}
                onChange={(event) => setDailyLimit(event.target.value)}
              />
            </Field>
            <Field label={t(($) => $.create.fields.max_cost_ticks)}>
              <Input
                type="number"
                min={1}
                inputMode="numeric"
                value={maxCostTicks}
                placeholder={t(($) => $.detail.unlimited)}
                onChange={(event) => setMaxCostTicks(event.target.value)}
              />
            </Field>
            <Field label={t(($) => $.create.fields.schedule_interval)}>
              <Select
                items={SCHEDULE_VALUES.map((minutes) => ({
                  value: String(minutes),
                  label:
                    minutes === 0
                      ? t(($) => $.detail.disabled)
                      : t(($) => $.detail.minutes, { count: minutes }),
                }))}
                value={scheduleMinutes}
                onValueChange={(value) => setScheduleMinutes(value ?? "0")}
              >
                <SelectTrigger className="w-full">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {SCHEDULE_VALUES.map((minutes) => (
                    <SelectItem key={minutes} value={String(minutes)}>
                      {minutes === 0
                        ? t(($) => $.detail.disabled)
                        : t(($) => $.detail.minutes, { count: minutes })}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </Field>
          </div>
        </div>

        <DialogFooter>
          <Button type="button" variant="outline" disabled={pending} onClick={cancel}>
            {t(($) => $.actions.cancel)}
          </Button>
          <Button type="button" disabled={!canSubmit} data-testid="room-create-submit" onClick={submit}>
            {pending ? <Loader2 className="animate-spin" aria-hidden="true" /> : null}
            {t(($) => $.actions.create)}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

function lines(value: string): string[] {
  return value.split("\n").map((line) => line.trim()).filter(Boolean);
}

function Field({ label, children }: { readonly label: string; readonly children: React.ReactNode }) {
  return (
    <label className="block space-y-1.5 text-caption font-medium text-muted-foreground">
      <span>{label}</span>
      {children}
    </label>
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
