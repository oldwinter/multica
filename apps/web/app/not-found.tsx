"use client";

import Link from "next/link";
import {
  DEFAULT_LOCALE,
  SUPPORTED_LOCALES,
  type SupportedLocale,
} from "@multica/core/i18n";
import { paths } from "@multica/core/paths";
import { buttonVariants } from "@multica/ui/components/ui/button";
import { useLocale, useT } from "@multica/views/i18n";
import { docsHrefForLocale } from "@/lib/docs-href";

export default function NotFound() {
  const { t } = useT("common");
  const locale = useLocale();
  const docsLocale: SupportedLocale = SUPPORTED_LOCALES.includes(
    locale as SupportedLocale,
  )
    ? (locale as SupportedLocale)
    : DEFAULT_LOCALE;
  return (
    <main className="flex min-h-screen flex-col items-center justify-center gap-6 px-6 py-24 text-center">
      <p className="text-body font-medium text-muted-foreground">404</p>
      <h1 className="text-display-sm font-semibold tracking-tight">
        {t(($) => $.not_found.title)}
      </h1>
      <p className="max-w-md text-body text-muted-foreground">
        {t(($) => $.not_found.description)}
      </p>
      <div className="mt-2 flex flex-wrap items-center justify-center gap-3">
        <Link href="/" className={buttonVariants()}>
          {t(($) => $.not_found.back_to_multica)}
        </Link>
        <Link
          href={paths.login()}
          className={buttonVariants({ variant: "outline" })}
        >
          {t(($) => $.not_found.log_in)}
        </Link>
        <Link
          href={docsHrefForLocale(docsLocale)}
          className={buttonVariants({ variant: "outline" })}
        >
          {t(($) => $.not_found.read_docs)}
        </Link>
      </div>
    </main>
  );
}
