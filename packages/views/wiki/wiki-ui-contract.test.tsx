// @vitest-environment jsdom

import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { WikiMasterDetail } from "./wiki-ui-contract";

describe("WikiMasterDetail", () => {
  it("removes the unused narrow detail track only for a collection echo", () => {
    const { rerender } = render(
      <WikiMasterDetail
        detailRole="collection-echo"
        collection="Collection"
        detail="Detail"
      />,
    );

    expect(screen.getByTestId("wiki-master-detail")).toHaveClass(
      "grid-rows-[minmax(10rem,1fr)]",
    );
    expect(screen.getByTestId("wiki-detail-pane")).toHaveClass("max-lg:hidden");

    rerender(
      <WikiMasterDetail
        detailRole="required"
        collection="Collection"
        detail="Detail"
      />,
    );

    expect(screen.getByTestId("wiki-master-detail")).toHaveClass(
      "grid-rows-[minmax(10rem,35dvh)_minmax(20rem,1fr)]",
    );
    expect(screen.getByTestId("wiki-detail-pane")).not.toHaveClass("max-lg:hidden");
  });
});
