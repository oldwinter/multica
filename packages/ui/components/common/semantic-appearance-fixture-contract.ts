export const SEMANTIC_APPEARANCE_FIXTURE_STATES = [
  "primary-text",
  "muted-text",
  "form-control",
  "focus",
  "selection",
  "success",
  "warning",
  "destructive",
  "charts",
  "markdown",
  "code-editor",
  "overlay",
] as const;

export type SemanticAppearanceFixtureState =
  (typeof SEMANTIC_APPEARANCE_FIXTURE_STATES)[number];

export const SEMANTIC_APPEARANCE_FIXTURE_LABEL_KEYS = [
  "reviewReady",
  "updatedMomentsAgo",
  "selectedTask",
  "assignee",
  "done",
  "watch",
  "remove",
  "summary",
  "linkedTask",
  "commandMenu",
] as const;

export type SemanticAppearanceFixtureLabels = Record<
  (typeof SEMANTIC_APPEARANCE_FIXTURE_LABEL_KEYS)[number],
  string
>;

export const SEMANTIC_APPEARANCE_FIXTURE_TOKENS = [
  "--background",
  "--foreground",
  "--muted-foreground",
  "--surface",
  "--surface-selected",
  "--surface-selected-foreground",
  "--control-border",
  "--ring",
  "--success",
  "--warning",
  "--destructive",
  "--destructive-foreground",
  "--chart-1",
  "--chart-2",
  "--chart-3",
  "--code-background",
  "--code-foreground",
  "--editor-selection",
  "--popover",
  "--popover-foreground",
] as const;
