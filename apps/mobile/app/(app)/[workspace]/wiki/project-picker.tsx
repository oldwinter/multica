import { useMemo } from "react";
import { ActivityIndicator, FlatList, Pressable, View } from "react-native";
import { SafeAreaView } from "react-native-safe-area-context";
import { router } from "expo-router";
import { useQuery } from "@tanstack/react-query";
import { Text } from "@/components/ui/text";
import { WikiErrorState } from "@/components/wiki/wiki-states";
import { projectListOptions } from "@/data/queries/projects";
import { useWorkspaceStore } from "@/data/workspace-store";
import { useNativeSearchBar } from "@/lib/use-native-search-bar";

export default function WikiProjectPicker() {
  const wsId = useWorkspaceStore((s) => s.currentWorkspaceId);
  const wsSlug = useWorkspaceStore((s) => s.currentWorkspaceSlug);
  const query = useNativeSearchBar("Search projects", { autoFocus: true });
  const projects = useQuery(projectListOptions(wsId));
  const filtered = useMemo(() => {
    const normalized = query.trim().toLocaleLowerCase();
    if (!normalized) return projects.data ?? [];
    return (projects.data ?? []).filter((project) =>
      project.title.toLocaleLowerCase().includes(normalized),
    );
  }, [projects.data, query]);

  if (projects.isLoading) {
    return <ActivityIndicator className="flex-1" />;
  }
  if (projects.error) {
    return (
      <WikiErrorState
        message="Failed to load projects."
        onRetry={() => void projects.refetch()}
      />
    );
  }
  return (
    <SafeAreaView className="flex-1 bg-background" edges={["bottom"]}>
      {filtered.length === 0 ? (
        <View className="flex-1 items-center justify-center px-6">
          <Text className="text-sm text-muted-foreground">No matching projects.</Text>
        </View>
      ) : (
        <FlatList
          data={filtered}
          keyExtractor={(item) => item.id}
          keyboardDismissMode="on-drag"
          ItemSeparatorComponent={() => <View className="ml-4 h-px bg-border" />}
          renderItem={({ item }) => (
            <Pressable
              accessibilityRole="button"
              accessibilityLabel={`Use ${item.title} for Wiki pages`}
              onPress={() => {
                if (!wsSlug) return;
                router.replace({
                  pathname: "/[workspace]/more/wiki",
                  params: { workspace: wsSlug, scope: "project", project_id: item.id },
                });
              }}
              className="px-4 py-3 active:bg-secondary"
            >
              <Text className="text-base text-foreground" numberOfLines={2}>
                {item.title}
              </Text>
            </Pressable>
          )}
        />
      )}
    </SafeAreaView>
  );
}
