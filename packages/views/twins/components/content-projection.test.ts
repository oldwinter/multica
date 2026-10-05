// @vitest-environment node

import { describe, expect, it } from "vitest";
import {
  projectTwinAssertions,
  projectTwinDiff,
} from "./content-projection";

describe("Twin content projection", () => {
  it("projects schema v2 assertion evidence and structured applicability without parsing text", () => {
    const [assertion] = projectTwinAssertions({
      schema_version: 2,
      assertions: [{
        id: "assertion-1",
        type: "quality_bar",
        text: "长文本约束用于验证窄屏下仍保留完整语义而不是截断审计内容。",
        applicability: {
          workspace_id: "workspace-1",
          project_id: "project-1",
          keywords: ["security", "登录流程"],
        },
        evidence_citations: ["wiki:security", "issue:42"],
        confidence: 0.91,
        provenance: { kind: "model", generator: "twin-generator/2" },
      }],
    });

    expect(assertion).toMatchObject({
      id: "assertion-1",
      kind: "quality_bar",
      citationKeys: ["wiki:security", "issue:42"],
      confidence: 0.91,
      applicability: {
        workspaceId: "workspace-1",
        projectId: "project-1",
        keywords: ["security", "登录流程"],
        legacyText: "",
      },
      provenance: { kind: "model", generator: "twin-generator/2" },
    });
  });

  it("keeps legacy v1 applicability readable and projects changed diffs", () => {
    const [assertion] = projectTwinAssertions({
      schema_version: 1,
      assertions: [{
        id: "legacy",
        text: "Prefer explicit review decisions.",
        applicability: "security-related tasks only",
        citation_keys: ["issue:1"],
      }],
    });

    expect(assertion?.applicability).toMatchObject({
      legacyText: "security-related tasks only",
      keywords: [],
    });
    expect(assertion?.citationKeys).toEqual(["issue:1"]);
    expect(projectTwinDiff({
      diff: {
        added: ["new"],
        changed: ["revised"],
        removed: ["old"],
        unchanged: ["stable"],
      },
    })).toEqual({
      added: ["new"],
      changed: ["revised"],
      removed: ["old"],
      unchanged: ["stable"],
    });
  });
});
