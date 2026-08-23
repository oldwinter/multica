import { Pressable, View } from "react-native";
import { Image as ExpoImage } from "expo-image";
import type { Room } from "@/data/rooms-types";
import { Text } from "@/components/ui/text";
import { useColorScheme } from "@/lib/use-color-scheme";
import { roomStatusLabel } from "@/lib/room-display";
import { timeAgo } from "@/lib/time-ago";

export function RoomRow({ room, onPress }: { room: Room; onPress: () => void }) {
  const { theme } = useColorScheme();
  const outcomeReady = room.accepted_memory_revision_id !== null;

  return (
    <Pressable
      onPress={onPress}
      className="active:bg-secondary px-4 py-3"
      accessibilityRole="button"
      accessibilityLabel={`${room.title}, ${roomStatusLabel(room.status)}`}
    >
      <View className="flex-row items-start gap-3">
        <View className="size-9 rounded-md bg-secondary items-center justify-center">
          <ExpoImage
            source="sf:person.3.fill"
            tintColor={theme.foreground}
            style={{ width: 18, height: 18 }}
          />
        </View>
        <View className="flex-1 min-w-0 gap-1">
          <Text className="text-base font-medium text-foreground" numberOfLines={1}>
            {room.title || "Untitled Room"}
          </Text>
          <Text className="text-xs text-muted-foreground" numberOfLines={2}>
            {room.objective || room.instructions || "No objective yet"}
          </Text>
        </View>
        <View className="items-end gap-1 min-w-20">
          <Text
            className={
              room.status === "paused"
                ? "text-xs font-medium text-warning"
                : "text-xs font-medium text-muted-foreground"
            }
          >
            {roomStatusLabel(room.status)}
          </Text>
          <Text className="text-[11px] text-muted-foreground/70">
            {outcomeReady ? `Memory v${room.memory_version}` : timeAgo(room.updated_at)}
          </Text>
        </View>
      </View>
    </Pressable>
  );
}
