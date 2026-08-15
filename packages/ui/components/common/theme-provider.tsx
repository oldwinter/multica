"use client";

import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ComponentProps,
  type ReactNode,
} from "react";
import {
  ThemeProvider as NextThemesProvider,
  useTheme as useNextTheme,
} from "next-themes";
import { useReducedMotion } from "motion/react";
import { TooltipProvider } from "../ui/tooltip";

export const SKIN_IDS = ["tension", "relay", "field"] as const;
export type Skin = (typeof SKIN_IDS)[number];

export const DEFAULT_SKIN: Skin = "tension";
export const SKIN_STORAGE_KEY = "multica-skin";

export type ThemeTransitionOrigin = {
  x: number;
  y: number;
};

type ViewTransitionDocument = Document & {
  startViewTransition(update: () => void): { finished: Promise<void> };
};

type SkinContextValue = {
  skin: Skin;
  setSkin: (skin: Skin, origin?: ThemeTransitionOrigin) => void;
};

const SkinContext = createContext<SkinContextValue | null>(null);

export function parseSkin(value: unknown): Skin {
  if (value === "tension" || value === "relay" || value === "field") return value;
  return DEFAULT_SKIN;
}

function supportsViewTransitions(value: Document): value is ViewTransitionDocument {
  return "startViewTransition" in value;
}

function applySkin(skin: Skin) {
  document.documentElement.dataset.skin = skin;
}

function storedSkin(): Skin {
  try {
    return parseSkin(window.localStorage.getItem(SKIN_STORAGE_KEY));
  } catch {
    return DEFAULT_SKIN;
  }
}

function persistSkin(skin: Skin) {
  try {
    window.localStorage.setItem(SKIN_STORAGE_KEY, skin);
  } catch {
    return;
  }
}

function activeControlOrigin(): ThemeTransitionOrigin | undefined {
  const active = document.activeElement;
  if (!(active instanceof HTMLElement) || active === document.body) return;
  const bounds = active.getBoundingClientRect();
  if (bounds.width === 0 && bounds.height === 0) return;
  return {
    x: bounds.left + bounds.width / 2,
    y: bounds.top + bounds.height / 2,
  };
}

function useThemeTransition() {
  const reduceMotion = useReducedMotion() ?? false;

  return useCallback(
    (update: () => void, origin?: ThemeTransitionOrigin) => {
      if (reduceMotion || !supportsViewTransitions(document)) {
        update();
        return;
      }

      const root = document.documentElement;
      const resolvedOrigin = origin ?? activeControlOrigin();
      const x = resolvedOrigin?.x ?? window.innerWidth / 2;
      const y = resolvedOrigin?.y ?? window.innerHeight;
      root.style.setProperty("--theme-reveal-x", `${x}px`);
      root.style.setProperty("--theme-reveal-y", `${y}px`);
      root.dataset.themeTransition = "reveal";

      const transition = document.startViewTransition(update);
      void transition.finished.finally(() => {
        delete root.dataset.themeTransition;
        root.style.removeProperty("--theme-reveal-x");
        root.style.removeProperty("--theme-reveal-y");
      });
    },
    [reduceMotion],
  );
}

export function SkinProvider({ children }: { children: ReactNode }) {
  const [skin, setSkinState] = useState<Skin>(DEFAULT_SKIN);
  const transition = useThemeTransition();

  useEffect(() => {
    const initial = parseSkin(document.documentElement.dataset.skin ?? storedSkin());
    applySkin(initial);
    setSkinState(initial);

    const syncSkin = (event: StorageEvent) => {
      if (event.key !== SKIN_STORAGE_KEY) return;
      const next = parseSkin(event.newValue);
      applySkin(next);
      setSkinState(next);
    };
    window.addEventListener("storage", syncSkin);
    return () => window.removeEventListener("storage", syncSkin);
  }, []);

  const setSkin = useCallback(
    (next: Skin, origin?: ThemeTransitionOrigin) => {
      if (next === skin) return;
      transition(() => {
        applySkin(next);
        setSkinState(next);
        persistSkin(next);
      }, origin);
    },
    [skin, transition],
  );

  const value = useMemo(() => ({ skin, setSkin }), [skin, setSkin]);
  return <SkinContext.Provider value={value}>{children}</SkinContext.Provider>;
}

export function useSkin() {
  const value = useContext(SkinContext);
  if (!value) throw new Error("useSkin must be used within SkinProvider");
  return value;
}

export function useTheme() {
  const value = useNextTheme();
  const transition = useThemeTransition();
  const setTheme = useCallback(
    (next: string, origin?: ThemeTransitionOrigin) => {
      if (next === value.theme) return;
      if (next === "system") {
        value.setTheme(next);
        return;
      }
      transition(() => value.setTheme(next), origin);
    },
    [transition, value],
  );
  return { ...value, setTheme };
}

export function ThemeProvider({
  children,
  ...props
}: ComponentProps<typeof NextThemesProvider>) {
  return (
    <NextThemesProvider
      attribute="class"
      defaultTheme="system"
      enableSystem
      disableTransitionOnChange
      {...props}
    >
      <SkinProvider>
        <TooltipProvider delay={500}>{children}</TooltipProvider>
      </SkinProvider>
    </NextThemesProvider>
  );
}
