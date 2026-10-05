// @vitest-environment jsdom

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
  act,
  fireEvent,
  screen,
  within,
} from "@testing-library/react";
import { ApiError } from "@multica/core/api";
import { toast } from "sonner";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { renderWithI18n } from "../test/i18n";
import { NavigationProvider, type NavigationAdapter } from "../navigation";
import { SkillEvolutionPage } from "./skill-evolution-page";

const state = vi.hoisted(() => ({
  overview: null as Record<string, unknown> | null,
  overviewError: null as Error | null,
  proposal: null as Record<string, unknown> | null,
  proposalError: null as Error | null,
  overviewCalls: 0,
}));

const mutations = vi.hoisted(() => ({
  configure: vi.fn(),
  pause: vi.fn(),
  request: vi.fn(),
  reject: vi.fn(),
  publish: vi.fn(),
  rollback: vi.fn(),
  fork: vi.fn(),
}));

vi.mock("sonner", () => ({
  toast: { success: vi.fn(), error: vi.fn() },
}));

vi.mock("@multica/core/skill-evolution", () => {
  const mutation = (mutate: ReturnType<typeof vi.fn>) => ({
    mutate,
    isPending: false,
    error: null,
  });
  return {
    skillEvolutionOverviewOptions: (workspaceId: string, skillId: string) => ({
      queryKey: ["skill-evolution", workspaceId, skillId],
      queryFn: async () => {
        state.overviewCalls += 1;
        if (state.overviewError) throw state.overviewError;
        return state.overview;
      },
      retry: false,
    }),
    skillEvolutionProposalOptions: (workspaceId: string, proposalId: string) => ({
      queryKey: ["skill-evolution", workspaceId, "proposal", proposalId],
      queryFn: async () => {
        if (state.proposalError) throw state.proposalError;
        return state.proposal;
      },
      enabled: Boolean(workspaceId && proposalId),
      retry: false,
    }),
    useConfigureSkillEvolution: () => mutation(mutations.configure),
    usePauseSkillEvolution: () => mutation(mutations.pause),
    useRequestSkillEvolutionProposal: () => mutation(mutations.request),
    useRejectSkillEvolutionProposal: () => mutation(mutations.reject),
    usePublishSkillEvolutionProposal: () => mutation(mutations.publish),
    useRollbackSkillEvolutionRelease: () => mutation(mutations.rollback),
    useForkSkillForEvolution: () => mutation(mutations.fork),
  };
});

const LOOP = {
  id: "loop-1",
  enabled: true,
  mode: "propose",
  cooldownSeconds: 3600,
  minimumSignals: 2,
  maxEvidenceRefs: 20,
  maxReplaySamples: 8,
  maxCostUsdTicks: 10000,
  policyVersion: "v1",
  lastObservedAt: "2026-08-28T12:00:00Z",
  lastProposalAt: "2026-08-28T12:15:00Z",
  nextEligibleAt: "2026-08-28T13:15:00Z",
  updatedAt: "2026-08-28T12:15:00Z",
};

const READY_PROPOSAL = {
  id: "proposal-1",
  skillId: "skill-1",
  state: "ready",
  baseRevisionId: "revision-base",
  baseHash: "base-hash-000000000000",
  candidateRevisionId: "revision-candidate",
  candidateHash: "candidate-hash-00000000",
  failureReason: null,
  staleReason: null,
  createdAt: "2026-08-28T12:00:00Z",
  updatedAt: "2026-08-28T12:05:00Z",
};

const RELEASE = {
  id: "release-1",
  skillId: "skill-1",
  proposalId: "proposal-0",
  sourceReleaseId: null,
  revisionId: "revision-base",
  kind: "publish",
  expectedBaseHash: "older-hash",
  preHash: "older-hash",
  postHash: "base-hash-000000000000",
  outcome: "succeeded",
  errorCode: null,
  createdAt: "2026-08-27T12:00:00Z",
  completedAt: "2026-08-27T12:01:00Z",
};

