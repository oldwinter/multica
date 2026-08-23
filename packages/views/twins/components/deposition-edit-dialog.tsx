"use client";

import { useMemo, useState } from "react";
import type { LifecycleContent } from "@multica/core/twins";
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
import { Loader2 } from "lucide-react";
import { useT } from "../../i18n";

type AssertionParseResult =
  | { readonly ok: true; readonly assertions: readonly LifecycleContent[] }
  | { readonly ok: false; readonly code: "json" | "array" | "assertion" | "duplicate" };

function isRecord(value: unknown): value is LifecycleContent {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

export function assertionsForDepositionEdit(content: LifecycleContent): readonly LifecycleContent[] {
  return Array.isArray(content.assertions) ? content.assertions.filter(isRecord) : [];
}

export function parseEditedAssertions(text: string): AssertionParseResult {
  let parsed: unknown;
  try {
    parsed = JSON.parse(text);
  } catch {
    return { ok: false, code: "json" };
  }
  if (!Array.isArray(parsed)) return { ok: false, code: "array" };

  const assertions: LifecycleContent[] = [];
  const ids = new Set<string>();
  for (const item of parsed) {
    if (!isRecord(item)) return { ok: false, code: "assertion" };
    const id = typeof item.id === "string" ? item.id.trim() : "";
    const assertionText = typeof item.text === "string" ? item.text.trim() : "";
    if (!id || !assertionText) return { ok: false, code: "assertion" };
    if (ids.has(id)) return { ok: false, code: "duplicate" };
    ids.add(id);
    assertions.push(item);
  }
  return { ok: true, assertions };
}

export function DepositionEditDialog({
  assertions,
  mode,
  pending,
  onOpenChange,
  onSubmit,
}: {
  assertions: readonly LifecycleContent[];
  mode: "proposal" | "deposition";
  pending: boolean;
  onOpenChange: (open: boolean) => void;
  onSubmit: (assertions: readonly LifecycleContent[]) => Promise<void>;
}) {
  const { t } = useT("twins");
  const initialText = useMemo(() => JSON.stringify(assertions, null, 2), [assertions]);
  const [text, setText] = useState(initialText);
  const [submitting, setSubmitting] = useState(false);
  const [submissionError, setSubmissionError] = useState(false);
  const parsed = useMemo(() => parseEditedAssertions(text), [text]);
  const busy = pending || submitting;
  const dirty = parsed.ok && JSON.stringify(parsed.assertions) !== JSON.stringify(assertions);
  const title = mode === "proposal" ? t(($) => $.correction.edit_title) : t(($) => $.deposition.edit_title);
  const description = mode === "proposal" ? t(($) => $.correction.edit_description) : t(($) => $.deposition.edit_description);
  const label = mode === "proposal" ? t(($) => $.correction.edit_label) : t(($) => $.deposition.edit_label);
  const ready = mode === "proposal" ? t(($) => $.correction.edit_ready) : t(($) => $.deposition.edit_ready);
  const noChanges = mode === "proposal" ? t(($) => $.correction.edit_no_changes) : t(($) => $.deposition.edit_no_changes);
  const submitLabel = mode === "proposal" ? t(($) => $.correction.edit_submit) : t(($) => $.deposition.edit_submit);
  const savingLabel = mode === "proposal" ? t(($) => $.correction.edit_saving) : t(($) => $.deposition.edit_saving);
  const validationMessage = parsed.ok
    ? null
    : parsed.code === "json"
      ? t(($) => $.deposition.edit_invalid_json)
      : parsed.code === "array"
        ? t(($) => $.deposition.edit_array_required)
        : parsed.code === "duplicate"
          ? t(($) => $.deposition.edit_duplicate_id)
          : t(($) => $.deposition.edit_assertion_required);

  const submit = async () => {
    if (!parsed.ok || !dirty || busy) return;
    setSubmitting(true);
    setSubmissionError(false);
    try {
      await onSubmit(parsed.assertions);
      onOpenChange(false);
    } catch {
      setSubmissionError(true);
      setSubmitting(false);
    }
  };

  return (
    <Dialog open onOpenChange={(open) => !busy && onOpenChange(open)}>
      <DialogContent className="max-w-2xl">
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
          <DialogDescription>{description}</DialogDescription>
        </DialogHeader>
        <div className="min-w-0 space-y-2">
          <label htmlFor="deposition-assertions" className="text-label font-medium">
            {label}
          </label>
          <Textarea
            id="deposition-assertions"
            value={text}
            rows={14}
            maxLength={100_000}
            spellCheck={false}
            autoComplete="off"
            className="min-h-72 max-w-full resize-y font-mono text-caption"
            aria-invalid={!parsed.ok || undefined}
            aria-describedby="deposition-assertions-status"
            onChange={(event) => setText(event.target.value)}
          />
          <p
            id="deposition-assertions-status"
            className={validationMessage ? "text-caption text-destructive" : "text-caption text-muted-foreground"}
            aria-live="polite"
          >
            {validationMessage ?? (dirty
              ? ready
              : noChanges)}
          </p>
          {submissionError ? (
            <p role="alert" className="text-body text-destructive">{t(($) => $.errors.request_failed)}</p>
          ) : null}
        </div>
        <DialogFooter>
          <Button variant="outline" disabled={busy} onClick={() => onOpenChange(false)}>
            {t(($) => $.actions.cancel)}
          </Button>
          <Button variant="brand" disabled={busy || !parsed.ok || !dirty} onClick={() => void submit()}>
            {busy ? <Loader2 className="size-4 animate-spin" aria-hidden="true" /> : null}
            {busy ? savingLabel : submitLabel}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
