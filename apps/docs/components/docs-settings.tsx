"use client";

import { Monitor, Moon, Palette, RotateCcw, Sun } from "lucide-react";
import { usePathname, useRouter } from "next/navigation";
import { type ReactNode } from "react";
import { Button } from "@multica/ui/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@multica/ui/components/ui/dropdown-menu";
import { cn } from "@multica/ui/lib/utils";
import { SKIN_IDS, type SkinId } from "@multica/core/appearance";
import { i18n } from "@/lib/i18n";
import { localeLabels } from "@/lib/translations";
import { useDocsAppearance } from "@/components/docs-appearance-provider";

// Sidebar-footer chrome: a language switch on the left and a theme switch
// on the right. Replaces Fumadocs's default icon-only row, which buried
// the language option behind a tiny globe. Each control shows the current
// value as a label so the affordance is obvious at a glance.

const BASE_PATH = "/docs";

function switchLocalePath(pathname: string, target: string): string {
  // Next strips basePath before the router, so `pathname` starts at `/`
  // or `/<locale>/...`. Default-locale URLs are prefix-less.
  const segments = pathname.split("/").filter(Boolean);
  const first = segments[0];
  const hasLocalePrefix =
    first && i18n.languages.some((l) => l === first && l !== i18n.defaultLanguage);

  const rest = hasLocalePrefix ? segments.slice(1) : segments;
  const prefixed =
    target === i18n.defaultLanguage ? rest : [target, ...rest];

  return "/" + prefixed.join("/");
}

const THEME_OPTIONS: { value: string; label: string; icon: ReactNode }[] = [
  { value: "light", label: "Light", icon: <Sun className="size-4" /> },
  { value: "dark", label: "Dark", icon: <Moon className="size-4" /> },
  { value: "system", label: "System", icon: <Monitor className="size-4" /> },
];

const SKIN_LABELS: Record<SkinId, string> = {
  tension: "Tension",
  relay: "Relay",
  field: "Field",
};

const RESET_APPEARANCE_LABELS: Record<string, string> = {
  en: "Reset appearance",
  "zh-Hans": "重置外观",
  ja: "外観をリセット",
  ko: "화면 표시 재설정",
};

export function DocsSettings({ locale }: { locale: string }) {
  const router = useRouter();
  const pathname = usePathname();
  const {
    preferences,
    selectSkin,
    selectAppearance,
    reset: resetAppearance,
  } = useDocsAppearance();
  const skin = preferences.skin;
  const activeTheme = preferences.requestedAppearance;
  const activeThemeOption =
    THEME_OPTIONS.find((o) => o.value === activeTheme) ?? THEME_OPTIONS[2]!;

  const handleLocaleChange = (next: string) => {
    if (next === locale) return;
    const internal = pathname.startsWith(BASE_PATH)
      ? pathname.slice(BASE_PATH.length) || "/"
      : pathname;
    router.push(switchLocalePath(internal, next));
  };

  return (
    <div className="flex w-full items-center justify-end gap-2">
      {/* Language — left pill. Shows current language name. */}
      <DropdownMenu>
        <DropdownMenuTrigger
          render={
            <Button
              variant="ghost"
              size="sm"
              className="min-h-11 px-3 font-normal text-muted-foreground md:min-h-7"
              aria-label="Switch language"
            >
              {localeLabels[locale as keyof typeof localeLabels] ?? locale}
            </Button>
          }
        />
        <DropdownMenuContent align="start" side="top" className="min-w-[140px]">
          {i18n.languages.map((lang) => (
            <DropdownMenuItem
              key={lang}
              onClick={() => handleLocaleChange(lang)}
              className={cn(lang === locale && "bg-accent")}
            >
              {localeLabels[lang as keyof typeof localeLabels]}
            </DropdownMenuItem>
          ))}
        </DropdownMenuContent>
      </DropdownMenu>

      <DropdownMenu>
        <DropdownMenuTrigger
          render={
            <Button
              variant="ghost"
              size="icon-sm"
              className="size-11 shrink-0 text-muted-foreground md:size-7"
              aria-label={`Switch skin. Current: ${SKIN_LABELS[skin]}`}
            >
              <Palette className="size-4" />
            </Button>
          }
        />
        <DropdownMenuContent align="end" side="top" className="min-w-[140px]">
          <DropdownMenuRadioGroup
            value={skin}
            onValueChange={(value) => selectSkin(value as SkinId)}
          >
            {SKIN_IDS.map((option) => (
              <DropdownMenuRadioItem
                key={option}
                value={option}
                className="min-h-11 md:min-h-8"
              >
                {SKIN_LABELS[option]}
              </DropdownMenuRadioItem>
            ))}
          </DropdownMenuRadioGroup>
          <DropdownMenuSeparator />
          <DropdownMenuItem
            onClick={resetAppearance}
            className="min-h-11 gap-2 md:min-h-8"
          >
            <RotateCcw className="size-4" aria-hidden="true" />
            {RESET_APPEARANCE_LABELS[locale] ?? RESET_APPEARANCE_LABELS.en}
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>

      {/* Theme — right icon button. Matched height to the sm pill via
          the icon-sm size token; without this the icon variant defaults
          to 32px while size="sm" is 28px, misaligning them. */}
      <DropdownMenu>
        <DropdownMenuTrigger
          render={
            <Button
              variant="ghost"
              size="icon-sm"
              className="size-11 shrink-0 text-muted-foreground md:size-7"
              aria-label="Switch theme"
            >
              {activeThemeOption.icon}
            </Button>
          }
        />
        <DropdownMenuContent align="end" side="top" className="min-w-[140px]">
          <DropdownMenuRadioGroup
            value={activeTheme}
            onValueChange={(value) =>
              selectAppearance(value as "system" | "light" | "dark")
            }
          >
            {THEME_OPTIONS.map((opt) => (
              <DropdownMenuRadioItem
                key={opt.value}
                value={opt.value}
                className="min-h-11 gap-2 md:min-h-8"
              >
                {opt.icon}
                {opt.label}
              </DropdownMenuRadioItem>
            ))}
          </DropdownMenuRadioGroup>
        </DropdownMenuContent>
      </DropdownMenu>
    </div>
  );
}