function overview(overrides: Record<string, unknown> = {}) {
  return {
    skill: {
      id: "skill-1",
      name: "release-notes",
      bundleHash: "base-hash-000000000000",
      ownership: "workspace",
      ownershipReason: "workspace_created",
      forkRequired: false,
    },
    loop: LOOP,
    revisions: [
      {
        id: "revision-base",
        kind: "base",
        bundleHash: "base-hash-000000000000",
        byteCount: 1200,
        supportFileCount: 2,
        createdAt: "2026-08-27T12:00:00Z",
      },
    ],
    proposals: [READY_PROPOSAL],
    releases: [RELEASE],
    permissions: { canConfigure: true, canPublish: true, canFork: true },
    ...overrides,
  };
}

function proposalDetail(overrides: Record<string, unknown> = {}) {
  return {
    proposal: READY_PROPOSAL,
    rationale: {
      observedPattern: "Repeated omissions in release summaries",
      expectedBenefit: "More complete release notes",
      regressionRisk: "Longer primary instructions",
    },
    diff: {
      truncated: false,
      omittedRows: 0,
      metadata: [
        { field: "description", before: "Draft release notes", after: "Draft cited release notes" },
      ],
      files: [
        {
          path: "SKILL.md",
          change: "modified",
          truncated: false,
          omittedRows: 0,
          rows: [
            { kind: "context", oldLine: 1, newLine: 1, text: "# Release notes" },
            { kind: "add", newLine: 2, text: "Cite each merged change." },
          ],
        },
      ],
    },
    evidence: [
      {
        kind: "task_review",
        sourceId: "review-1",
        sourceRevisionId: "task-1",
        sourceState: "accepted",
        digest: "evidence-digest-0001",
        observedAt: "2026-08-28T11:00:00Z",
      },
    ],
    evaluations: [
      {
        id: "evaluation-1",
        kind: "replay",
        result: "passed",
        adapter: "room-replay",
        adapterVersion: "v1",
        policyVersion: "v1",
        resultDigest: "result-digest-0001",
        safeMetrics: { passed: 8, failed: 0 },
        costUsdTicks: 420,
        durationMs: 1800,
        createdAt: "2026-08-28T12:04:00Z",
      },
    ],
    reviews: [
      {
        id: "review-decision-1",
        decision: "publish",
        actorId: "user-owner-1",
        reason: "Evidence is sufficient",
        createdAt: "2026-08-28T12:06:00Z",
      },
    ],
    ...overrides,
  };
}

const navigation: NavigationAdapter = {
  push: vi.fn(),
  replace: vi.fn(),
  back: vi.fn(),
  pathname: "/acme/skills/skill-1/evolution",
  searchParams: new URLSearchParams(),
  hash: "",
  getShareableUrl: (path) => `https://multica.test${path}`,
};

function renderPage() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: Infinity } },
  });
  return renderWithI18n(
    <QueryClientProvider client={queryClient}>
      <NavigationProvider value={navigation}>
        <SkillEvolutionPage workspaceId="workspace-1" workspaceSlug="acme" skillId="skill-1" />
      </NavigationProvider>
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  state.overview = overview();
  state.overviewError = null;
  state.proposal = proposalDetail();
  state.proposalError = null;
  state.overviewCalls = 0;
  vi.clearAllMocks();
});

afterEach(() => {
  vi.useRealTimers();
});

