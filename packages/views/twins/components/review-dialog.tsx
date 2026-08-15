"use client";

import { useId, useRef, useState } from "react";
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
  onConfirm: (reason: string) => Promise<void>;
}) {
  const { t } = useT("twins");
  const reasonId = useId();
  const [reason, setReason] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [submissionError, setSubmissionError] = useState<string | null>(null);
  const attemptRef = useRef(0);
  const rejects = kind === "reject-wiki" || kind === "reject-twin";
  const busy = pending || submitting;
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

  const confirm = async () => {
    const attempt = ++attemptRef.current;
    setSubmitting(true);
    setSubmissionError(null);
    try {
      await onConfirm(reason.trim());
      if (attemptRef.current !== attempt) return;
      setSubmitting(false);
      setReason("");
      onOpenChange(false);
    } catch (error) {
      if (attemptRef.current !== attempt) return;
      const message = error instanceof Error && error.name === "TimeoutError"
        ? t(($) => $.errors.request_timed_out)
        : t(($) => $.errors.request_failed);
      setSubmissionError(message);
      setSubmitting(false);
    }
  };

  const changeOpen = (nextOpen: boolean) => {
    if (!nextOpen) {
      attemptRef.current += 1;
      setSubmitting(false);
      setReason("");
      setSubmissionError(null);
    }
    onOpenChange(nextOpen);
  };

  return (
    <Dialog open={open} onOpenChange={changeOpen}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
          <DialogDescription>{t(($) => $.dialog.description)}</DialogDescription>
        </DialogHeader>
        {rejects ? (
          <div className="space-y-2">
            <label htmlFor={reasonId} className="text-label font-medium">
              {t(($) => $.dialog.reason)}
            </label>
            <Textarea
              id={reasonId}
              value={reason}
              maxLength={2000}
              className="min-w-0 max-w-full field-sizing-fixed break-all"
              aria-describedby={`${reasonId}-count`}
              onChange={(event) => setReason(event.target.value)}
            />
            <p id={`${reasonId}-count`} className="text-right text-caption text-muted-foreground">
              {t(($) => $.dialog.reason_count, { count: reason.length })}
            </p>
          </div>
        ) : null}
        {submissionError ? <p role="alert" className="text-body text-destructive">{submissionError}</p> : null}
        <DialogFooter>
          <Button variant="outline" onClick={() => changeOpen(false)}>
            {busy ? t(($) => $.actions.dismiss) : t(($) => $.actions.cancel)}
          </Button>
          <Button
            variant={rejects ? "destructive" : "brand"}
            disabled={busy || (rejects && reason.trim().length === 0)}
            onClick={() => void confirm()}
          >
            {busy ? t(($) => $.actions.saving) : confirmLabel}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
