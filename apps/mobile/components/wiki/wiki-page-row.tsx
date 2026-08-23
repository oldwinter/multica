import { Pressable, View } from "react-native";
import { Ionicons } from "@expo/vector-icons";
import { useTheme } from "@react-navigation/native";
import { Text } from "@/components/ui/text";
import type { WikiPageSummary } from "@/data/wiki-schema";
import { timeAgo } from "@/lib/time-ago";

const SCOPE_LABELS: Record<WikiPageSummary["scope"], string> = {
  workspace: "Workspace",
  project: "Project",
  user: "Personal",
  unknown: "Unknown scope",
};

export function WikiPageRow({
  page,
  onPress,
}: {
  page: WikiPageSummary;
  onPress: () => void;
}) {
  const { colors } = useTheme();
  return (
    <Pressable
      onPress={onPress}
      accessibilityRole="button"
      accessibilityLabel={`${page.title || page.path}, ${SCOPE_LABELS[page.scope]}, revision ${page.currentRevisionNumber}`}
      className="active:bg-secondary px-4 py-3"
    >
      <View className="flex-row items-start gap-3">
        <View className="mt-0.5 size-9 items-center justify-center rounded-md bg-secondary">
          <Ionicons name="document-text-outline" size={19} color={colors.text} />
        </View>
        <View className="min-w-0 flex-1 gap-1">
          <Text className="text-base font-medium text-foreground" numberOfLines={2}>
            {page.title || page.path}
          </Text>
          <Text className="text-xs text-muted-foreground" numberOfLines={2}>
            {page.path}
          </Text>
        </View>
        <View className="shrink-0 items-end gap-1">
          <Text className="text-xs text-muted-foreground tabular-nums">
            r{page.currentRevisionNumber}
          </Text>
          <Text className="text-xs text-muted-foreground/70">
            {timeAgo(page.updatedAt)}
          </Text>
        </View>
      </View>
    </Pressable>
  );
}

export function wikiScopeLabel(scope: WikiPageSummary["scope"]): string {
  return SCOPE_LABELS[scope];
}
