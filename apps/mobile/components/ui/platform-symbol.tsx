import { Platform, type ColorValue } from "react-native";
import { Image } from "expo-image";
import { Ionicons } from "@expo/vector-icons";
import type { ComponentProps } from "react";

// SF Symbols are an iOS image source. Android renders the bundled vector
// glyphs already used by the app's IconButton, with the same token colors.
const ANDROID_SYMBOLS = {
  tray: "file-tray-outline",
  "tray.fill": "file-tray",
  checklist: "checkbox",
  "checklist.unchecked": "checkbox-outline",
  "bubble.left": "chatbubble-outline",
  "bubble.left.fill": "chatbubble",
  ellipsis: "ellipsis-horizontal",
  pin: "pin-outline",
  "list.bullet": "list-outline",
  "square.stack": "layers-outline",
  "book.closed": "book-outline",
  "person.3": "people-outline",
  "person.3.fill": "people",
  "chevron.right": "chevron-forward",
  checkmark: "checkmark",
} satisfies Record<string, ComponentProps<typeof Ionicons>["name"]>;

export type PlatformSymbolName = keyof typeof ANDROID_SYMBOLS;

export function PlatformSymbol({
  name,
  size,
  color,
}: {
  name: PlatformSymbolName;
  size: number;
  color: ColorValue;
}) {
  return Platform.OS === "ios" ? (
    <Image
      source={`sf:${name}`}
      style={{ width: size, height: size, color }}
      accessible={false}
    />
  ) : (
    <Ionicons name={ANDROID_SYMBOLS[name]} size={size} color={color} accessible={false} />
  );
}
