import { useCallback } from "react";
import { ActivityIndicator, FlatList, View } from "react-native";
import { SafeAreaView } from "react-native-safe-area-context";
import { router, useLocalSearchParams } from "expo-router";
import { useQuery } from "@tanstack/react-query";
import { Text } from "@/components/ui/text";
import { Button } from "@/components/ui/button";
import { WikiErrorState, WikiOfflineNotice } from "@/components/wiki/wiki-states";
import { Markdown } from "@/lib/markdown";
import {
  wikiPageDetailOptions,
  wikiPageProposalsOptions,
} from "@/data/queries/wiki";
import { useWikiPageRealtime } from "@/data/realtime/use-wiki-page-realtime";
import { useWorkspaceStore } from "@/data/workspace-store";
import {
  canReviewWikiProposal,
  wikiProposalReviewRoute,
} from "@/data/wiki-navigation";
import type { WikiProposal } from "@/data/wiki-schema";
import { timeAgo } from "@/lib/time-ago";

const STATUS_LABELS: Record<WikiProposal["status"], string> = {
  pending: "Needs review",
  accepted: "Accepted",
  rejected: "Rejected",
  unknown: "Unknown status",
};

export default function WikiProposals() {
  const { id, workspace } = useLocalSearchParams<{
    id: string;
    workspace: string;
  }>();
  const wsId = useWorkspaceStore((s) => s.currentWorkspaceId);
  const detail = useQuery(wikiPageDetailOptions(wsId, id));
  const proposals = useQuery(wikiPageProposalsOptions(wsId, id));
  const onRemoteDelete = useCallback(() => router.back(), []);
  useWikiPageRealtime(id, onRemoteDelete);

  if (proposals.isLoading || detail.isLoading) return <ActivityIndicator className="flex-1" />;
  if (proposals.error || detail.error) {
    return (
      <WikiErrorState
        message="Failed to load Agent proposals."
        onRetry={() => void Promise.all([proposals.refetch(), detail.refetch()])}
      />
    );
  }
  return (
    <SafeAreaView className="flex-1 bg-background" edges={["bottom"]}>
      <WikiOfflineNotice />
      {(proposals.data ?? []).length === 0 ? (
        <View className="flex-1 items-center justify-center gap-2 px-6">
          <Text className="text-base font-medium">No Agent proposals</Text>
          <Text className="text-center text-sm text-muted-foreground">
            Agent-authored changes appear here for human review before they can alter shared knowledge.
          </Text>
        </View>
      ) : (
        <FlatList
          data={proposals.data ?? []}
          keyExtractor={(item) => item.id}
          contentContainerClassName="gap-4 px-4 pb-10 pt-4"
          renderItem={({ item }) => (
            <View className="gap-4 rounded-md border border-border bg-card p-4">
              <View className="flex-row items-start justify-between gap-3">
                <View className="min-w-0 flex-1 gap-1">
                  <Text className="font-medium text-foreground" numberOfLines={2}>
                    {item.proposedTitle || item.proposedPath}
                  </Text>
                  <Text className="text-xs text-muted-foreground" numberOfLines={2}>
                    Based on revision {item.baseRevisionNumber} · {timeAgo(item.createdAt)}
                  </Text>
                </View>
                <Text className="shrink-0 rounded-md bg-secondary px-2 py-1 text-xs">
                  {STATUS_LABELS[item.status]}
                </Text>
              </View>
              <View className="gap-1">
                <Text className="text-xs font-medium text-muted-foreground">Rationale</Text>
                <Text className="text-sm text-foreground" selectable>
                  {item.rationale || "No rationale provided."}
                </Text>
              </View>
              <View className="gap-2 rounded-md bg-secondary/50 p-3">
                <Text className="text-xs text-muted-foreground" selectable>
                  {item.proposedPath}
                </Text>
                {item.proposedContent.trim() ? (
                  <Markdown content={item.proposedContent} />
                ) : (
                  <Text className="text-sm text-muted-foreground">Empty proposed content.</Text>
                )}
              </View>
              <Text className="text-xs text-muted-foreground">
                {item.evidenceRefs.length} evidence reference{item.evidenceRefs.length === 1 ? "" : "s"}
              </Text>
              {canReviewWikiProposal(item.status) ? (
                <Button
                  variant="outline"
                  onPress={() => router.push(wikiProposalReviewRoute(workspace, id, item.id))}
                  accessibilityLabel={`Review proposal for ${item.proposedTitle || item.proposedPath}`}
                >
                  <Text>Review and edit</Text>
                </Button>
              ) : item.reviewReason ? (
                <Text className="text-xs text-muted-foreground" selectable>
                  Review note: {item.reviewReason}
                </Text>
              ) : null}
            </View>
          )}
        />
      )}
    </SafeAreaView>
  );
}
