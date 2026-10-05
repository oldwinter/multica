"use client";

import { cn } from "@multica/ui/lib/utils";
import { DocumentPage, DocumentSections } from "./document-page";
import { useLocale } from "../i18n";

export function LicensingPageClient() {
  const { t } = useLocale();
  const l = t.licensing;

  return (
    <DocumentPage title={l.title} intro={l.intro}>
      <aside className="mt-12 rounded-2xl bg-background p-6 sm:p-8">
        <p className="text-micro font-semibold uppercase tracking-[0.16em] text-muted-foreground">
          {l.rule.title}
        </p>
        <p className="mt-3 text-title-sm leading-[1.7] text-foreground">
          {l.rule.text}
        </p>
      </aside>

      <section className="mt-12">
        <h2 className="text-title font-semibold leading-snug text-foreground">
          {l.scenarios.title}
        </h2>
        <table className="mt-6 w-full border-collapse text-left">
          <thead>
            <tr className="border-b border-border text-caption font-semibold uppercase tracking-[0.1em] text-muted-foreground">
              <th scope="col" className="pb-3 pr-4 font-semibold">
                {l.scenarios.scenarioColumn}
              </th>
              <th
                scope="col"
                className="whitespace-nowrap pb-3 text-right font-semibold"
              >
                {l.scenarios.licenseColumn}
              </th>
            </tr>
          </thead>
          <tbody className="divide-y divide-border">
            {l.scenarios.items.map((item) => (
              <tr key={item.scenario} className="align-top">
                <td className="py-4 pr-4">
                  <p className="text-body-lg font-medium leading-[1.6] text-foreground">
                    {item.scenario}
                  </p>
                  {item.example && (
                    <p className="mt-1 text-body leading-[1.6] text-muted-foreground">
                      {item.example}
                    </p>
                  )}
                </td>
                <td className="py-4 text-right">
                  <span
                    className={cn(
                      "inline-flex whitespace-nowrap rounded-full px-2.5 py-1 text-caption font-semibold",
                      item.required
                        ? "bg-foreground text-background"
                        : "border border-border text-muted-foreground",
                    )}
                  >
                    {item.required
                      ? l.scenarios.required
                      : l.scenarios.notRequired}
                  </span>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </section>

      <DocumentSections sections={l.sections} />
    </DocumentPage>
  );
}
