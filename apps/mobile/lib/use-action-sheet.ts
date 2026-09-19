import { useCallback } from "react";
import {
  useActionSheet,
  type ActionSheetOptions,
} from "@expo/react-native-action-sheet";
import { useColorScheme } from "@/lib/use-color-scheme";

export type ShowActionSheet = (
  options: ActionSheetOptions,
  callback: (index: number) => void | Promise<void>,
) => void;

/** Native iOS sheets and Expo's Android sheet share the app's skin tokens. */
export function useAppActionSheet(): ShowActionSheet {
  const { showActionSheetWithOptions } = useActionSheet();
  const { theme, colorScheme } = useColorScheme();
  return useCallback<ShowActionSheet>(
    (options, callback) => {
      showActionSheetWithOptions(
        {
          useModal: true,
          userInterfaceStyle: colorScheme,
          tintColor: theme.foreground,
          destructiveColor: theme.destructive,
          containerStyle: { backgroundColor: theme.background },
          textStyle: { color: theme.foreground },
          titleTextStyle: { color: theme.mutedForeground },
          messageTextStyle: { color: theme.mutedForeground },
          ...options,
        },
        // Android Back/outside dismissal must also clear long-press state.
        (index) => callback(index ?? options.cancelButtonIndex ?? -1),
      );
    },
    [showActionSheetWithOptions, theme, colorScheme],
  );
}
