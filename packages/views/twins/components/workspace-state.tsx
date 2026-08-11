"use client";

import { AlertTriangle, LoaderCircle, LockKeyhole } from "lucide-react";
import { Button } from "@multica/ui/components/ui/button";
import { useT } from "../../i18n";

export function WorkspaceState({ state, onRetry }: { state: "loading" | "error"; onRetry: () => void }) {
  const { t } = useT("twins");
  const loading = state === "loading";
  return (
    <section
      className="flex min-h-72 items-center justify-center rounded-lg border border-surface-border bg-surface px-6 py-12 shadow-[var(--surface-shadow)]"
      role={loading ? "status" : "alert"}
      aria-busy={loading}
    >
      <div className="flex max-w-md flex-col items-center gap-3 text-center">
        {loading
          ? <LoaderCircle className="size-5 animate-spin text-muted-foreground motion-reduce:animate-none" aria-hidden="true" />
          : <AlertTriangle className="size-5 text-destructive" aria-hidden="true" />}
        <h2 className="text-title font-medium text-foreground">
          {loading ? t(($) => $.states.loading) : t(($) => $.states.error_title)}
        </h2>
        <p className="text-body text-muted-foreground">
          {loading ? t(($) => $.states.loading_description) : t(($) => $.states.error_description)}
        </p>
        {!loading ? <Button variant="outline" onClick={onRetry}>{t(($) => $.actions.try_again)}</Button> : null}
      </div>
    </section>
  );
}

export function ReadOnlyNotice() {
  const { t } = useT("twins");
  return (
    <section className="flex items-start gap-3 rounded-lg border border-surface-border bg-surface px-4 py-3" aria-label={t(($) => $.permissions.read_only)}>
      <LockKeyhole className="mt-0.5 size-4 shrink-0 text-muted-foreground" aria-hidden="true" />
      <div>
        <p className="text-label font-medium text-foreground">{t(($) => $.permissions.read_only)}</p>
        <p className="text-body text-muted-foreground">{t(($) => $.permissions.read_only_description)}</p>
      </div>
    </section>
  );
}
