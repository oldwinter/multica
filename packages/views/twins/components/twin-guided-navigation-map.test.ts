// @vitest-environment node

import { describe, expect, it } from "vitest";
import type {
  TwinActivationActionKey,
  TwinActivationInspectionLink,
} from "@multica/core/twins";
import {
  resolveTwinGuide,
  type TwinGuideDestination,
  type TwinGuideRequest,
} from "./twin-guided-navigation";
import type { TwinWorkspaceTab } from "./twin-workspace-tabs";

const actionCases = [
  ["inspect_disabled", "use-status", "use"],
  ["configure_source", "wiki-source-policy", "wiki"],
  ["review_evidence", "wiki-evidence", "wiki"],
  ["refresh_evidence", "wiki-overview", "wiki"],
  ["review_twin", "twin-history", "twin"],
  ["generate_twin", "twin-overview", "twin"],
  ["compile_preview", "use-preview", "use"],
  ["configure_binding", "use-binding", "use"],
  ["run_with_twin", "use-preview", "use"],
  ["review_run", "use-effectiveness", "use"],
  ["review_deposition", "twin-history", "twin"],
  ["monitor_effectiveness", "use-effectiveness", "use"],
] satisfies ReadonlyArray<
  readonly [TwinActivationActionKey, TwinGuideDestination, TwinWorkspaceTab]
>;

const inspectionCases = [
  ["evidence_history", "wiki-evidence", "wiki"],
  ["twin_history", "twin-history", "twin"],
  ["execution_evidence", "use-effectiveness", "use"],
] satisfies ReadonlyArray<
  readonly [
    TwinActivationInspectionLink["key"],
    TwinGuideDestination,
    TwinWorkspaceTab,
  ]
>;

describe("Twin guided destination map", () => {
  it.each(actionCases)(
    "maps action %s to %s on %s",
    (key, destination, tab) => {
      const request: TwinGuideRequest = { kind: "action", key };
      expect(resolveTwinGuide(request)).toMatchObject({ destination, tab });
    },
  );

  it.each(inspectionCases)(
    "maps inspection %s to %s on %s",
    (key, destination, tab) => {
      const request: TwinGuideRequest = { kind: "inspection", key };
      expect(resolveTwinGuide(request)).toMatchObject({ destination, tab });
    },
  );
});
