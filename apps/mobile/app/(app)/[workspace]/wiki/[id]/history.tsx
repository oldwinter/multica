import { useCallback } from "react";
import { ActivityIndicator, Alert, FlatList, Pressable, View } from "react-native";
import { SafeAreaView } from "react-native-safe-area-context";
import { router, useLocalSearchParams } from "expo-router";
import { useQuery } from "@tanstack/react-query";
import { Text } from "@/components/ui/text";
import { Button } from "@/components/ui/button";
import { WikiErrorState, WikiOfflineNotice } from "@/components/wiki/wiki-states";
import { useWikiKnowledgeActivation } from "@/components/wiki/use-wiki-knowledge-activation";
import {
  wikiPageDetailOptions,
  wikiPageRevisionsOptions,
} from "@/data/queries/wiki";
import {
  useRestoreWikiPageRevision,
  wikiConflictFromError,
} from "@/data/mutations/wiki";
import { useWikiPageRealtime } from "@/data/realtime/use-wiki-page-realtime";
import { useWorkspaceStore } from "@/data/workspace-store";
import type { WikiRevision } from "@/data/wiki-schema";
import { timeAgo } from "@/lib/time-ago";

export default function WikiHistory() {
  const { id } = useLocalSearchParams<{ id: string }>();
  const wsId = useWorkspaceStore((s) => s.currentWorkspaceId);
  const detail = useQuery(wikiPageDetailOptions(wsId, id));
  const revisions = useQuery(wikiPageRevisionsOptions(wsId, id));
  const restore = useRestoreWikiPageRevision(id);
  const activation = useWikiKnowledgeActivation(id);
  const onRemoteDelete = useCallback(() => router.back(), []);
  useWikiPageRealtime(id, onRemoteDelete);

  const onRestore = useCallback(
    (revision: WikiRevision) => {
      const expected = detail.data?.currentRevisionNumber;
      if (!expected) return;
      Alert.alert(
        `Restore revision ${revision.revisionNumber}?`,
        "The old revision remains immutable. Restore creates a new current revision.",
        [
          { text: "Cancel", style: "cancel" },
          {
            text: "Restore",
            onPress: () =>
              restore.mutate(
                { revisionId: revision.id, expectedRevisionNumber: expected },
                {
                  onError: (error) => {
                    const conflict = wikiConflictFromError(error);
                    Alert.alert(
                      conflict ? "A newer revision exists" : "Restore failed",
                      conflict
                        ? `The page is now revision ${conflict.currentRevisionNumber}. Refresh history before restoring.`
                        : error instanceof Error
                          ? error.message
                          : "Unknown error",
                      conflict
                        ? [
                            {
                              text: "Refresh",
                              onPress: () =>
                                void Promise.all([detail.refetch(), revisions.refetch()]),
                            },
                          ]
                        : undefined,
                    );
                  },
                },
              ),
          },
        ],
      );
    },
    [detail.data?.currentRevisionNumber, restore, revisions],
  );

  if (revisions.isLoading || detail.isLoading) return <ActivityIndicator className="flex-1" />;
  if (revisions.error || detail.error) {
    return (
      <WikiErrorState
        message="Failed to load revision history."
        onRetry={() => void Promise.all([revisions.refetch(), detail.refetch()])}
      />
    );
  }
  return (
    <SafeAreaView className="flex-1 bg-background" edges={["bottom"]}>
      <WikiOfflineNotice />
      {(revisions.data ?? []).length === 0 ? (
        <View className="flex-1 items-center justify-center px-6">
          <Text className="text-sm text-muted-foreground">No revisions yet.</Text>
        </View>
      ) : (
        <FlatList
          data={revisions.data ?? []}
          keyExtractor={(item) => item.id}
          contentContainerClassName="gap-3 px-4 pb-10 pt-4"
          ListHeaderComponent={(
            <View className="gap-1 border-b border-border pb-3">
              <Text className="text-sm font-medium text-foreground">LM Wiki source health</Text>
              <Text className="text-sm text-muted-foreground">
                {detail.data?.scope === "user" ? "Always excluded" : activation.statusLabel}
              </Text>
              {detail.data?.scope === "user" ? (
                <Text className="text-xs text-muted-foreground">
                  Personal Wiki is permanently excluded from shared LM Wiki evidence.
                </Text>
              ) : !activation.canManage ? (
                <Text className="text-xs text-muted-foreground">
                  Only workspace owners and admins can change LM Wiki evidence.
                </Text>
              ) : null}
            </View>
          )}
          renderItem={({ item }) => {
            const current = item.revisionNumber === detail.data?.currentRevisionNumber;
            const exactPinned = activation.source?.selectedRevisionId === item.id
              && activation.source.state !== "excluded";
            const scope = detail.data?.scope ?? "unknown";
            return (
              <View className="gap-3 rounded-md border border-border bg-card p-3">
                <View className="flex-row items-start justify-between gap-3">
                  <View className="min-w-0 flex-1 gap-1">
                    <Text className="font-medium text-foreground" numberOfLines={2}>
                      Revision {item.revisionNumber} · {item.title || item.path}
                    </Text>
                    <Text className="text-xs text-muted-foreground" numberOfLines={2}>
                      {item.sourceKind} · {item.actorType} · {timeAgo(item.createdAt)}
                    </Text>
                  </View>
                  {current ? (
                    <Text className="rounded-md bg-secondary px-2 py-1 text-xs">Current</Text>
                  ) : null}
                </View>
                <Text className="text-sm text-muted-foreground" numberOfLines={6} selectable>
                  {item.content || "Empty revision"}
                </Text>
                <Button
                  variant="outline"
                  size="sm"
                  disabled={!activation.canPinRevision(item.id, scope)}
                  onPress={() => activation.confirmRevision({
                    pageId: item.pageId,
                    revisionId: item.id,
                    revisionNumber: item.revisionNumber,
                    title: item.title,
                    path: item.path,
                    contentDigest: item.contentDigest,
                    scope,
                    sourceKind: item.sourceKind,
                    actorType: item.actorType,
                  })}
                  accessibilityLabel={exactPinned
                    ? `Revision ${item.revisionNumber} is pinned as LM Wiki evidence`
                    : `Use revision ${item.revisionNumber} as LM Wiki evidence`}
                >
                  <Text>{exactPinned ? "Exact revision pinned" : "Use as LM Wiki evidence"}</Text>
                </Button>
                {!current ? (
                  <Button
                    variant="outline"
                    size="sm"
                    disabled={restore.isPending}
                    onPress={() => onRestore(item)}
                    accessibilityLabel={`Restore revision ${item.revisionNumber}`}
                  >
                    <Text>Restore as new revision</Text>
                  </Button>
                ) : null}
              </View>
            );
          }}
        />
      )}
    </SafeAreaView>
  );
}
