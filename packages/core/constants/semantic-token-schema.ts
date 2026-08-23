export const SEMANTIC_TOKEN_CONTRACT_VERSION = 1;

export const SEMANTIC_TOKEN_ROLES = [
  "canvas",
  "shell",
  "surface",
  "raisedSurface",
  "surfaceHover",
  "selection",
  "selectionForeground",
  "border",
  "controlBorder",
  "text",
  "mutedText",
  "disabledText",
  "focus",
  "brand",
  "brandForeground",
  "destructive",
  "destructiveForeground",
  "success",
  "warning",
  "info",
  "statusBacklog",
  "statusTodo",
  "statusInProgress",
  "statusDone",
  "statusCancelled",
  "chart1",
  "chart2",
  "chart3",
  "chart4",
  "chart5",
  "codeSurface",
  "codeText",
  "platformChrome",
  "platformChromeForeground",
] as const;

export type SemanticTokenRole = (typeof SEMANTIC_TOKEN_ROLES)[number];
export type SemanticTokenValues = Record<SemanticTokenRole, string>;

export type SemanticContrastCategory =
  | "text"
  | "focus"
  | "controlBorders"
  | "statusGraphics"
  | "charts"
  | "code"
  | "selection"
  | "destructive"
  | "disabled";

export type SemanticContrastRequirement = {
  id: string;
  category: SemanticContrastCategory;
  foreground: SemanticTokenRole;
  background: SemanticTokenRole;
  minimum: number;
};

export const SEMANTIC_CONTRAST_REQUIREMENTS = [
  { id: "body-on-canvas", category: "text", foreground: "text", background: "canvas", minimum: 4.5 },
  {
    id: "muted-on-canvas",
    category: "text",
    foreground: "mutedText",
    background: "canvas",
    minimum: 4.5,
  },
  { id: "body-on-surface", category: "text", foreground: "text", background: "surface", minimum: 4.5 },
  { id: "focus-on-canvas", category: "focus", foreground: "focus", background: "canvas", minimum: 3 },
  {
    id: "control-border-on-canvas",
    category: "controlBorders",
    foreground: "controlBorder",
    background: "canvas",
    minimum: 3,
  },
  ...(["statusBacklog", "statusTodo", "statusInProgress", "statusDone", "statusCancelled"] as const).map(
    (foreground) => ({
      id: `${foreground}-on-canvas`,
      category: "statusGraphics" as const,
      foreground,
      background: "canvas" as const,
      minimum: 3,
    }),
  ),
  ...(["chart1", "chart2", "chart3", "chart4", "chart5"] as const).map((foreground) => ({
    id: `${foreground}-on-canvas`,
    category: "charts" as const,
    foreground,
    background: "canvas" as const,
    minimum: 3,
  })),
  {
    id: "code-on-code-surface",
    category: "code",
    foreground: "codeText",
    background: "codeSurface",
    minimum: 4.5,
  },
  {
    id: "selection-text-on-selection",
    category: "selection",
    foreground: "selectionForeground",
    background: "selection",
    minimum: 4.5,
  },
  {
    id: "destructive-text-on-destructive",
    category: "destructive",
    foreground: "destructiveForeground",
    background: "destructive",
    minimum: 4.5,
  },
  {
    id: "destructive-on-canvas",
    category: "destructive",
    foreground: "destructive",
    background: "canvas",
    minimum: 3,
  },
  {
    id: "disabled-on-canvas",
    category: "disabled",
    foreground: "disabledText",
    background: "canvas",
    minimum: 3,
  },
] satisfies ReadonlyArray<SemanticContrastRequirement>;

export function missingSemanticTokenRoles(
  values: Partial<Record<SemanticTokenRole, unknown>>,
): SemanticTokenRole[] {
  return SEMANTIC_TOKEN_ROLES.filter((role) => {
    const value = values[role];
    return typeof value !== "string" || value.trim().length === 0;
  });
}
