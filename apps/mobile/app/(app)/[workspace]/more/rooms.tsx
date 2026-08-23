import { useMemo } from "react";
import {
  ActivityIndicator,
  FlatList,
  RefreshControl,
  View,
} from "react-native";
import { SafeAreaView } from "react-native-safe-area-context";
import { router } from "expo-router";
import { useQuery } from "@tanstack/react-query";
import { RoomRow } from "@/components/room/room-row";
import { Button } from "@/components/ui/button";
import { Text } from "@/components/ui/text";
import { roomListOptions } from "@/data/queries/rooms";
import { filterRooms } from "@/lib/room-selectors";
import { useNativeSearchBar } from "@/lib/use-native-search-bar";
import { useWorkspaceStore } from "@/data/workspace-store";

export default function RoomsPage() {
  const wsId = useWorkspaceStore((state) => state.currentWorkspaceId);
  const wsSlug = useWorkspaceStore((state) => state.currentWorkspaceSlug);
  const query = useQuery(roomListOptions(wsId));
  const search = useNativeSearchBar("Search Rooms");
  const rooms = useMemo(
    () =>
      [...filterRooms(query.data ?? [], search)].sort(
        (a, b) => Date.parse(b.updated_at) - Date.parse(a.updated_at),
      ),
    [query.data, search],
  );

  return (
    <SafeAreaView className="flex-1 bg-background" edges={[]}>
      {query.isLoading ? (
        <View className="flex-1 items-center justify-center">
          <ActivityIndicator />
        </View>
      ) : query.error ? (
        <View className="px-4 pt-4 gap-3">
          <Text className="text-sm text-destructive">
            Failed to load Rooms: {query.error.message}
          </Text>
          <Button variant="outline" onPress={() => void query.refetch()}>
            <Text>Retry</Text>
          </Button>
        </View>
      ) : rooms.length === 0 ? (
        <View className="flex-1 items-center justify-center px-6 gap-3">
          <Text className="text-base font-medium text-foreground">
            {search ? "No matching Rooms" : "No Rooms yet"}
          </Text>
          {!search ? (
            <Text className="text-sm text-muted-foreground text-center">
              Rooms created on web or desktop will appear here for outcome review
              and human approval.
            </Text>
          ) : null}
        </View>
      ) : (
        <FlatList
          data={rooms}
          keyExtractor={(room) => room.id}
          renderItem={({ item }) => (
            <RoomRow
              room={item}
              onPress={() => {
                if (wsSlug) router.push(`/${wsSlug}/room/${item.id}`);
              }}
            />
          )}
          ItemSeparatorComponent={() => <View className="h-px bg-border ml-4" />}
          refreshControl={
            <RefreshControl
              refreshing={query.isRefetching}
              onRefresh={() => void query.refetch()}
            />
          }
          contentContainerClassName="pb-6"
        />
      )}
    </SafeAreaView>
  );
}
