import { View } from "react-native";
import { Ionicons } from "@expo/vector-icons";
import { useNetInfo } from "@react-native-community/netinfo";
import { useTheme } from "@react-navigation/native";
import { Text } from "@/components/ui/text";
import { Button } from "@/components/ui/button";

export function WikiOfflineNotice() {
  const network = useNetInfo();
  const { colors } = useTheme();
  if (network.isConnected !== false) return null;
  return (
    <View
      accessibilityRole="alert"
      className="mx-4 mt-3 flex-row items-center gap-2 rounded-md bg-muted px-3 py-2"
    >
      <Ionicons name="cloud-offline-outline" size={16} color={colors.text} />
      <Text className="min-w-0 flex-1 text-xs text-muted-foreground">
        Offline. Showing cached Wiki content. Reconnect before saving changes.
      </Text>
    </View>
  );
}

export function WikiErrorState({
  message,
  onRetry,
}: {
  message: string;
  onRetry: () => void;
}) {
  return (
    <View className="flex-1 items-center justify-center gap-3 px-6 py-8">
      <Text accessibilityRole="alert" className="text-center text-sm text-destructive">
        {message}
      </Text>
      <Button variant="outline" onPress={onRetry}>
        <Text>Retry</Text>
      </Button>
    </View>
  );
}
