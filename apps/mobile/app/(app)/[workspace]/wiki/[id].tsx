import { useCallback } from "react";
import {
  ActionSheetIOS,
  ActivityIndicator,
  Alert,
  RefreshControl,
  ScrollView,
  View,
} from "react-native";
import { SafeAreaView } from "react-native-safe-area-context";
import { Stack, router, useLocalSearchParams } from "expo-router";
import * as Clipboard from "expo-clipboard";
import { useQuery } from "@tanstack/react-query";
import { Text } from "@/components/ui/text";
import { Button } from "@/components/ui/button";
import { IconButton } from "@/components/ui/icon-button";
import { WikiErrorState, WikiOfflineNotice } from "@/components/wiki/wiki-states";
import { wikiScopeLabel } from "@/components/wiki/wiki-page-row";
import { Markdown } from "@/lib/markdown";
import { timeAgo } from "@/lib/time-ago";
import { wikiPageDetailOptions } from "@/data/queries/wiki";
import { useDeleteWikiPage } from "@/data/mutations/wiki";
import { useWikiPageRealtime } from "@/data/realtime/use-wiki-page-realtime";
import { useWorkspaceStore } from "@/data/workspace-store";

const SOURCE_LABELS = {
  human: "Human edit",
  room_promotion: "Room promotion",
  agent_proposal: "Accepted Agent proposal",
  restore: "Restored revision",
  system: "System migration",
  unknown: "Unknown source",
} as const;

export default function WikiPageDetail() {
  const { id, workspace } = useLocalSearchParams<{ id: string; workspace: string }>();
  const wsId = useWorkspaceStore((s) => s.currentWorkspaceId);
  const detail = useQuery(wikiPageDetailOptions(wsId, id));
  const remove = useDeleteWikiPage(id);
  const onRemoteDelete = useCallback(() => router.back(), []);
  useWikiPageRealtime(id, onRemoteDelete);
  const page = detail.data;
  const citationKey = page?.currentRevisionId
    ? `wiki_page_revision:${page.currentRevisionId}`
    : "";

  const copyCitation = useCallback(() => {
    if (!citationKey) return;
    void Clipboard.setStringAsync(citationKey).then(() => {
      Alert.alert("Citation copied", citationKey);
    });
  }, [citationKey]);

  const navigate = useCallback(
    (target: "edit" | "history" | "proposals") => {
      router.push({
        pathname: `/[workspace]/wiki/[id]/${target}`,
        params: { workspace, id },
      });
    },
    [id, workspace],
  );

  const onDelete = useCallback(() => {
    Alert.alert(
      "Delete Wiki page?",
      "This removes the page from the current library. Existing immutable evidence citations remain auditable.",
      [
        { text: "Cancel", style: "cancel" },
        {
          text: "Delete",
          style: "destructive",
          onPress: () =>
            remove.mutate(undefined, {
              onSuccess: () => router.back(),
              onError: (error) =>
                Alert.alert(
                  "Failed to delete page",
                  error instanceof Error ? error.message : "Unknown error",
                ),
            }),
        },
      ],
    );
  }, [remove]);

  const onActions = useCallback(() => {
    const options = ["Cancel", "Edit", "Revision history", "Agent proposals", "Delete"];
    ActionSheetIOS.showActionSheetWithOptions(
      { options, cancelButtonIndex: 0, destructiveButtonIndex: 4 },
      (index) => {
        if (index === 1) navigate("edit");
        else if (index === 2) navigate("history");
        else if (index === 3) navigate("proposals");
        else if (index === 4) onDelete();
      },
    );
  }, [navigate, onDelete]);

  return (
    <SafeAreaView className="flex-1 bg-background" edges={["bottom"]}>
      <Stack.Screen
        options={{
          title: page?.title || "Wiki Page",
          headerRight: page
            ? () => (
                <IconButton
                  name="ellipsis-horizontal"
                  onPress={onActions}
                  accessibilityLabel="Wiki page actions"
                />
              )
            : undefined,
        }}
      />
      <WikiOfflineNotice />
      {detail.isLoading ? (
        <ActivityIndicator className="flex-1" />
      ) : detail.error || !page?.id ? (
        <WikiErrorState
          message={`Failed to load Wiki page: ${detail.error instanceof Error ? detail.error.message : "not found"}`}
          onRetry={() => void detail.refetch()}
        />
      ) : (
        <ScrollView
          refreshControl={
            <RefreshControl refreshing={detail.isRefetching} onRefresh={detail.refetch} />
          }
          contentContainerClassName="gap-5 px-4 pb-12 pt-4"
        >
          <View className="gap-2">
            <Text className="text-2xl font-semibold text-foreground" selectable>
              {page.title || page.path}
            </Text>
            <Text className="text-sm text-muted-foreground" selectable>
              {page.path}
            </Text>
            <View className="flex-row flex-wrap gap-2">
              <Text className="rounded-md bg-secondary px-2 py-1 text-xs text-muted-foreground">
                {wikiScopeLabel(page.scope)}
              </Text>
              <Text className="rounded-md bg-secondary px-2 py-1 text-xs text-muted-foreground">
                Revision {page.currentRevisionNumber}
              </Text>
              <Text className="rounded-md bg-secondary px-2 py-1 text-xs text-muted-foreground">
                {SOURCE_LABELS[page.lastSourceKind]}
              </Text>
            </View>
            <Text className="text-xs text-muted-foreground">
              Updated {timeAgo(page.updatedAt)} · digest {page.contentDigest.slice(0, 18) || "unavailable"}
            </Text>
            <View className="flex-row items-center gap-2 rounded-md bg-secondary/50 px-3 py-2">
              <Text
                className="min-w-0 flex-1 font-mono text-xs text-foreground"
                numberOfLines={2}
                selectable
              >
                {citationKey}
              </Text>
              <IconButton
                name="copy-outline"
                iconSize={18}
                onPress={copyCitation}
                accessibilityLabel="Copy exact Wiki revision citation"
              />
            </View>
          </View>
          <View className="h-px bg-border" />
          {page.content.trim() ? (
            <Markdown content={page.content} />
          ) : (
            <Text className="text-sm text-muted-foreground">This page is empty.</Text>
          )}
          <View className="flex-row flex-wrap gap-2 pt-2">
            <Button variant="outline" onPress={() => navigate("history")}>
              <Text>History</Text>
            </Button>
            <Button variant="outline" onPress={() => navigate("proposals")}>
              <Text>Agent proposals</Text>
            </Button>
          </View>
        </ScrollView>
      )}
    </SafeAreaView>
  );
}
