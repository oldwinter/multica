import "../global.css";
import { RootProvider } from "fumadocs-ui/provider";
import { DocsLayout } from "fumadocs-ui/layouts/docs";
import { Inter, Geist_Mono, Source_Serif_4 } from "next/font/google";
import type { ReactNode } from "react";
import type { Metadata } from "next";
import Script from "next/script";
import { cn } from "@multica/ui/lib/utils";
import { SkinProvider } from "@multica/ui/components/common/theme-provider";
import { baseOptions } from "@/app/layout.config";
import { source } from "@/lib/source";
import { i18n, type Lang } from "@/lib/i18n";
import { uiTranslations, localeLabels } from "@/lib/translations";
import { DocsSettings } from "@/components/docs-settings";
import { DocsAppearanceProvider } from "@/components/docs-appearance-provider";

// Inter (Latin UI face) is exposed under `--font-inter`. The full `--font-sans`
// stack — Inter + the per-locale CJK fallback chain, including the Japanese-first
// override scoped to `<html lang="ja">` — is composed in static CSS in
// ./global.css (CSP-safe, no inline <style>). Mirrors apps/web/app/layout.tsx.
const inter = Inter({
  subsets: ["latin"],
  variable: "--font-inter",
});

const geistMono = Geist_Mono({
  subsets: ["latin"],
  variable: "--font-mono",
  fallback: ["ui-monospace", "SFMono-Regular", "Menlo", "Consolas", "monospace"],
});

// Editorial serif used for headings and showpiece elements. Italic style is
// deliberately NOT loaded — italic in CJK is a synthetic slant that breaks
// glyph design. Emphasis in docs is carried by brand color + weight, never
// font-style. Mirrors apps/web/app/layout.tsx for the upright family.
const sourceSerif = Source_Serif_4({
  subsets: ["latin"],
  style: ["normal"],
  variable: "--font-serif",
  fallback: [
    "ui-serif",
    "Iowan Old Style",
    "Apple Garamond",
    "Baskerville",
    "Times New Roman",
    "serif",
  ],
});

export const metadata: Metadata = {
  title: {
    template: "%s | Multica Docs",
    default: "Multica Docs",
  },
  description:
    "Documentation for Multica — the open-source managed agents platform.",
};

export function generateStaticParams() {
  return i18n.languages.map((lang) => ({ lang }));
}

export default async function Layout({
  params,
  children,
}: {
  params: Promise<{ lang: string }>;
  children: ReactNode;
}) {
  const { lang: rawLang } = await params;
  const lang = (i18n.languages as readonly string[]).includes(rawLang)
    ? (rawLang as Lang)
    : (i18n.defaultLanguage as Lang);
  const locales = i18n.languages.map((l) => ({
    locale: l,
    name: localeLabels[l],
  }));

  return (
    <html
      lang={lang}
      suppressHydrationWarning
      className={cn(
        "antialiased",
        inter.variable,
        geistMono.variable,
        sourceSerif.variable,
      )}
    >
      <body className="font-sans">
        <Script id="multica-docs-skin" strategy="beforeInteractive">
          {`var e=document.documentElement,m=false,s="tension",a="system",d=false;try{m=matchMedia("(prefers-color-scheme: dark)").matches}catch(_){}try{var r=localStorage.getItem("multica-appearance-preferences"),p=null;try{p=r?JSON.parse(r):null}catch(_){}var f=p&&(p.version>1||p.tokenContractVersion>1),v=p&&p.version===1&&p.tokenContractVersion===1;s=f?"tension":v?p.skin:localStorage.getItem("multica-skin");a=f?"system":v?p.requestedAppearance:localStorage.getItem("theme");d=a==="dark"||(a!=="light"&&m);if(v)localStorage.setItem("theme",a)}catch(_){s="tension";a="system";d=m}e.dataset.skin=["tension","relay","field"].includes(s)?s:"tension";e.classList.toggle("dark",d);e.style.colorScheme=d?"dark":"light"`}
        </Script>
        <SkinProvider>
          <RootProvider
            i18n={{
              locale: lang,
              locales,
              translations: uiTranslations[lang],
            }}
            search={{ options: { api: "/docs/api/search" } }}
          >
            <DocsAppearanceProvider>
              <DocsLayout
                tree={source.getPageTree(lang)}
                themeSwitch={{ enabled: false }}
                searchToggle={{ enabled: false }}
                sidebar={{ footer: <DocsSettings locale={lang} /> }}
                {...baseOptions}
              >
                {children}
              </DocsLayout>
            </DocsAppearanceProvider>
          </RootProvider>
        </SkinProvider>
      </body>
    </html>
  );
}
