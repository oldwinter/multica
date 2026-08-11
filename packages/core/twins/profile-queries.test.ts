import { afterEach, describe, expect, it, vi } from "vitest";
import { ApiClient } from "../api/client";
import { twinProfileKeys, twinProfileOverviewOptions } from "./queries";

afterEach(() => {
  vi.unstubAllGlobals();
});

function overviewPayload(overrides: Record<string, unknown> = {}) {
  return {
    twin: {
      id: "twin-1",
      name: "Product partner",
      state: "pending-signoff",
      reviewDigest: "sha256:review",
      updatedAt: "2026-08-03T00:00:00Z",
      sourceCount: 2,
      assertionCount: 1,
      skillCount: 0,
      ruleCount: 1,
      assertions: [
        {
          id: "assertion-1",
          text: "Keep the change reviewable.",
          sourceCount: 1,
          sourceRefs: ["design-note"],
          reviewed: true,
        },
      ],
      topics: [],
      reviewSteps: [],
      ...overrides,
    },
  };
}

describe("Twin API boundary", () => {
  it("parses a valid workspace overview", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(
        new Response(JSON.stringify(overviewPayload()), {
          status: 200,
          headers: { "Content-Type": "application/json" },
        }),
      ),
    );

    const result = await new ApiClient("http://localhost:8080").getTwinOverview();

    expect(result.twin).toMatchObject({
      id: "twin-1",
      name: "Product partner",
      assertionCount: 1,
    });
    expect(result.twin?.assertions[0]?.sourceRefs).toEqual(["design-note"]);
  });

  it("parses a null twin overview", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(
        new Response(JSON.stringify({ twin: null }), {
          status: 200,
          headers: { "Content-Type": "application/json" },
        }),
      ),
    );

    await expect(new ApiClient("http://localhost:8080").getTwinOverview()).resolves.toEqual({
      twin: null,
    });
  });

  it("defaults nested assertion fields when the API omits them", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(
        new Response(
          JSON.stringify({
            twin: {
              id: "twin-2",
              name: "Sparse twin",
              state: "signed-off",
              reviewDigest: "sha256:sparse",
              updatedAt: "2026-08-04T00:00:00Z",
              assertions: [{ id: "a1", text: "Only required fields" }],
            },
          }),
          { status: 200, headers: { "Content-Type": "application/json" } },
        ),
      ),
    );

    const result = await new ApiClient("http://localhost:8080").getTwinOverview();

    expect(result.twin).toMatchObject({
      id: "twin-2",
      sourceCount: 0,
      assertionCount: 0,
      skillCount: 0,
      ruleCount: 0,
    });
    expect(result.twin?.assertions).toEqual([
      {
        id: "a1",
        text: "Only required fields",
        sourceCount: 0,
        sourceRefs: [],
        reviewed: false,
      },
    ]);
    expect(result.twin?.topics).toEqual([]);
    expect(result.twin?.reviewSteps).toEqual([]);
  });

  it("falls back safely when the nested overview is malformed", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(
        new Response(JSON.stringify({ twin: { id: "missing-required-fields" } }), {
          status: 200,
          headers: { "Content-Type": "application/json" },
        }),
      ),
    );

    await expect(new ApiClient("http://localhost:8080").getTwinOverview()).resolves.toEqual({
      twin: null,
    });
  });

  it("falls back safely when the top-level envelope is not an object", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(
        new Response(JSON.stringify(["not", "an", "envelope"]), {
          status: 200,
          headers: { "Content-Type": "application/json" },
        }),
      ),
    );

    await expect(new ApiClient("http://localhost:8080").getTwinOverview()).resolves.toEqual({
      twin: null,
    });
  });

  it("keeps the workspace id in the query key", () => {
    expect(twinProfileKeys.all("ws-1")).toEqual(["workspaces", "ws-1", "twin-profile"]);
    expect(twinProfileKeys.overview("ws-1")).toEqual(["workspaces", "ws-1", "twin-profile", "overview"]);
    expect(twinProfileKeys.overview("ws-2")).not.toEqual(twinProfileKeys.overview("ws-1"));
  });
});

describe("twinProfileOverviewOptions", () => {
  it("disables the query when workspace id is empty and enables it otherwise", () => {
    expect(twinProfileOverviewOptions("").enabled).toBe(false);
    expect(twinProfileOverviewOptions("ws-1").enabled).toBe(true);
    expect(twinProfileOverviewOptions("ws-1").queryKey).toEqual(["workspaces", "ws-1", "twin-profile", "overview"]);
  });

  it("selects the nested twin from the response envelope", () => {
    const options = twinProfileOverviewOptions("ws-1");
    const select = options.select;
    expect(select).toBeTypeOf("function");
    if (!select) throw new Error("profile overview select is missing");
    expect(select({ twin: null })).toBeNull();
    expect(
      select({
        twin: {
          id: "twin-1",
          name: "Selected",
          state: "pending-signoff",
          reviewDigest: "sha256:x",
          updatedAt: "2026-08-03T00:00:00Z",
          sourceCount: 0,
          assertionCount: 0,
          skillCount: 0,
          ruleCount: 0,
          assertions: [],
          topics: [],
          reviewSteps: [],
        },
      })?.name,
    ).toBe("Selected");
  });
});
