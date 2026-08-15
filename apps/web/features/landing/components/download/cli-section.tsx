"use client";

import { useState } from "react";
import { Check, Copy, Terminal } from "lucide-react";
import { copyText } from "@multica/ui/lib/clipboard";
import { useLocale } from "../../i18n";

const INSTALL_CMD =
  "curl -fsSL https://raw.githubusercontent.com/multica-ai/multica/main/scripts/install.sh | bash";
const SETUP_CMD = "multica setup";

/**
 * Scenario-first CLI section. Copy leans into servers / remote dev
 * boxes / headless setups rather than positioning CLI as a
 * lightweight Desktop. Two copy-and-paste command blocks.
 */
export function CliSection() {
  const { t } = useLocale();
  const d = t.download.cli;

  return (
    <section id="cli" className="bg-background py-20 text-foreground sm:py-24">
      <div className="mx-auto max-w-[820px] px-4 sm:px-6 lg:px-8">
        <h2 className="landing-serif text-[2.2rem] leading-[1.1] tracking-normal sm:text-[2.6rem]">
          {d.title}
        </h2>
        <p className="mt-4 max-w-[620px] text-body-lg leading-7 text-muted-foreground">
          {d.sub}
        </p>

        <div className="mt-10 flex flex-col gap-5">
          <CommandBlock
            label={d.installLabel}
            cmd={INSTALL_CMD}
            copyLabel={d.copyLabel}
            copiedLabel={d.copiedLabel}
          />
          <CommandBlock
            label={d.startLabel}
            cmd={SETUP_CMD}
            copyLabel={d.copyLabel}
            copiedLabel={d.copiedLabel}
          />
        </div>

        <p className="mt-6 text-label text-muted-foreground">{d.sshNote}</p>
      </div>
    </section>
  );
}

function CommandBlock({
  label,
  cmd,
  copyLabel,
  copiedLabel,
}: {
  label: string;
  cmd: string;
  copyLabel: string;
  copiedLabel: string;
}) {
  const [copied, setCopied] = useState(false);

  const onCopy = async () => {
    if (await copyText(cmd)) {
      setCopied(true);
      setTimeout(() => setCopied(false), 1800);
    }
  };

  return (
    <div>
      <p className="mb-2 text-caption font-medium uppercase tracking-normal text-muted-foreground">
        {label}
      </p>
      <div className="flex items-start gap-3 rounded-xl border border-border bg-surface px-4 py-3 font-mono text-label">
        <Terminal
          className="mt-0.5 size-4 shrink-0 text-muted-foreground"
          aria-hidden
        />
        <code className="min-w-0 flex-1 whitespace-pre-wrap break-all">
          {cmd}
        </code>
        <button
          type="button"
          onClick={onCopy}
          aria-label={copied ? copiedLabel : copyLabel}
          className="inline-flex shrink-0 items-center gap-1.5 rounded-md px-2 py-1 text-caption font-medium text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
        >
          {copied ? (
            <>
              <Check className="size-3.5" />
              {copiedLabel}
            </>
          ) : (
            <>
              <Copy className="size-3.5" />
              {copyLabel}
            </>
          )}
        </button>
      </div>
    </div>
  );
}
