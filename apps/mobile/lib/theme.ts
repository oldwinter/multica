import { DarkTheme, DefaultTheme, type Theme } from "@react-navigation/native";
import {
  missingSemanticTokenRoles,
  SEMANTIC_TOKEN_CONTRACT_VERSION,
  SEMANTIC_TOKEN_ROLES,
  type SemanticTokenRole,
} from "@multica/core/constants/semantic-token-schema";

export const SKIN_IDS = ["tension", "relay", "field"] as const;
export type AppSkin = (typeof SKIN_IDS)[number];
export type AppColorScheme = "light" | "dark";

export type AppTheme = {
  background: string;
  foreground: string;
  card: string;
  cardForeground: string;
  popover: string;
  popoverForeground: string;
  primary: string;
  primaryForeground: string;
  secondary: string;
  secondaryForeground: string;
  muted: string;
  mutedForeground: string;
  accent: string;
  accentForeground: string;
  destructive: string;
  destructiveForeground: string;
  border: string;
  input: string;
  ring: string;
  radius: string;
  chart1: string;
  chart2: string;
  chart3: string;
  chart4: string;
  chart5: string;
  brand: string;
  brandForeground: string;
  success: string;
  warning: string;
  info: string;
  priority: string;
  codeSurface: string;
  surface1: string;
  surface2: string;
};

export const MOBILE_SEMANTIC_TOKEN_CONTRACT_VERSION = SEMANTIC_TOKEN_CONTRACT_VERSION;

const MOBILE_SEMANTIC_ROLE_SOURCES = {
  canvas: "background",
  shell: "surface1",
  surface: "card",
  raisedSurface: "popover",
  surfaceHover: "surface2",
  selection: "accent",
  selectionForeground: "accentForeground",
  border: "border",
  controlBorder: "input",
  text: "foreground",
  mutedText: "mutedForeground",
  disabledText: "mutedForeground",
  focus: "ring",
  brand: "brand",
  brandForeground: "brandForeground",
  destructive: "destructive",
  destructiveForeground: "destructiveForeground",
  success: "success",
  warning: "warning",
  info: "info",
  statusBacklog: "mutedForeground",
  statusTodo: "foreground",
  statusInProgress: "info",
  statusDone: "success",
  statusCancelled: "mutedForeground",
  chart1: "chart1",
  chart2: "chart2",
  chart3: "chart3",
  chart4: "chart4",
  chart5: "chart5",
  codeSurface: "codeSurface",
  codeText: "foreground",
  platformChrome: "surface1",
  platformChromeForeground: "foreground",
} satisfies Record<SemanticTokenRole, keyof AppTheme>;

export function validateMobileThemeContract(palette: Partial<AppTheme>) {
  const values = Object.fromEntries(
    SEMANTIC_TOKEN_ROLES.map((role) => [role, palette[MOBILE_SEMANTIC_ROLE_SOURCES[role]]]),
  );
  return missingSemanticTokenRoles(values).map((role) => ({
    role,
    source: MOBILE_SEMANTIC_ROLE_SOURCES[role],
  }));
}

const tensionLight: AppTheme = {
  background: "hsl(20 14% 98%)",
  foreground: "hsl(20 10% 8%)",
  card: "hsl(0 0% 100%)",
  cardForeground: "hsl(20 10% 8%)",
  popover: "hsl(0 0% 100%)",
  popoverForeground: "hsl(20 10% 8%)",
  primary: "hsl(5 72% 49%)",
  primaryForeground: "hsl(20 20% 99%)",
  secondary: "hsl(20 8% 95%)",
  secondaryForeground: "hsl(20 10% 12%)",
  muted: "hsl(20 8% 95%)",
  mutedForeground: "hsl(20 6% 42%)",
  accent: "hsl(8 48% 94%)",
  accentForeground: "hsl(5 62% 34%)",
  destructive: "hsl(356 67% 48%)",
  destructiveForeground: "hsl(0 0% 98%)",
  border: "hsl(20 7% 84%)",
  input: "hsl(20 7% 84%)",
  ring: "hsl(5 72% 49%)",
  radius: "0.5rem",
  chart1: "hsl(5 72% 49%)",
  chart2: "hsl(178 48% 37%)",
  chart3: "hsl(42 68% 48%)",
  chart4: "hsl(286 38% 48%)",
  chart5: "hsl(143 42% 38%)",
  brand: "hsl(5 72% 49%)",
  brandForeground: "hsl(20 20% 99%)",
  success: "hsl(143 52% 36%)",
  warning: "hsl(42 76% 45%)",
  info: "hsl(206 70% 46%)",
  priority: "hsl(24 82% 49%)",
  codeSurface: "hsl(20 7% 92%)",
  surface1: "hsl(20 10% 97%)",
  surface2: "hsl(20 7% 90%)",
};

