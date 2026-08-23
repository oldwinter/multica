import { useCallback, useEffect, useMemo, useState } from "react";
import {
  Alert,
  KeyboardAvoidingView,
  Platform,
  Pressable,
  ScrollView,
  View,
} from "react-native";
import { Stack, router, useLocalSearchParams } from "expo-router";
import { useQuery } from "@tanstack/react-query";
import { Text } from "@/components/ui/text";
import { TextField } from "@/components/ui/text-field";
import { AutosizeTextArea } from "@/components/ui/autosize-textarea";
import { WikiErrorState, WikiOfflineNotice } from "@/components/wiki/wiki-states";
import { wikiPageDetailOptions } from "@/data/queries/wiki";
import {
  useUpdateWikiPage,
  wikiConflictFromError,
} from "@/data/mutations/wiki";
import { useWorkspaceStore } from "@/data/workspace-store";
import {
  buildWikiUpdateBody,
  rebaseWikiDraftAfterConflict,
} from "@/data/wiki-edit-conflict";

export default function EditWikiPage() {
  const { id } = useLocalSearchParams<{ id: string }>();
  const wsId = useWorkspaceStore((s) => s.currentWorkspaceId);
  const detail = useQuery(wikiPageDetailOptions(wsId, id));
  const update = useUpdateWikiPage(id);
  const [path, setPath] = useState("");
  const [title, setTitle] = useState("");
  const [content, setContent] = useState("");
  const [baseRevision, setBaseRevision] = useState(0);
  const [seeded, setSeeded] = useState(false);
  const [conflictRevision, setConflictRevision] = useState<number | null>(null);

  const seedFromPage = useCallback((page: NonNullable<typeof detail.data>) => {
    setPath(page.path);
    setTitle(page.title);
    setContent(page.content);
    setBaseRevision(page.currentRevisionNumber);
    setConflictRevision(null);
    setSeeded(true);
  }, []);

  useEffect(() => {
    if (!detail.data?.id || seeded) return;
    seedFromPage(detail.data);
  }, [detail.data, seedFromPage, seeded]);

  const dirty = useMemo(() => {
    const page = detail.data;
    if (!page || !seeded) return false;
    return path.trim() !== page.path || title.trim() !== page.title || content !== page.content;
  }, [content, detail.data, path, seeded, title]);
  const canSave = seeded && path.trim().length > 0 && dirty && !update.isPending;

  const reloadLatest = useCallback(async () => {
    const result = await detail.refetch();
    if (result.data?.id) seedFromPage(result.data);
  }, [detail, seedFromPage]);

  const handleConflict = useCallback(
    (error: unknown) => {
      const conflict = wikiConflictFromError(error);
      if (!conflict) {
        Alert.alert(
          "Failed to save",
          error instanceof Error ? error.message : "Unknown error",
        );
        return;
      }
      Alert.alert(
        "A newer revision exists",
        `Your draft is based on revision ${baseRevision}; the page is now revision ${conflict.currentRevisionNumber}. Keep the draft and retry against the new revision, or reload and discard it.`,
        [
          {
            text: "Keep draft",
            style: "cancel",
            onPress: () => {
              const rebased = rebaseWikiDraftAfterConflict(
                { path, title, content, baseRevision },
                conflict.currentRevisionNumber,
              );
              setPath(rebased.path);
              setTitle(rebased.title);
              setContent(rebased.content);
              setBaseRevision(rebased.baseRevision);
              setConflictRevision(conflict.currentRevisionNumber);
            },
          },
          {
            text: "Reload latest",
            style: "destructive",
            onPress: () => void reloadLatest(),
          },
        ],
      );
    },
    [baseRevision, content, path, reloadLatest, title],
  );

  const onSave = useCallback(() => {
    if (!canSave) return;
    update.mutate(
      buildWikiUpdateBody({ path, title, content, baseRevision }),
      { onSuccess: () => router.back(), onError: handleConflict },
    );
  }, [baseRevision, canSave, content, handleConflict, path, title, update]);

  const onCancel = useCallback(() => {
    if (!dirty) {
      router.back();
      return;
    }
    Alert.alert("Discard changes?", "Your Wiki draft will be lost.", [
      { text: "Keep editing", style: "cancel" },
      { text: "Discard", style: "destructive", onPress: () => router.back() },
    ]);
  }, [dirty]);

  const headerLeft = useCallback(
    () => (
      <Pressable
        onPress={onCancel}
        accessibilityRole="button"
        accessibilityLabel="Cancel Wiki edit"
        className="px-1 py-1"
      >
        <Text className="text-base text-brand">Cancel</Text>
      </Pressable>
    ),
    [onCancel],
  );
  const headerRight = useCallback(
    () => (
      <Pressable
        onPress={onSave}
        disabled={!canSave}
        accessibilityRole="button"
        accessibilityLabel="Save Wiki page"
        className={canSave ? "px-1 py-1" : "px-1 py-1 opacity-40"}
      >
        <Text className="text-base font-semibold text-brand">
          {update.isPending ? "Saving…" : "Save"}
        </Text>
      </Pressable>
    ),
    [canSave, onSave, update.isPending],
  );

  if (detail.error || (!detail.isLoading && !detail.data?.id)) {
    return (
      <WikiErrorState
        message="Failed to load this Wiki page for editing."
        onRetry={() => void detail.refetch()}
      />
    );
  }

  return (
    <KeyboardAvoidingView
      className="flex-1 bg-background"
      behavior={Platform.OS === "ios" ? "padding" : undefined}
    >
      <Stack.Screen options={{ headerLeft, headerRight }} />
      <WikiOfflineNotice />
      {conflictRevision ? (
        <View accessibilityRole="alert" className="mx-4 mt-3 rounded-md bg-warning/15 px-3 py-2">
          <Text className="text-xs text-warning">
            Draft kept. The next save will apply it after revision {conflictRevision}.
          </Text>
        </View>
      ) : null}
      <ScrollView
        contentContainerClassName="gap-5 px-4 pb-10 pt-4"
        keyboardShouldPersistTaps="handled"
        keyboardDismissMode="on-drag"
      >
        {!seeded ? (
          <Text className="text-sm text-muted-foreground">Loading…</Text>
        ) : (
          <>
            <View className="gap-1.5">
              <Text className="text-xs text-muted-foreground">Path</Text>
              <TextField
                value={path}
                onChangeText={setPath}
                autoCapitalize="none"
                autoCorrect={false}
                accessibilityLabel="Wiki page path"
              />
            </View>
            <View className="gap-1.5">
              <Text className="text-xs text-muted-foreground">Title</Text>
              <TextField
                value={title}
                onChangeText={setTitle}
                accessibilityLabel="Wiki page title"
              />
            </View>
            <View className="gap-1.5">
              <Text className="text-xs text-muted-foreground">Markdown</Text>
              <AutosizeTextArea
                value={content}
                onChangeText={setContent}
                minHeight={280}
                maxHeight={560}
                className="rounded-md border border-border bg-secondary/50 px-3 py-3 font-mono"
                accessibilityLabel="Wiki page Markdown"
              />
            </View>
            <Text className="text-xs text-muted-foreground">
              Editing revision {baseRevision}. Saves are conflict-protected.
            </Text>
          </>
        )}
      </ScrollView>
    </KeyboardAvoidingView>
  );
}
