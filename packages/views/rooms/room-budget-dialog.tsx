"use client";

import { useEffect, useState } from "react";
import type { Room, UpdateRoomBudgetInput } from "@multica/core/rooms";
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
import { useT } from "../i18n";

interface RoomBudgetDialogProps {
  readonly open: boolean;
  readonly room: Room;
  readonly pending: boolean;
  readonly onOpenChange: (open: boolean) => void;
  readonly onSave: (input: UpdateRoomBudgetInput) => void;
}

export function RoomBudgetDialog({
  open,
  room,
  pending,
  onOpenChange,
  onSave,
}: RoomBudgetDialogProps) {
  const { t } = useT("rooms");
  const [dailyLimit, setDailyLimit] = useState("");
  const [costLimit, setCostLimit] = useState("");

  useEffect(() => {
    if (!open) return;
    setDailyLimit(room.daily_turn_limit?.toString() ?? "");
    setCostLimit(room.max_cost_ticks?.toString() ?? "");
  }, [open, room.daily_turn_limit, room.id, room.max_cost_ticks]);

  const dailyValue = positiveIntegerOrNull(dailyLimit);
  const costValue = positiveIntegerOrNull(costLimit);
  const invalid = dailyValue === undefined || costValue === undefined;

  return (
    <Dialog open={open} onOpenChange={(next) => !pending && onOpenChange(next)}>
      <DialogContent className="max-h-[calc(100dvh-2rem)] overflow-y-auto sm:max-w-md max-lg:[&_[data-slot=dialog-close]]:size-11">
        <DialogHeader>
          <DialogTitle>{t(($) => $.budget.title)}</DialogTitle>
          <DialogDescription>{t(($) => $.budget.description)}</DialogDescription>
        </DialogHeader>
        <div className="space-y-4">
          <label className="block space-y-1.5 text-body text-foreground">
            <span>{t(($) => $.budget.daily_turn_limit)}</span>
            <Input
              autoFocus
              type="number"
              className="max-lg:h-11"
              min={1}
              max={10000}
              inputMode="numeric"
              value={dailyLimit}
              placeholder={t(($) => $.detail.unlimited)}
              onChange={(event) => setDailyLimit(event.target.value)}
            />
            <span className="block text-caption text-muted-foreground">
              {t(($) => $.budget.daily_hint)}
            </span>
          </label>
          <label className="block space-y-1.5 text-body text-foreground">
            <span>{t(($) => $.budget.max_cost_ticks)}</span>
            <Input
              type="number"
              className="max-lg:h-11"
              min={1}
              inputMode="numeric"
              value={costLimit}
              placeholder={t(($) => $.detail.unlimited)}
              onChange={(event) => setCostLimit(event.target.value)}
            />
            <span className="block text-caption text-muted-foreground">
              {t(($) => $.budget.cost_hint)}
            </span>
          </label>
          {invalid ? (
            <p className="text-caption text-destructive" role="alert">
              {t(($) => $.budget.invalid)}
            </p>
          ) : null}
        </div>
        <DialogFooter>
          <Button
            type="button"
            variant="outline"
            className="max-lg:h-11"
            disabled={pending}
            onClick={() => onOpenChange(false)}
          >
            {t(($) => $.actions.cancel)}
          </Button>
          <Button
            type="button"
            className="max-lg:h-11"
            disabled={pending || invalid}
            onClick={() => {
              if (dailyValue === undefined || costValue === undefined) return;
              onSave({ daily_turn_limit: dailyValue, max_cost_ticks: costValue });
            }}
          >
            {pending ? t(($) => $.budget.saving) : t(($) => $.budget.save)}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

function positiveIntegerOrNull(value: string): number | null | undefined {
  if (!value.trim()) return null;
  const parsed = Number(value);
  return Number.isSafeInteger(parsed) && parsed > 0 ? parsed : undefined;
}
