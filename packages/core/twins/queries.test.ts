import { describe, expect, it } from "vitest";
import { twinKeys, twinOverviewOptions, twinProposalOptions, twinVersionOptions, wikiKeys, wikiOverviewOptions, wikiRevisionOptions } from "./queries";

describe("Wiki and Twin query keys", () => {
  it("keeps two workspace scopes isolated", () => {
    expect(wikiKeys.overview("workspace-a")).not.toEqual(
      wikiKeys.overview("workspace-b"),
    );
    expect(wikiKeys.revision("workspace-a", "revision-1")).not.toEqual(
      wikiKeys.revision("workspace-b", "revision-1"),
    );
    expect(twinKeys.overview("workspace-a")).not.toEqual(
      twinKeys.overview("workspace-b"),
    );
    expect(twinKeys.proposal("workspace-a", "proposal-1")).not.toEqual(
      twinKeys.proposal("workspace-b", "proposal-1"),
    );
  });

  it("builds options from explicit workspace ids", () => {
    expect(wikiOverviewOptions("workspace-a").queryKey).toEqual(
      wikiKeys.overview("workspace-a"),
    );
    expect(wikiRevisionOptions("workspace-a", "revision-1").queryKey).toEqual(
      wikiKeys.revision("workspace-a", "revision-1"),
    );
    expect(twinOverviewOptions("workspace-a").queryKey).toEqual(
      twinKeys.overview("workspace-a"),
    );
    expect(twinProposalOptions("workspace-a", "proposal-1").queryKey).toEqual(
      twinKeys.proposal("workspace-a", "proposal-1"),
    );
    expect(twinVersionOptions("workspace-a", "version-1").queryKey).toEqual(
      twinKeys.version("workspace-a", "version-1"),
    );
  });
});
