"use client";

import Link from "next/link";
import { LandingHeader } from "./landing-header";
import { LandingFooter } from "./landing-footer";
import { GitHubMark, githubUrl } from "./shared";
import { useLocale } from "../i18n";

export function AboutPageClient() {
  const { t } = useLocale();
  const n = t.about.nameLine;

  return (
    <>
      <LandingHeader variant="light" />
      <main className="bg-surface text-foreground">
        <div className="mx-auto max-w-[720px] px-4 py-16 sm:px-6 sm:py-20 lg:py-24">
          <h1 className="landing-serif text-[2.6rem] leading-[1.05] tracking-normal sm:text-[3.4rem]">
            {t.about.title}
          </h1>
          <div className="mt-8 space-y-6 text-body-lg leading-[1.8] text-muted-foreground sm:text-title-sm">
            <p>
              {n.prefix}
              <strong className="font-semibold text-foreground">
                {n.mul}
              </strong>
              {n.tiplexed}
              <strong className="font-semibold text-foreground">
                {n.i}
              </strong>
              {n.nformationAnd}
              <strong className="font-semibold text-foreground">
                {n.c}
              </strong>
              {n.omputing}
              <strong className="font-semibold text-foreground">
                {n.a}
              </strong>
              {n.gent}
            </p>
            {t.about.paragraphs.map((p, i) => (
              <p key={i}>{p}</p>
            ))}
          </div>

          <div className="mt-12">
            <Link
              href={githubUrl}
              target="_blank"
              rel="noreferrer"
              className="inline-flex items-center gap-2.5 rounded-[12px] bg-[var(--landing-night)] px-5 py-3 text-body font-semibold text-white transition-colors hover:bg-[var(--landing-night-hover)]"
            >
              <GitHubMark className="size-4" />
              {t.about.cta}
            </Link>
          </div>
        </div>
      </main>
      <LandingFooter />
    </>
  );
}