const tensionDark: AppTheme = {
  ...tensionLight,
  background: "hsl(20 12% 7%)",
  foreground: "hsl(20 10% 96%)",
  card: "hsl(20 10% 10%)",
  cardForeground: "hsl(20 10% 96%)",
  popover: "hsl(20 10% 12%)",
  popoverForeground: "hsl(20 10% 96%)",
  primary: "hsl(6 80% 65%)",
  primaryForeground: "hsl(20 15% 8%)",
  secondary: "hsl(20 8% 15%)",
  secondaryForeground: "hsl(20 10% 96%)",
  muted: "hsl(20 8% 15%)",
  mutedForeground: "hsl(20 6% 66%)",
  accent: "hsl(5 30% 20%)",
  accentForeground: "hsl(6 70% 82%)",
  destructive: "hsl(356 72% 66%)",
  border: "hsl(20 7% 25%)",
  input: "hsl(20 7% 25%)",
  ring: "hsl(6 80% 65%)",
  chart1: "hsl(6 80% 65%)",
  chart2: "hsl(178 48% 58%)",
  chart3: "hsl(42 68% 62%)",
  chart4: "hsl(286 42% 66%)",
  chart5: "hsl(143 42% 58%)",
  brand: "hsl(6 80% 65%)",
  codeSurface: "hsl(20 8% 18%)",
  surface1: "hsl(20 9% 10%)",
  surface2: "hsl(20 8% 19%)",
};

function withPalette(base: AppTheme, values: Partial<AppTheme>): AppTheme {
  return { ...base, ...values };
}

const relayLight = withPalette(tensionLight, {
  background: "hsl(195 18% 98%)",
  foreground: "hsl(205 20% 8%)",
  cardForeground: "hsl(205 20% 8%)",
  popoverForeground: "hsl(205 20% 8%)",
  primary: "hsl(174 55% 34%)",
  primaryForeground: "hsl(180 25% 99%)",
  secondary: "hsl(195 14% 95%)",
  secondaryForeground: "hsl(205 20% 12%)",
  muted: "hsl(195 14% 95%)",
  mutedForeground: "hsl(205 8% 42%)",
  accent: "hsl(174 35% 93%)",
  accentForeground: "hsl(174 55% 28%)",
  ring: "hsl(174 55% 34%)",
  chart1: "hsl(174 55% 34%)",
  chart2: "hsl(10 72% 58%)",
  brand: "hsl(174 55% 34%)",
  codeSurface: "hsl(195 10% 92%)",
  surface1: "hsl(195 16% 97%)",
  surface2: "hsl(195 10% 90%)",
});