describe("SkillEvolutionPage", () => {
  it("renders a null loop and requires a fork for externally owned Skills", async () => {
    state.overview = overview({
      skill: {
        id: "skill-1",
        name: "vendor-skill",
        bundleHash: "external-hash",
        ownership: "plugin",
        ownershipReason: "plugin_installation",
        forkRequired: true,
      },
      loop: null,
      proposals: [],
      releases: [],
      revisions: [],
    });

    renderPage();

    expect(await screen.findByText("Evolution is off")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Fork Skill" })).toBeEnabled();
    expect(screen.getByRole("switch", { name: "Enabled" })).toHaveAttribute("aria-disabled", "true");
    expect(screen.queryByRole("button", { name: "Request proposal" })).not.toBeInTheDocument();
  });

  it("renders rationale, structured diff, evidence provenance, metrics, and reviews", async () => {
    renderPage();

    expect(await screen.findByText("Repeated omissions in release summaries")).toBeInTheDocument();
    expect(screen.getByText("Cite each merged change.").closest("[data-diff-kind]"))
      .toHaveAttribute("data-diff-kind", "add");
    expect(screen.getByText("review-1")).toBeInTheDocument();
    expect(screen.getByText("evidence-digest-0001")).toBeInTheDocument();
    expect(screen.getByText("room-replay v1")).toBeInTheDocument();
    expect(screen.getByText(/Evidence is sufficient/)).toBeInTheDocument();
  });

  it("marks a truncated diff as incomplete", async () => {
    state.proposal = proposalDetail({
      diff: {
        truncated: true,
        omittedRows: 37,
        metadata: [],
        files: [
          {
            path: "SKILL.md",
            change: "modified",
            truncated: true,
            omittedRows: 12,
            rows: [{ kind: "add", newLine: 1, text: "bounded row" }],
          },
        ],
      },
    });
    renderPage();

    expect(await screen.findByText("Diff truncated; 37 rows omitted")).toBeInTheDocument();
    expect(screen.getByText("Diff truncated; 12 rows omitted")).toBeInTheDocument();
  });

  it("requires confirmation for reject, publish, and rollback mutations", async () => {
    mutations.reject.mockImplementation((_input, options) => options.onSuccess());
    mutations.publish.mockImplementation((_input, options) => options.onSuccess());
    mutations.rollback.mockImplementation((_input, options) => options.onSuccess());
    renderPage();
    await screen.findByText("Repeated omissions in release summaries");

    fireEvent.click(screen.getByRole("button", { name: "Reject" }));
    const rejectDialog = await screen.findByRole("dialog", { name: "Reject proposal" });
    fireEvent.change(within(rejectDialog).getByLabelText("Reason"), {
      target: { value: "Insufficient replay coverage" },
    });
    fireEvent.click(within(rejectDialog).getByRole("button", { name: "Reject" }));
    expect(mutations.reject).toHaveBeenCalledWith(
      expect.objectContaining({
        reason: "Insufficient replay coverage",
        idempotencyKey: expect.stringContaining("skill-evolution-reject-proposal-1-"),
      }),
      expect.any(Object),
    );

    fireEvent.click(screen.getByRole("button", { name: "Publish" }));
    const publishDialog = await screen.findByRole("alertdialog", { name: "Publish this proposal?" });
    fireEvent.click(within(publishDialog).getByRole("button", { name: "Publish" }));
    expect(mutations.publish).toHaveBeenCalledWith(
      expect.objectContaining({ idempotencyKey: expect.stringContaining("skill-evolution-publish-proposal-1-") }),
      expect.any(Object),
    );

    fireEvent.click(screen.getByRole("button", { name: "Roll back" }));
    const rollbackDialog = await screen.findByRole("alertdialog", { name: "Roll back to this release?" });
    fireEvent.click(within(rollbackDialog).getByRole("button", { name: "Roll back" }));
    expect(mutations.rollback).toHaveBeenCalledWith(
      expect.objectContaining({
        releaseId: "release-1",
        idempotencyKey: expect.stringContaining("skill-evolution-rollback-release-1-"),
      }),
      expect.any(Object),
    );
  }, 15_000);

  it("does not report an unknown publication outcome as success", async () => {
    mutations.publish.mockImplementation((_input, options) => options.onSuccess({
      proposal: { ...READY_PROPOSAL, state: "publication_unknown" },
      release: { ...RELEASE, outcome: "publication_unknown" },
    }));
    renderPage();
    await screen.findByText("Repeated omissions in release summaries");

    fireEvent.click(screen.getByRole("button", { name: "Publish" }));
    const dialog = await screen.findByRole("alertdialog", { name: "Publish this proposal?" });
    fireEvent.click(within(dialog).getByRole("button", { name: "Publish" }));

    expect(toast.error).toHaveBeenCalledWith(
      "Publication outcome is unknown; inspect the release history.",
    );
    expect(toast.success).not.toHaveBeenCalledWith("Proposal published");
  });

  it("keeps proposal actions visible but disabled without human permissions", async () => {
    state.overview = overview({
      permissions: { canConfigure: false, canPublish: false, canFork: false },
    });
    renderPage();

    await screen.findByText("Repeated omissions in release summaries");
    expect(screen.getByText("Read only")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Reject" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "Publish" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "Roll back" })).toBeDisabled();
  });

  it.each([
    ["disabled", { ...LOOP, enabled: false, mode: "propose" }],
    ["observe-only", { ...LOOP, enabled: true, mode: "observe" }],
  ] as const)("disables proposal requests for %s loops", async (_name, loop) => {
    state.overview = overview({ loop, proposals: [] });
    state.proposal = null;
    renderPage();

    const request = await screen.findByRole("button", { name: "Request proposal" });
    expect(request).toBeDisabled();
    expect(mutations.request).not.toHaveBeenCalled();
  });

  it("keeps the replay sample control aligned with the server minimum", async () => {
    state.overview = overview({ loop: null, proposals: [] });
    state.proposal = null;
    renderPage();

    const replay = await screen.findByLabelText("Replay samples");
    expect(replay).toHaveAttribute("min", "1");
  });

  it("blocks saving a replay sample count below the server minimum", async () => {
    state.overview = overview({ loop: null, proposals: [] });
    state.proposal = null;
    renderPage();

    fireEvent.click(await screen.findByRole("switch", { name: "Enabled" }));
    fireEvent.change(screen.getByLabelText("Replay samples"), { target: { value: "0" } });
    expect(screen.getByRole("button", { name: "Save configuration" })).toBeDisabled();
    expect(mutations.configure).not.toHaveBeenCalled();
  });

  it("renders a dedicated forbidden state for a non-member actor", async () => {
    state.overviewError = new ApiError("forbidden", 403, "Forbidden");
    renderPage();

    expect(await screen.findByText("Evolution access restricted")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Retry" })).not.toBeInTheDocument();
  });

  it("renders a retryable unavailable state when the Skill is missing", async () => {
    state.overviewError = new ApiError("not_found", 404, "Not found");
    renderPage();

    expect(await screen.findByText("Evolution state unavailable")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Retry" })).toBeEnabled();
  });

  it("fails future proposal and loop enum values to an explicit unknown state", async () => {
    state.overview = overview({
      loop: { ...LOOP, mode: "future-mode" },
      proposals: [{ ...READY_PROPOSAL, state: "unknown" }],
    });
    state.proposal = proposalDetail({
      proposal: { ...READY_PROPOSAL, state: "unknown" },
    });
    renderPage();

    expect(await screen.findByText("This proposal uses a state this client does not recognize.")).toBeInTheDocument();
    expect(screen.getAllByText("unknown").length).toBeGreaterThanOrEqual(2);
    expect(screen.queryByRole("button", { name: "Publish" })).not.toBeInTheDocument();
  });

  it("polls for at most two minutes after an Improvement Room is queued", async () => {
    state.overview = overview({ proposals: [] });
    state.proposal = null;
    mutations.request.mockImplementation((_input, options) => {
      options.onSuccess({
        state: "improvement_room_queued",
        roomId: "room-1",
        proposal: null,
      });
    });
    renderPage();
    const requestButton = await screen.findByRole("button", { name: "Request proposal" });
    vi.useFakeTimers({ shouldAdvanceTime: true });

    fireEvent.click(requestButton);
    expect(screen.getByText("Improvement Room queued for this Skill")).toBeInTheDocument();
    expect(screen.getByRole("status")).toHaveTextContent("Checking for updates");
    const callsAfterRequest = state.overviewCalls;

    await act(async () => {
      await vi.advanceTimersByTimeAsync(3100);
    });
    expect(state.overviewCalls).toBeGreaterThan(callsAfterRequest);

    await act(async () => {
      await vi.advanceTimersByTimeAsync(120000);
    });
    expect(screen.queryByText("Improvement Room queued for this Skill")).not.toBeInTheDocument();
  }, 15_000);

  it("saves an enabled loop configuration and pauses with stable intent keys", async () => {
    state.overview = overview({ loop: null, proposals: [] });
    const first = renderPage();
    const enabled = await screen.findByRole("switch", { name: "Enabled" });
    fireEvent.click(enabled);
    fireEvent.click(screen.getByRole("button", { name: "Save configuration" }));
    expect(mutations.configure).toHaveBeenCalledWith(
      expect.objectContaining({ enabled: true, mode: "observe", policyVersion: "v1" }),
      expect.any(Object),
    );

    first.unmount();
    state.overview = overview();
    renderPage();
    const pauseButton = await screen.findByRole("button", { name: "Pause now" });
    fireEvent.click(pauseButton);
    expect(mutations.pause).toHaveBeenCalledWith(
      expect.objectContaining({ idempotencyKey: expect.stringContaining("skill-evolution-pause-skill-1-") }),
      expect.any(Object),
    );
  });
});
