import { useCallback } from "react";
import { Alert } from "react-native";
import { useQuery } from "@tanstack/react-query";
import { ApiError } from "@/data/api";
import { usePinWikiRevisionAsLMWikiEvidence } from "@/data/mutations/wiki";
import { wikiKnowledgeReadinessOptions } from "@/data/queries/wiki";
import {
  getLMWikiSourcePolicyStaleConflict,
  type WikiActorType,
  type WikiKnowledgeSourceReadiness,
  type WikiScope,
  type WikiSourceKind,
} from "@/data/wiki-schema";
import { useWorkspaceStore } from "@/data/workspace-store";

export interface MobileWikiActivationTarget {
  pageId: string;
  revisionId: string;
  revisionNumber: number;
  title: string;
  path: string;
  contentDigest: string;
  scope: WikiScope;
  sourceKind: WikiSourceKind;
  actorType: WikiActorType;
}

export function useWikiKnowledgeActivation(pageId: string) {
  const wsId = useWorkspaceStore((state) => state.currentWorkspaceId);
  const readinessQuery = useQuery(wikiKnowledgeReadinessOptions(wsId));
  const pinRevision = usePinWikiRevisionAsLMWikiEvidence();
  const readiness = readinessQuery.data;
  const source = readiness?.sources.find((candidate) => candidate.pageId === pageId);

  const canPinRevision = useCallback((revisionId: string, scope: WikiScope) => (
    scope !== "user"
      && scope !== "unknown"
      && Boolean(readiness?.canManage)
      && Boolean(source)
      && source?.state !== "source_deleted"
      && (source?.state !== "excluded" || source.nextAction.kind === "pin_revision")
      && (source?.state === "excluded" || source?.selectedRevisionId !== revisionId)
      && !pinRevision.isPending
  ), [pinRevision.isPending, readiness?.canManage, source]);

  const confirmRevision = useCallback((target: MobileWikiActivationTarget) => {
    if (target.scope === "user" || target.scope === "unknown") {
      Alert.alert(
        "Always excluded",
        "Personal Wiki is private and can never become shared LM Wiki evidence.",
      );
      return;
    }
    if (!readiness || !source) {
      Alert.alert(
        "LM Wiki status unavailable",
        "The source status could not be verified. Reload the page before trying again.",
      );
      return;
    }
    if (!readiness.canManage) {
      Alert.alert(
        "Owner or admin required",
        "Only workspace owners and admins can change LM Wiki evidence.",
      );
      return;
    }
    if (source.selectedRevisionId === target.revisionId) {
      Alert.alert("Exact revision pinned", `Revision ${target.revisionNumber} is already selected by the source policy.`);
      return;
    }
    if (source.state === "source_deleted" || (source.state === "excluded" && source.nextAction.kind !== "pin_revision")) {
      Alert.alert("Source is not eligible", sourceStateLabel(source));
      return;
    }

    const exclusions = readiness.policy.exclusions.length > 0
      ? readiness.policy.exclusions
        .map((exclusion) => `• ${exclusion.sourceClass}: ${exclusion.reason}`)
        .join("\n")
      : "None reported";
    const remoteState = readiness.policy.remoteGenerationEnabled
      ? "Enabled. This revision may leave the workspace boundary during a later LM Wiki refresh."
      : "Disabled. Pinning does not enable remote egress.";
    const message = [
      `Page: ${target.title || target.path}`,
      `Path: ${target.path}`,
      `Scope: ${target.scope}`,
      `Exact revision: ${target.revisionNumber}`,
      `Digest: ${target.contentDigest}`,
      `Provenance: ${target.sourceKind} by ${target.actorType}`,
      `Source policy: version ${readiness.policy.policyVersion} · ${readiness.policy.policyDigest}`,
      `Remote generation: ${remoteState}`,
      "Permanent exclusions:",
      exclusions,
      "",
      "This changes the source policy only. It does not refresh or accept LM Wiki evidence.",
    ].join("\n");

    Alert.alert("Use this exact revision as LM Wiki evidence?", message, [
      { text: "Cancel", style: "cancel" },
      {
        text: "Pin exact revision",
        onPress: () => {
          pinRevision.mutate({
            pageId: target.pageId,
            revisionId: target.revisionId,
            expectedPolicyVersion: readiness.policy.policyVersion,
            expectedPolicyDigest: readiness.policy.policyDigest,
          }, {
            onSuccess: () => Alert.alert(
              "Exact revision pinned",
              "The source policy changed. LM Wiki was not refreshed or accepted.",
            ),
            onError: (error) => {
              const conflict = error instanceof ApiError
                ? getLMWikiSourcePolicyStaleConflict(error.body)
                : null;
              if (conflict) {
                Alert.alert(
                  "Source policy changed",
                  `The server is now on policy version ${conflict.currentPolicy.policyVersion}. Your revision context is unchanged. Reload policy details before confirming again.`,
                  [{ text: "Reload", onPress: () => void readinessQuery.refetch() }],
                );
                return;
              }
              Alert.alert(
                "Could not pin revision",
                error instanceof Error ? error.message : "Unknown error",
              );
            },
          });
        },
      },
    ]);
  }, [pinRevision, readiness, readinessQuery, source]);

  return {
    source,
    statusLabel: readinessQuery.isPending
      ? "Checking LM Wiki status"
      : readinessQuery.isError
        ? "LM Wiki status unavailable"
        : source
          ? sourceStateLabel(source)
          : "LM Wiki status unavailable",
    canManage: readiness?.canManage === true,
    isPending: pinRevision.isPending,
    canPinRevision,
    confirmRevision,
  };
}

export function sourceStateLabel(source: WikiKnowledgeSourceReadiness): string {
  switch (source.state) {
    case "eligible_unpinned": return "Eligible, not pinned";
    case "pinned_current": return "Pinned and current";
    case "newer_revision_available": return "Newer revision available";
    case "source_deleted": return "Pinned source deleted";
    case "excluded": return "Always excluded";
    case "policy_stale": return "LM Wiki refresh required";
    default: return source.state;
  }
}
