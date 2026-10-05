import { useCallback, useMemo } from "react";
import { ActivityIndicator, FlatList, Pressable, View } from "react-native";
import SegmentedControl from "@react-native-segmented-control/segmented-control";
import { SafeAreaView } from "react-native-safe-area-context";
import { useTheme } from "@react-navigation/native";
import { Stack, router, useLocalSearchParams } from "expo-router";
import { useQuery } from "@tanstack/react-query";
import { Text } from "@/components/ui/text";
import { IconButton } from "@/components/ui/icon-button";
import { WikiPageRow } from "@/components/wiki/wiki-page-row";
import {
  WikiErrorState,
  WikiOfflineNotice,
} from "@/components/wiki/wiki-states";
import {
  wikiPageListOptions,
  wikiPageSearchOptions,
} from "@/data/queries/wiki";
import { projectListOptions } from "@/data/queries/projects";
import { useWorkspaceStore } from "@/data/workspace-store";
import type { ListWikiPagesParams } from "@/data/wiki-schema";
import { useNativeSearchBar } from "@/lib/use-native-search-bar";

const SCOPES = ["workspace", "project", "user"] as const;
const SCOPE_LABELS = ["Workspace", "Project", "Personal"];

function normalizeScope(value: string | undefined): ListWikiPagesParams["scope"] {
  return value === "project" || value === "user" ? value : "workspace";
}

export default function WikiListScreen() {
  const { scope: scopeParam, project_id: projectId } = useLocalSearchParams<{
    scope?: string;
    project_id?: string;
  }>();
  const wsId = useWorkspaceStore((s) => s.currentWorkspaceId);
  const wsSlug = useWorkspaceStore((s) => s.currentWorkspaceSlug);
  const scope = normalizeScope(scopeParam);
  const query = useNativeSearchBar("Search Wiki");
  const params = useMemo<ListWikiPagesParams>(
    () => ({ scope, projectId: scope === "project" ? projectId : undefined }),
    [scope, projectId],
  );
  const list = useQuery(wikiPageListOptions(wsId, params));
  const search = useQuery(wikiPageSearchOptions(wsId, query, params));
  const projects = useQuery(projectListOptions(wsId));
  const active = query.trim() ? search : list;
  const selectedProject = projects.data?.find((project) => project.id === projectId);
  const { colors } = useTheme();

  const replaceScope = useCallback(
    (next: ListWikiPagesParams["scope"]) => {
      if (!wsSlug) return;
      router.replace({
        pathname: "/[workspace]/more/wiki",
        params: {
          workspace: wsSlug,
          scope: next,
          ...(next === "project" && projectId ? { project_id: projectId } : {}),
        },
      });
    },
    [projectId, wsSlug],
  );

  const goCreate = useCallback(() => {
    if (!wsSlug || (scope === "project" && !projectId)) return;
    router.push({
      pathname: "/[workspace]/wiki/new",
      params: {
        workspace: wsSlug,
        scope,
        ...(projectId ? { project_id: projectId } : {}),
      },
    });
  }, [projectId, scope, wsSlug]);

  const headerRight = useCallback(
    () => (
      <IconButton
        name="add"
        onPress={goCreate}
        disabled={scope === "project" && !projectId}
        accessibilityLabel="New Wiki page"
      />
    ),
    [goCreate, projectId, scope],
  );

  const needsProject = scope === "project" && !projectId;
  return (
    <SafeAreaView className="flex-1 bg-background" edges={[]}>
      <Stack.Screen options={{ headerRight }} />
      <WikiOfflineNotice />
      <View className="gap-3 px-4 pb-3 pt-3">
        <SegmentedControl
          values={SCOPE_LABELS}
          selectedIndex={SCOPES.indexOf(scope)}
          onChange={(event) => replaceScope(SCOPES[event.nativeEvent.selectedSegmentIndex])}
          tintColor={colors.primary}
          fontStyle={{ color: colors.text }}
          activeFontStyle={{ color: colors.background, fontWeight: "600" }}
          accessibilityLabel="Wiki scope"
        />
        {scope === "project" ? (
          <Pressable
            accessibilityRole="button"
            accessibilityLabel="Select project for Wiki pages"
            onPress={() => {
              if (!wsSlug) return;
              router.push({
                pathname: "/[workspace]/wiki/project-picker",
                params: { workspace: wsSlug, ...(projectId ? { project_id: projectId } : {}) },
              });
            }}
            className="flex-row items-center justify-between rounded-md bg-secondary px-3 py-2.5 active:opacity-70"
          >
            <Text className="min-w-0 flex-1 text-sm text-foreground" numberOfLines={2}>
              {selectedProject?.title ?? "Select a project"}
            </Text>
            <Text className="ml-3 text-sm text-muted-foreground">Change</Text>
          </Pressable>
        ) : null}
        {scope === "user" ? (
          <Text className="text-xs text-muted-foreground">
            Personal pages are private to you and excluded from shared LM Wiki evidence.
          </Text>
        ) : null}
      </View>

      {needsProject ? (
        <View className="flex-1 items-center justify-center px-6 gap-2">
          <Text className="text-base font-medium">Choose a project</Text>
          <Text className="text-center text-sm text-muted-foreground">
            Project Wiki pages keep project-specific knowledge out of the workspace library.
          </Text>
        </View>
      ) : active.isLoading ? (
        <View className="flex-1 items-center justify-center">
          <ActivityIndicator />
        </View>
      ) : active.error ? (
        <WikiErrorState
          message={`Failed to load Wiki pages: ${active.error instanceof Error ? active.error.message : "unknown error"}`}
          onRetry={() => void active.refetch()}
        />
      ) : (active.data ?? []).length === 0 ? (
        <View className="flex-1 items-center justify-center gap-2 px-6">
          <Text className="text-base font-medium">
            {query.trim() ? "No matching pages" : "No Wiki pages yet"}
          </Text>
          <Text className="text-center text-sm text-muted-foreground">
            {query.trim()
              ? "Try a different title, path, or content search."
              : "Capture durable Markdown knowledge beside the work it supports."}
          </Text>
        </View>
      ) : (
        <FlatList
          data={active.data ?? []}
          keyExtractor={(item) => item.id}
          keyboardDismissMode="on-drag"
          ItemSeparatorComponent={() => <View className="ml-16 h-px bg-border" />}
          renderItem={({ item }) => (
            <WikiPageRow
              page={item}
              onPress={() => {
                if (!wsSlug) return;
                router.push({
                  pathname: "/[workspace]/wiki/[id]",
                  params: { workspace: wsSlug, id: item.id },
                });
              }}
            />
          )}
          contentContainerClassName="pb-8"
        />
      )}
    </SafeAreaView>
  );
}
