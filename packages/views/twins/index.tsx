"use client";

import { previewTwinOverview } from "@multica/core/twins";
import { useWorkspacePaths } from "@multica/core/paths";
import { useT } from "../i18n";
import {
  TwinWorkspaceView,
  type TwinCopy,
  type TwinViewState,
} from "./components/twin-workspace-view";

export type { TwinCopy, TwinViewState } from "./components/twin-workspace-view";
export { TwinWorkspaceView } from "./components/twin-workspace-view";

export function TwinsPage({ state = "ready" }: { state?: TwinViewState }) {
  const { t } = useT("twins");
  const paths = useWorkspacePaths();
  const copy: TwinCopy = {
    eyebrow: t(($) => $.page.eyebrow),
    title: t(($) => $.page.title),
    description: t(($) => $.page.description),
    previewBadge: t(($) => $.page.preview_badge),
    actions: {
      openIssues: t(($) => $.actions.open_issues),
      openAgents: t(($) => $.actions.open_agents),
      reviewProfile: t(($) => $.actions.review_profile),
      tryAgain: t(($) => $.actions.try_again),
      connectEvidence: t(($) => $.actions.connect_evidence),
    },
    status: {
      pending: {
        label: t(($) => $.status.pending.label),
        title: t(($) => $.status.pending.title),
        description: t(($) => $.status.pending.description),
      },
      signedOff: {
        label: t(($) => $.status.signed_off.label),
        title: t(($) => $.status.signed_off.title),
        description: t(($) => $.status.signed_off.description),
      },
      invalid: {
        label: t(($) => $.status.invalid.label),
        title: t(($) => $.status.invalid.title),
        description: t(($) => $.status.invalid.description),
      },
    },
    summary: {
      sources: t(($) => $.summary.sources),
      assertions: t(($) => $.summary.assertions),
      skills: t(($) => $.summary.skills),
      rules: t(($) => $.summary.rules),
      lastReviewed: t(($) => $.summary.last_reviewed),
    },
    review: {
      title: t(($) => $.review.title),
      description: t(($) => $.review.description),
      steps: {
        import: {
          label: t(($) => $.review.steps.import.label),
          description: t(($) => $.review.steps.import.description),
        },
        generate: {
          label: t(($) => $.review.steps.generate.label),
          description: t(($) => $.review.steps.generate.description),
        },
        topic: {
          label: t(($) => $.review.steps.topic.label),
          description: t(($) => $.review.steps.topic.description),
        },
        coordinate: {
          label: t(($) => $.review.steps.coordinate.label),
          description: t(($) => $.review.steps.coordinate.description),
        },
        accept: {
          label: t(($) => $.review.steps.accept.label),
          description: t(($) => $.review.steps.accept.description),
        },
        deposition: {
          label: t(($) => $.review.steps.deposition.label),
          description: t(($) => $.review.steps.deposition.description),
        },
      },
    },
    tabs: {
      overview: t(($) => $.tabs.overview),
      evidence: t(($) => $.tabs.evidence),
      topics: t(($) => $.tabs.topics),
    },
    evidence: {
      title: t(($) => $.evidence.title),
      description: t(($) => $.evidence.description),
      sourceCount: t(($) => $.evidence.source_count),
      viewDetail: t(($) => $.evidence.view_detail),
    },
    topics: {
      title: t(($) => $.topics.title),
      description: t(($) => $.topics.description),
      openIssue: t(($) => $.topics.open_issue),
      empty: t(($) => $.topics.empty),
    },
    states: {
      loading: t(($) => $.states.loading),
      emptyTitle: t(($) => $.states.empty_title),
      emptyDescription: t(($) => $.states.empty_description),
      errorTitle: t(($) => $.states.error_title),
      errorDescription: t(($) => $.states.error_description),
    },
    stateLabels: {
      complete: t(($) => $.state_labels.complete),
      current: t(($) => $.state_labels.current),
      upcoming: t(($) => $.state_labels.upcoming),
    },
    topicStates: {
      active: t(($) => $.topic_states.active),
      waiting: t(($) => $.topic_states.waiting),
      accepted: t(($) => $.topic_states.accepted),
    },
  };

  return (
    <TwinWorkspaceView
      state={state}
      data={state === "ready" ? previewTwinOverview : undefined}
      copy={copy}
      links={{ issues: paths.issues(), agents: paths.agents() }}
      onRetry={() => globalThis.location.reload()}
    />
  );
}
