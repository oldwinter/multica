// @vitest-environment node

import { describe, expect, it } from "vitest";
import { OfficePage } from "@multica/views/office";
import Page from "./page";

describe("Web Office route", () => {
  it("registers the shared Office page directly", () => {
    expect(Page).toBe(OfficePage);
  });
});
