"use client";

import { useMemo, useState } from "react";
import { Bot, Check, Loader2, Users } from "lucide-react";
import type { Agent, MemberWithUser, Squad } from "@multica/core/types";
import type { CreateRoomInput, RoomDetail } from "@multica/core/rooms";
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
  const [instructions, setInstructions] = useState("");
  const [facilitatorId, setFacilitatorId] = useState("");
  const [participantAgentIds, setParticipantAgentIds] = useState<Set<string>>(
    new Set(),
  );
  const [participantMemberIds, setParticipantMemberIds] = useState<Set<string>>(
    new Set(),
  );
  const [dailyLimit, setDailyLimit] = useState("");
  const [scheduleMinutes, setScheduleMinutes] = useState("0");

  const reset = () => {
    setMode("agent");
    setTitle("");
    setInstructions("");
    setFacilitatorId("");
    setParticipantAgentIds(new Set());
    setParticipantMemberIds(new Set());
    setDailyLimit("");
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
      instructions: instructions.trim() || undefined,
      participants,
      daily_turn_limit: dailyLimit ? Number(dailyLimit) : undefined,
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
    title.trim().length > 0 && facilitatorId.length > 0 && !pending;

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

          <Field label={t(($) => $.create.fields.participants)}>
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
          </Field>

          <div className="grid gap-3 sm:grid-cols-2">
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
