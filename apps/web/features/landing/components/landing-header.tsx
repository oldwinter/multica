"use client";

import { useState } from "react";
import Link from "next/link";
import { Menu, Monitor, Moon, Palette, Sun, X } from "lucide-react";
import { MulticaIcon } from "@multica/ui/components/common/multica-icon";
import {
  SKIN_IDS,
  parseSkin,
  useSkin,
  useTheme,
  type Skin,
} from "@multica/ui/components/common/theme-provider";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@multica/ui/components/ui/dropdown-menu";
import { cn } from "@multica/ui/lib/utils";
import { useAuthStore } from "@multica/core/auth";
import { docsHrefForLocale, useLocale, type Locale } from "../i18n";
import { useDashboardCtaHref } from "../utils/use-dashboard-cta";
import { formatStarCount, useGithubStars } from "../utils/use-github-stars";
import { GitHubMark, githubUrl, headerButtonClassName } from "./shared";

export function LandingHeader({
  variant = "dark",
}: {
  variant?: "dark" | "light";
}) {
  const { t, locale } = useLocale();
  const user = useAuthStore((s) => s.user);
  const stars = useGithubStars();
  const starsLabel = stars != null ? formatStarCount(stars) : null;
  const [isMenuOpen, setIsMenuOpen] = useState(false);
  const docsHref = docsHrefForLocale(locale);
  const navLinks = [
    { href: "/usecases", label: t.header.useCases },
    { href: docsHref, label: t.header.docs },
    { href: "/changelog", label: t.header.changelog },
  ];
  const ctaHref = useDashboardCtaHref();
  const ctaLabel = user ? t.header.dashboard : t.header.cta;

  return (
    <header
      className={cn(
        "relative inset-x-0 top-0 z-30",
        variant === "dark"
          ? "absolute bg-transparent"
          : "border-b border-border bg-surface",
      )}
    >
      <div className="mx-auto flex h-[76px] max-w-[1320px] items-center justify-between px-4 sm:px-6 lg:px-8">
        <div className="flex min-w-0 items-center gap-6 lg:gap-8">
          <Link href="/" className="flex shrink-0 items-center gap-3">
            <MulticaIcon
              className={cn(
                "size-5",
                variant === "dark" ? "text-white" : "text-foreground",
              )}
              noSpin
            />
            <span
              className={cn(
                "text-title font-semibold tracking-normal lowercase sm:text-title-lg",
                variant === "dark" ? "text-white/92" : "text-foreground",
              )}
            >
              multica
            </span>
          </Link>

          <nav
            aria-label={t.header.navigation}
            className="hidden items-center gap-1 md:flex"
          >
            {navLinks.map((link) => (
              <Link
                key={link.href}
                href={link.href}
                className={navLinkClassName(variant)}
              >
                {link.label}
              </Link>
            ))}
          </nav>
        </div>

        <div className="flex shrink-0 items-center gap-2 sm:gap-2.5">
          <AppearancePicker locale={locale} variant={variant} />
          <button
            type="button"
            aria-label={isMenuOpen ? t.header.closeMenu : t.header.openMenu}
            aria-expanded={isMenuOpen}
            onClick={() => setIsMenuOpen((open) => !open)}
            className={cn(
              headerButtonClassName("ghost", variant),
              "size-11 px-0 md:hidden",
            )}
          >
            {isMenuOpen ? (
              <X className="size-4" aria-hidden />
            ) : (
              <Menu className="size-4" aria-hidden />
            )}
          </button>
          <Link
            href={githubUrl}
            target="_blank"
            rel="noreferrer"
            className={cn(
              headerButtonClassName("ghost", variant),
              "hidden lg:inline-flex",
            )}
          >
            <GitHubMark className="size-3.5" />
            {t.header.github}
            {starsLabel ? <GitHubStarsBadge label={starsLabel} /> : null}
          </Link>
          <Link
            href={ctaHref}
            className={headerButtonClassName("solid", variant)}
          >
            {ctaLabel}
          </Link>
        </div>
      </div>

      {isMenuOpen ? (
        <div
          className={cn(
            "absolute left-4 right-4 top-[calc(100%+8px)] z-50 rounded-[14px] border p-2 shadow-[0_18px_60px_rgba(0,0,0,0.18)] backdrop-blur-xl md:hidden",
            variant === "dark"
              ? "border-white/14 bg-[var(--landing-night)] text-white"
              : "border-border bg-surface text-foreground",
          )}
        >
          <nav aria-label={t.header.navigation} className="flex flex-col">
            {navLinks.map((link) => (
              <Link
                key={link.href}
                href={link.href}
                onClick={() => setIsMenuOpen(false)}
                className={mobileNavLinkClassName(variant)}
              >
                {link.label}
              </Link>
            ))}
          </nav>
          <div
            className={cn(
              "mt-2 border-t pt-2",
              variant === "dark" ? "border-white/10" : "border-border",
            )}
          >
            <Link
              href={githubUrl}
              target="_blank"
              rel="noreferrer"
              onClick={() => setIsMenuOpen(false)}
              className={mobileNavLinkClassName(variant)}
            >
              <GitHubMark className="size-3.5" />
              {t.header.github}
              {starsLabel ? <GitHubStarsBadge label={starsLabel} /> : null}
            </Link>
          </div>
        </div>
      ) : null}
    </header>
  );
}

