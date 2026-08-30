import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { NavigationProvider } from "../../navigation";
import type { NavigationAdapter } from "../../navigation";

const detailProps = vi.hoisted<{ highlightCommentId: string | undefined }>(
  () => ({ highlightCommentId: undefined }),
);

vi.mock("@multica/core/hooks", () => ({
  useWorkspaceId: () => "ws-1",
}));

vi.mock("@multica/core/issues/canonical-id", () => ({
  useCanonicalIssue: () => ({
    canonicalId: "issue-1",
    issue: { identifier: "MUL-1" },
    isResolving: false,
    notFound: false,
  }),
}));

vi.mock("@multica/core/paths", async () => {
  const actual = await vi.importActual<typeof import("@multica/core/paths")>(
    "@multica/core/paths",
  );
  return {
    ...actual,
    useWorkspacePaths: () => actual.paths.workspace("acme"),
  };
});

vi.mock("./issue-detail", () => ({
  IssueDetail: ({
    highlightCommentId,
  }: {
    highlightCommentId?: string;
  }) => {
    detailProps.highlightCommentId = highlightCommentId;
    return <div>Issue detail</div>;
  },
  IssueDetailSkeleton: () => <div>Loading</div>,
  IssueNotFound: () => <div>Not found</div>,
}));

import { IssueDetailRoute } from "./issue-detail-route";

describe("IssueDetailRoute comment permalink", () => {
  it("passes the comment query target to the issue timeline", () => {
    const adapter: NavigationAdapter = {
      push: vi.fn(),
      replace: vi.fn(),
      back: vi.fn(),
      pathname: "/acme/issues/MUL-1",
      searchParams: new URLSearchParams("comment=reply-7"),
      hash: "",
      getShareableUrl: (path: string) => `https://app.example${path}`,
    };

    render(
      <NavigationProvider value={adapter}>
        <IssueDetailRoute routeId="MUL-1" />
      </NavigationProvider>,
    );

    expect(screen.getByText("Issue detail")).toBeInTheDocument();
    expect(detailProps.highlightCommentId).toBe("reply-7");
  });
});
