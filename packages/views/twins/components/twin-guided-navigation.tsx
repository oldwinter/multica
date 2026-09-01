"use client";

import {
  useCallback,
  useEffect,
  useRef,
  useState,
  type ComponentProps,
  type RefObject,
} from "react";
import type {
  TwinActivationActionKey,
  TwinActivationInspectionLink,
} from "@multica/core/twins";
import { cn } from "@multica/ui/lib/utils";
import type { TwinWorkspaceTab } from "./twin-workspace-tabs";

export type TwinGuideDestination =
  | "wiki-source-policy"
  | "wiki-overview"
  | "wiki-evidence"
  | "twin-overview"
  | "twin-history"
  | "use-status"
  | "use-binding"
  | "use-preview"
  | "use-effectiveness";

export type TwinGuideRequest =
  | { readonly kind: "action"; readonly key: TwinActivationActionKey }
  | {
      readonly kind: "inspection";
      readonly key: TwinActivationInspectionLink["key"];
    };

export interface TwinGuidePlan {
  readonly destination: TwinGuideDestination;
  readonly tab: TwinWorkspaceTab;
  readonly fallback?: TwinGuideDestination;
}

interface TwinGuideDestinationSpec {
  readonly tab: TwinWorkspaceTab;
  readonly fallback: TwinGuideDestination | null;
}

const destinationSpecs = {
  "wiki-source-policy": { tab: "wiki", fallback: null },
  "wiki-overview": { tab: "wiki", fallback: null },
  "wiki-evidence": { tab: "wiki", fallback: null },
  "twin-overview": { tab: "twin", fallback: null },
  "twin-history": { tab: "twin", fallback: null },
  "use-status": { tab: "use", fallback: null },
  "use-binding": { tab: "use", fallback: "use-status" },
  "use-preview": { tab: "use", fallback: "use-status" },
  "use-effectiveness": { tab: "use", fallback: "use-status" },
} as const satisfies Record<TwinGuideDestination, TwinGuideDestinationSpec>;

const actionDestinations = {
  inspect_disabled: "use-status",
  configure_source: "wiki-source-policy",
  review_evidence: "wiki-evidence",
  refresh_evidence: "wiki-overview",
  review_twin: "twin-history",
  generate_twin: "twin-overview",
  compile_preview: "use-preview",
  configure_binding: "use-binding",
  run_with_twin: "use-preview",
  review_run: "use-effectiveness",
  review_deposition: "twin-history",
  monitor_effectiveness: "use-effectiveness",
} as const satisfies Record<TwinActivationActionKey, TwinGuideDestination>;

const inspectionDestinations = {
  evidence_history: "wiki-evidence",
  twin_history: "twin-history",
  execution_evidence: "use-effectiveness",
} as const satisfies Record<
  TwinActivationInspectionLink["key"],
  TwinGuideDestination
>;

export function resolveTwinGuide(request: TwinGuideRequest): TwinGuidePlan {
  const destination = request.kind === "action"
    ? actionDestinations[request.key]
    : inspectionDestinations[request.key];
  const spec = destinationSpecs[destination];
  return {
    destination,
    tab: spec.tab,
    ...(spec.fallback ? { fallback: spec.fallback } : {}),
  };
}

type TwinDestinationProps = Omit<ComponentProps<"section">, "tabIndex"> & {
  readonly destination: TwinGuideDestination;
};

export function TwinDestination({
  destination,
  className,
  ...props
}: TwinDestinationProps) {
  return (
    <section
      {...props}
      data-twin-destination={destination}
      tabIndex={-1}
      className={cn(
        "scroll-mt-4 outline-none focus-visible:ring-2 focus-visible:ring-ring/70",
        className,
      )}
    />
  );
}

interface PendingGuide {
  readonly id: number;
  readonly plan: TwinGuidePlan;
}

interface TwinGuidedNavigation {
  readonly guide: (request: TwinGuideRequest) => void;
  readonly selectTab: (tab: TwinWorkspaceTab) => void;
}

export function useTwinGuidedNavigation({
  activeTab,
  rootRef,
  commitTab,
}: {
  readonly activeTab: TwinWorkspaceTab;
  readonly rootRef: RefObject<HTMLElement | null>;
  readonly commitTab: (tab: TwinWorkspaceTab) => void;
}): TwinGuidedNavigation {
  const guideSequence = useRef(0);
  const [pendingGuide, setPendingGuide] = useState<PendingGuide | null>(null);

  const selectTab = useCallback((tab: TwinWorkspaceTab) => {
    setPendingGuide(null);
    commitTab(tab);
  }, [commitTab]);

  const guide = useCallback((request: TwinGuideRequest) => {
    const plan = resolveTwinGuide(request);
    const id = ++guideSequence.current;
    setPendingGuide({ id, plan });
    if (plan.tab !== activeTab) commitTab(plan.tab);
  }, [activeTab, commitTab]);

  useEffect(() => {
    if (!pendingGuide || pendingGuide.plan.tab !== activeTab) return;
    const root = rootRef.current;
    if (!root) return;

    let observer: MutationObserver | null = null;
    const settle = () => {
      const destination = findVisibleDestination(
        root,
        pendingGuide.plan.destination,
      ) ?? (pendingGuide.plan.fallback
        ? findVisibleDestination(root, pendingGuide.plan.fallback)
        : null);
      if (!destination) return false;

      const rootRect = root.getBoundingClientRect();
      const destinationRect = destination.getBoundingClientRect();
      const behavior = window.matchMedia?.("(prefers-reduced-motion: reduce)")
        .matches
        ? "auto"
        : "smooth";
      root.scrollTo({
        top: Math.max(
          0,
          root.scrollTop + destinationRect.top - rootRect.top,
        ),
        behavior,
      });
      destination.focus({ preventScroll: true });
      observer?.disconnect();
      setPendingGuide((current) => (
        current?.id === pendingGuide.id ? null : current
      ));
      return true;
    };

    if (settle()) return;
    observer = new MutationObserver(() => {
      settle();
    });
    observer.observe(root, {
      childList: true,
      subtree: true,
      attributes: true,
      attributeFilter: ["hidden", "aria-hidden"],
    });
    settle();
    return () => observer?.disconnect();
  }, [activeTab, pendingGuide, rootRef]);

  return { guide, selectTab };
}

function findVisibleDestination(
  root: HTMLElement,
  destination: TwinGuideDestination,
): HTMLElement | null {
  const target = root.querySelector<HTMLElement>(
    `[data-twin-destination="${destination}"]`,
  );
  if (!target) return null;

  let ancestor: HTMLElement | null = target;
  while (ancestor && ancestor !== root) {
    if (ancestor.hidden || ancestor.getAttribute("aria-hidden") === "true") {
      return null;
    }
    ancestor = ancestor.parentElement;
  }
  return target;
}