const APPEARANCE_COPY: Record<
  Locale,
  {
    title: string;
    skin: string;
    mode: string;
    skins: Record<Skin, string>;
    modes: Record<"system" | "light" | "dark", string>;
  }
> = {
  en: {
    title: "Appearance",
    skin: "Skin",
    mode: "Mode",
    skins: { tension: "Tension", relay: "Relay", field: "Field" },
    modes: { system: "System", light: "Light", dark: "Dark" },
  },
  "zh-Hans": {
    title: "外观",
    skin: "皮肤",
    mode: "模式",
    skins: { tension: "张力", relay: "中继", field: "场域" },
    modes: { system: "跟随系统", light: "浅色", dark: "深色" },
  },
  ko: {
    title: "화면 모양",
    skin: "스킨",
    mode: "모드",
    skins: { tension: "텐션", relay: "릴레이", field: "필드" },
    modes: { system: "시스템", light: "라이트", dark: "다크" },
  },
  ja: {
    title: "外観",
    skin: "スキン",
    mode: "モード",
    skins: { tension: "テンション", relay: "リレー", field: "フィールド" },
    modes: { system: "システム", light: "ライト", dark: "ダーク" },
  },
};

const APPEARANCE_MODES = [
  { value: "system" as const, icon: Monitor },
  { value: "light" as const, icon: Sun },
  { value: "dark" as const, icon: Moon },
];

function AppearancePicker({
  locale,
  variant,
}: {
  locale: Locale;
  variant: "dark" | "light";
}) {
  const { skin, setSkin } = useSkin();
  const { theme, setTheme } = useTheme();
  const copy = APPEARANCE_COPY[locale];

  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        render={
          <button
            type="button"
            aria-label={copy.title}
            title={copy.title}
            className={cn(
              headerButtonClassName("ghost", variant),
              "size-11 px-0 sm:size-9",
            )}
          >
            <Palette className="size-4" aria-hidden="true" />
          </button>
        }
      />
      <DropdownMenuContent align="end" className="min-w-44">
        <DropdownMenuGroup>
          <DropdownMenuLabel>{copy.skin}</DropdownMenuLabel>
          <DropdownMenuRadioGroup
            value={skin}
            onValueChange={(value) => setSkin(parseSkin(value))}
          >
            {SKIN_IDS.map((option) => (
              <DropdownMenuRadioItem
                key={option}
                value={option}
                className="min-h-11 sm:min-h-8"
              >
                {copy.skins[option]}
              </DropdownMenuRadioItem>
            ))}
          </DropdownMenuRadioGroup>
        </DropdownMenuGroup>
        <DropdownMenuSeparator />
        <DropdownMenuGroup>
          <DropdownMenuLabel>{copy.mode}</DropdownMenuLabel>
          <DropdownMenuRadioGroup
            value={theme ?? "system"}
            onValueChange={(value) => setTheme(value)}
          >
            {APPEARANCE_MODES.map((option) => {
              const Icon = option.icon;
              return (
                <DropdownMenuRadioItem
                  key={option.value}
                  value={option.value}
                  className="min-h-11 gap-2 sm:min-h-8"
                >
                  <Icon className="size-4 text-muted-foreground" />
                  {copy.modes[option.value]}
                </DropdownMenuRadioItem>
              );
            })}
          </DropdownMenuRadioGroup>
        </DropdownMenuGroup>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

/** Star-count segment appended to the header's GitHub button — a faint
 *  divider and the compact count (e.g. "37.6k"). No star glyph: in the GitHub
 *  button context the number reads as the star count on its own. Inherits the
 *  button's text color so it adapts to both the dark and light header
 *  variants. */
function GitHubStarsBadge({ label }: { label: string }) {
  return (
    <span className="inline-flex items-center gap-1.5 tabular-nums">
      <span aria-hidden className="h-3 w-px bg-current opacity-25" />
      {label}
    </span>
  );
}

function navLinkClassName(variant: "dark" | "light") {
  return cn(
    "inline-flex h-9 items-center rounded-[9px] px-3 text-label font-medium transition-colors",
    variant === "dark"
      ? "text-white/72 hover:bg-surface/8 hover:text-white"
      : "text-muted-foreground hover:bg-muted hover:text-foreground",
  );
}

function mobileNavLinkClassName(variant: "dark" | "light") {
  return cn(
    "flex min-h-11 items-center gap-2 rounded-[10px] px-3 text-body font-medium transition-colors",
    variant === "dark"
      ? "text-white/76 hover:bg-surface/8 hover:text-white"
      : "text-muted-foreground hover:bg-muted hover:text-foreground",
  );
}
