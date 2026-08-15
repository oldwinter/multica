"use client";

import { useEffect, useState } from "react";
import { Loader2 } from "lucide-react";
import type {
  PromoteRoomArtifactInput,
  RoomArtifact,
  RoomPromotionKind,
} from "@multica/core/rooms";
import { createSafeId } from "@multica/core/utils";
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
import { useT } from "../i18n";

export type PromotionSource =
  | { readonly entryId: string; readonly cycleId?: never; readonly suggestedTitle: string }
  | { readonly entryId?: never; readonly cycleId: string; readonly suggestedTitle: string };

interface PromoteRoomDialogProps {
  readonly source: PromotionSource | null;
  readonly pending: boolean;
  readonly onOpenChange: (open: boolean) => void;
  readonly onPromote: (
    input: PromoteRoomArtifactInput,
    onSuccess: (artifact: RoomArtifact) => void,
  ) => void;
}

const PROMOTION_KINDS = ["issue", "wiki", "decision"] as const;

export function PromoteRoomDialog({
  source,
  pending,
  onOpenChange,
  onPromote,
}: PromoteRoomDialogProps) {
  const { t } = useT("rooms");
  const [kind, setKind] = useState<RoomPromotionKind>("issue");
  const [title, setTitle] = useState("");
  const [rationale, setRationale] = useState("");

  useEffect(() => {
    if (source) setTitle(source.suggestedTitle);
  }, [source]);

  const close = () => {
    setKind("issue");
    setTitle("");
    setRationale("");
    onOpenChange(false);
  };

  const submit = () => {
    if (!source || !title.trim()) return;
    const shared = {
      kind,
      idempotency_key: createSafeId(),
      title: title.trim(),
      rationale: rationale.trim() || undefined,
    };
    let input: PromoteRoomArtifactInput;
    if (source.entryId !== undefined) {
      input = { ...shared, entry_id: source.entryId };
    } else if (source.cycleId !== undefined) {
      input = { ...shared, cycle_id: source.cycleId };
    } else {
      return;
    }
    onPromote(input, close);
  };

  return (
    <Dialog open={source !== null} onOpenChange={(open) => !open && !pending && close()}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>{t(($) => $.promote.title)}</DialogTitle>
          <DialogDescription className="sr-only">
            {t(($) => $.promote.description)}
          </DialogDescription>
        </DialogHeader>
        <div className="space-y-3">
          <label className="block space-y-1.5 text-caption font-medium text-muted-foreground">
            <span>{t(($) => $.promote.fields.kind)}</span>
            <Select
              items={PROMOTION_KINDS.map((value) => ({
                value,
                label: t(($) => $.promote.kinds[value]),
              }))}
              value={kind}
              onValueChange={(value) => {
                if (value === "issue" || value === "wiki" || value === "decision") {
                  setKind(value);
                }
              }}
            >
              <SelectTrigger className="w-full">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {PROMOTION_KINDS.map((value) => (
                  <SelectItem key={value} value={value}>
                    {t(($) => $.promote.kinds[value])}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </label>
          <label className="block space-y-1.5 text-caption font-medium text-muted-foreground">
            <span>{t(($) => $.promote.fields.title)}</span>
            <Input
              value={title}
              maxLength={300}
              placeholder={t(($) => $.promote.placeholders.title)}
              onChange={(event) => setTitle(event.target.value)}
            />
          </label>
          <label className="block space-y-1.5 text-caption font-medium text-muted-foreground">
            <span>{t(($) => $.promote.fields.rationale)}</span>
            <Textarea
              value={rationale}
              className="min-h-20 resize-y"
              placeholder={t(($) => $.promote.placeholders.rationale)}
              onChange={(event) => setRationale(event.target.value)}
            />
          </label>
        </div>
        <DialogFooter>
          <Button type="button" variant="outline" disabled={pending} onClick={close}>
            {t(($) => $.actions.cancel)}
          </Button>
          <Button type="button" disabled={pending || !title.trim()} data-testid="room-promote-submit" onClick={submit}>
            {pending ? <Loader2 className="animate-spin" aria-hidden="true" /> : null}
            {pending ? t(($) => $.actions.promoting) : t(($) => $.actions.promote)}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
