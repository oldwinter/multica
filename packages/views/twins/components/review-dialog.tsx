"use client";

import { useState } from "react";
import { Button } from "@multica/ui/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@multica/ui/components/ui/dialog";
import { Textarea } from "@multica/ui/components/ui/textarea";
import { useT } from "../../i18n";

export function ReviewDialog({
  open,
  kind,
  pending,
  onOpenChange,
  onConfirm,
}: {
  open: boolean;
  kind: "accept-wiki" | "reject-wiki" | "accept-twin" | "reject-twin";
  pending: boolean;
  onOpenChange: (open: boolean) => void;
  onConfirm: (reason: string) => void;
}) {
  const { t } = useT("twins");
  const [reason, setReason] = useState("");
  const rejects = kind === "reject-wiki" || kind === "reject-twin";
  const title = kind === "accept-wiki"
    ? t(($) => $.dialog.accept_wiki_title)
    : kind === "reject-wiki"
      ? t(($) => $.dialog.reject_wiki_title)
      : kind === "accept-twin"
        ? t(($) => $.dialog.accept_twin_title)
        : t(($) => $.dialog.reject_twin_title);
  const confirmLabel = kind === "accept-wiki"
    ? t(($) => $.dialog.confirm_acceptance)
    : kind === "accept-twin"
      ? t(($) => $.dialog.confirm_sign_off)
      : t(($) => $.dialog.confirm_rejection);

  const confirm = () => {
    onConfirm(reason.trim());
    setReason("");
    onOpenChange(false);
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
          <DialogDescription>{t(($) => $.dialog.description)}</DialogDescription>
        </DialogHeader>
        {rejects ? (
          <label className="space-y-2 text-label font-medium">
            {t(($) => $.dialog.reason)}
            <Textarea value={reason} onChange={(event) => setReason(event.target.value)} />
          </label>
        ) : null}
        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>{t(($) => $.actions.cancel)}</Button>
          <Button
            variant={rejects ? "destructive" : "brand"}
            disabled={pending || (rejects && reason.trim().length === 0)}
            onClick={confirm}
          >
            {pending ? t(($) => $.actions.saving) : confirmLabel}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
