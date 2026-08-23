import { useCallback, useMemo, useState } from "react";
import {
  Alert,
  KeyboardAvoidingView,
  Platform,
  Pressable,
  ScrollView,
  View,
} from "react-native";
import { Stack, router, useLocalSearchParams } from "expo-router";
import { Text } from "@/components/ui/text";
import { TextField } from "@/components/ui/text-field";
import { AutosizeTextArea } from "@/components/ui/autosize-textarea";
import { WikiOfflineNotice } from "@/components/wiki/wiki-states";
import { useCreateWikiPage } from "@/data/mutations/wiki";
import type { CreateWikiPageInput } from "@/data/wiki-schema";

function createScope(value: string | undefined): CreateWikiPageInput["scope"] {
  return value === "project" || value === "user" ? value : "workspace";
}

export default function NewWikiPage() {
  const { workspace, scope: scopeParam, project_id: projectId } =
    useLocalSearchParams<{
      workspace: string;
      scope?: string;
      project_id?: string;
    }>();
  const scope = createScope(scopeParam);
  const create = useCreateWikiPage();
  const [path, setPath] = useState("index.md");
  const [title, setTitle] = useState("");
  const [content, setContent] = useState("# ");

  const canSave = useMemo(
    () =>
      path.trim().length > 0 &&
      (scope !== "project" || Boolean(projectId)) &&
      !create.isPending,
    [create.isPending, path, projectId, scope],
  );

  const onSave = useCallback(() => {
    if (!canSave) return;
    create.mutate(
      {
        scope,
        ...(scope === "project" && projectId ? { projectId } : {}),
        path: path.trim(),
        title: title.trim() || undefined,
        content,
      },
      {
        onSuccess: (page) => {
          if (!page.id) {
            Alert.alert("Failed to create page", "The server returned an invalid page.");
            return;
          }
          router.replace({
            pathname: "/[workspace]/wiki/[id]",
            params: { workspace, id: page.id },
          });
        },
        onError: (error) => {
          Alert.alert(
            "Failed to create page",
            error instanceof Error ? error.message : "Unknown error",
          );
        },
      },
    );
  }, [canSave, content, create, path, projectId, scope, title, workspace]);

  const headerRight = useCallback(
    () => (
      <Pressable
        accessibilityRole="button"
        accessibilityLabel="Create Wiki page"
        disabled={!canSave}
        onPress={onSave}
        className={canSave ? "px-1 py-1" : "px-1 py-1 opacity-40"}
      >
        <Text className="text-base font-semibold text-brand">
          {create.isPending ? "Creating…" : "Create"}
        </Text>
      </Pressable>
    ),
    [canSave, create.isPending, onSave],
  );

  return (
    <KeyboardAvoidingView
      className="flex-1 bg-background"
      behavior={Platform.OS === "ios" ? "padding" : undefined}
    >
      <Stack.Screen options={{ headerRight }} />
      <WikiOfflineNotice />
      <ScrollView
        contentContainerClassName="gap-5 px-4 pb-10 pt-4"
        keyboardShouldPersistTaps="handled"
        keyboardDismissMode="on-drag"
      >
        <View className="gap-1.5">
          <Text className="text-xs text-muted-foreground">Path</Text>
          <TextField
            value={path}
            onChangeText={setPath}
            placeholder="playbook/on-call.md"
            autoCapitalize="none"
            autoCorrect={false}
            accessibilityLabel="Wiki page path"
          />
          <Text className="text-xs text-muted-foreground">
            Use a stable Markdown path so links remain understandable.
          </Text>
        </View>
        <View className="gap-1.5">
          <Text className="text-xs text-muted-foreground">Title</Text>
          <TextField
            value={title}
            onChangeText={setTitle}
            placeholder="Page title"
            accessibilityLabel="Wiki page title"
          />
        </View>
        <View className="gap-1.5">
          <Text className="text-xs text-muted-foreground">Markdown</Text>
          <AutosizeTextArea
            value={content}
            onChangeText={setContent}
            minHeight={260}
            maxHeight={520}
            className="rounded-md border border-border bg-secondary/50 px-3 py-3 font-mono"
            accessibilityLabel="Wiki page Markdown"
          />
        </View>
      </ScrollView>
    </KeyboardAvoidingView>
  );
}
