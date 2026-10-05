import { useState } from "react";
import { View } from "react-native";
import { AutosizeTextArea } from "@/components/ui/autosize-textarea";
import { IconButton } from "@/components/ui/icon-button";
import { Text } from "@/components/ui/text";

interface Props {
  readonly value: string;
  readonly onChangeText: (value: string) => void;
  readonly onSend: () => Promise<void>;
  readonly disabled?: boolean;
  readonly disabledReason?: string;
}

/** Room replies deliberately omit attachment and mention semantics. A Room
 * message is plain context for the lifecycle and keeps its own draft/key. */
export function RoomComposer({
  value,
  onChangeText,
  onSend,
  disabled = false,
  disabledReason,
}: Props) {
  const [sending, setSending] = useState(false);
  const canSend = !disabled && !sending && value.trim().length > 0;

  const submit = async () => {
    if (!canSend) return;
    setSending(true);
    try {
      await onSend();
    } finally {
      setSending(false);
    }
  };

  return (
    <View className="border-t border-border bg-background px-3 py-2">
      {disabledReason ? (
        <Text className="text-xs text-muted-foreground mb-1.5">
          {disabledReason}
        </Text>
      ) : null}
      <View className="flex-row items-end gap-2 rounded-md bg-secondary/60 border border-border px-3 py-2">
        <AutosizeTextArea
          value={value}
          onChangeText={onChangeText}
          editable={!disabled && !sending}
          placeholder="Reply to this Room"
          minHeight={36}
          maxHeight={112}
          className="flex-1"
          accessibilityLabel="Room reply"
          returnKeyType="default"
        />
        <IconButton
          name={sending ? "hourglass-outline" : "arrow-up"}
          onPress={() => void submit()}
          disabled={!canSend}
          accessibilityLabel={sending ? "Sending reply" : "Send reply"}
          className="bg-primary"
          color="white"
        />
      </View>
    </View>
  );
}
