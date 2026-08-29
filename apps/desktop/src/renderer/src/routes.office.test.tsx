import { isValidElement } from "react";
import { describe, expect, it } from "vitest";
import { OfficePage } from "@multica/views/office";
import { appRoutes } from "./routes";

describe("Desktop Office route", () => {
  it("registers Office in the existing workspace route tree", () => {
    const workspaceRoute = appRoutes[0]?.children?.find(
      (route) => route.path === ":workspaceSlug",
    );
    const officeRoute = workspaceRoute?.children?.find(
      (route) => route.path === "office",
    );

    expect(officeRoute?.handle).toEqual({ title: "Office" });
    expect(isValidElement(officeRoute?.element)).toBe(true);
    if (!isValidElement(officeRoute?.element)) return;
    expect(officeRoute.element.type).toBe(OfficePage);
  });
});