const relayDark = withPalette(tensionDark, {
  background: "hsl(205 20% 7%)",
  foreground: "hsl(195 14% 96%)",
  card: "hsl(205 18% 10%)",
  cardForeground: "hsl(195 14% 96%)",
  popover: "hsl(205 18% 12%)",
  popoverForeground: "hsl(195 14% 96%)",
  primary: "hsl(172 56% 58%)",
  primaryForeground: "hsl(205 24% 8%)",
  secondary: "hsl(205 14% 15%)",
  secondaryForeground: "hsl(195 14% 96%)",
  muted: "hsl(205 14% 15%)",
  mutedForeground: "hsl(195 8% 66%)",
  accent: "hsl(174 34% 20%)",
  accentForeground: "hsl(172 50% 82%)",
  ring: "hsl(172 56% 58%)",
  chart1: "hsl(172 56% 58%)",
  chart2: "hsl(10 76% 68%)",
  brand: "hsl(172 56% 58%)",
  codeSurface: "hsl(205 14% 18%)",
  surface1: "hsl(205 16% 10%)",
  surface2: "hsl(205 14% 19%)",
});

const fieldLight = withPalette(tensionLight, {
  background: "hsl(120 10% 98%)",
  foreground: "hsl(145 15% 8%)",
  cardForeground: "hsl(145 15% 8%)",
  popoverForeground: "hsl(145 15% 8%)",
  primary: "hsl(143 42% 34%)",
  primaryForeground: "hsl(120 18% 99%)",
  secondary: "hsl(125 10% 95%)",
  secondaryForeground: "hsl(145 15% 12%)",
  muted: "hsl(125 10% 95%)",
  mutedForeground: "hsl(145 7% 42%)",
  accent: "hsl(138 24% 93%)",
  accentForeground: "hsl(143 42% 28%)",
  ring: "hsl(143 42% 34%)",
  chart1: "hsl(143 42% 34%)",
  chart2: "hsl(42 74% 48%)",
  brand: "hsl(143 42% 34%)",
  warning: "hsl(42 74% 45%)",
  codeSurface: "hsl(125 8% 92%)",
  surface1: "hsl(125 10% 97%)",
  surface2: "hsl(125 8% 90%)",
});

const fieldDark = withPalette(tensionDark, {
  background: "hsl(145 18% 7%)",
  foreground: "hsl(120 10% 96%)",
  card: "hsl(145 16% 10%)",
  cardForeground: "hsl(120 10% 96%)",
  popover: "hsl(145 16% 12%)",
  popoverForeground: "hsl(120 10% 96%)",
  primary: "hsl(142 45% 58%)",
  primaryForeground: "hsl(145 20% 8%)",
  secondary: "hsl(145 12% 15%)",
  secondaryForeground: "hsl(120 10% 96%)",
  muted: "hsl(145 12% 15%)",
  mutedForeground: "hsl(130 6% 66%)",
  accent: "hsl(143 28% 20%)",
  accentForeground: "hsl(142 38% 82%)",
  ring: "hsl(142 45% 58%)",
  chart1: "hsl(142 45% 58%)",
  chart2: "hsl(42 74% 62%)",
  brand: "hsl(142 45% 58%)",
  warning: "hsl(42 74% 60%)",
  codeSurface: "hsl(145 12% 18%)",
  surface1: "hsl(145 14% 10%)",
  surface2: "hsl(145 12% 19%)",
});

export const THEMES: Record<AppSkin, Record<AppColorScheme, AppTheme>> = {
  tension: { light: tensionLight, dark: tensionDark },
  relay: { light: relayLight, dark: relayDark },
  field: { light: fieldLight, dark: fieldDark },
};

function navigationTheme(palette: AppTheme, scheme: AppColorScheme): Theme {
  return {
    ...(scheme === "dark" ? DarkTheme : DefaultTheme),
    colors: {
      background: palette.background,
      border: palette.border,
      card: palette.card,
      notification: palette.destructive,
      primary: palette.primary,
      text: palette.foreground,
    },
  };
}

export const NAV_THEMES: Record<AppSkin, Record<AppColorScheme, Theme>> = {
  tension: {
    light: navigationTheme(tensionLight, "light"),
    dark: navigationTheme(tensionDark, "dark"),
  },
  relay: {
    light: navigationTheme(relayLight, "light"),
    dark: navigationTheme(relayDark, "dark"),
  },
  field: {
    light: navigationTheme(fieldLight, "light"),
    dark: navigationTheme(fieldDark, "dark"),
  },
};
