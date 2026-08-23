import { useCallback, useEffect, useState } from "react";
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
import { Button } from "@/components/ui/button";
import { TextField } from "@/components/ui/text-field";
import { AutosizeTextArea } from "@/components/ui/autosize-textarea";
import { WikiErrorState, WikiOfflineNotice } from "@/components/wiki/wiki-states";
import {
  wikiPageDetailOptions,
  wikiPageProposalsOptions,
} from "@/data/queries/wiki";
import {
  useAcceptWikiPageProposal,
  useRejectWikiPageProposal,
  wikiConflictFromError,
} from "@/data/mutations/wiki";
import { useWikiPageRealtime } from "@/data/realtime/use-wiki-page-realtime";
import { useWorkspaceStore } from "@/data/workspace-store";
import { canReviewWikiProposal } from "@/data/wiki-navigation";

export default function ReviewWikiProposal() {
  const { id, proposalId } = useLocalSearchParams<{
    id: string;
    proposalId: string;
  }>();
  const wsId = useWorkspaceStore((s) => s.currentWorkspaceId);
  const detail = useQuery(wikiPageDetailOptions(wsId, id));
  const proposals = useQuery(wikiPageProposalsOptions(wsId, id));
  const proposal = proposals.data?.find((item) => item.id === proposalId);
  const accept = useAcceptWikiPageProposal(id);
  const reject = useRejectWikiPageProposal(id);
  const [path, setPath] = useState("");
  const [title, setTitle] = useState("");
  const [content, setContent] = useState("");
  const [seeded, setSeeded] = useState(false);
  const onRemoteDelete = useCallback(() => router.back(), []);
  useWikiPageRealtime(id, onRemoteDelete);

  useEffect(() => {
    if (!proposal || seeded) return;
    setPath(proposal.proposedPath);
    setTitle(proposal.proposedTitle);
    setContent(proposal.proposedContent);
    setSeeded(true);
  }, [proposal, seeded]);

  const refreshAfterConflict = useCallback(
    () => void Promise.all([detail.refetch(), proposals.refetch()]),
    [detail, proposals],
  );

  const onReviewError = useCallback(
    (error: unknown) => {
      const conflict = wikiConflictFromError(error);
      Alert.alert(
        conflict ? "The page changed" : "Review failed",
        conflict
          ? `The page is now revision ${conflict.currentRevisionNumber}. Your edits remain here; refresh the page state before trying again.`
          : error instanceof Error
            ? error.message
            : "Unknown error",
        conflict ? [{ text: "Refresh", onPress: refreshAfterConflict }] : undefined,
      );
    },
    [refreshAfterConflict],
  );

  const onAccept = useCallback(() => {
    const expected = detail.data?.currentRevisionNumber;
    if (!proposal || !expected || !path.trim()) return;
    Alert.alert(
      "Accept edited proposal?",
      "Your reviewed title, path, and Markdown will become a new append-only Wiki revision.",
      [
        { text: "Cancel", style: "cancel" },
        {
          text: "Accept",
          onPress: () =>
            accept.mutate(
              {
                proposalId: proposal.id,
                expectedRevisionNumber: expected,
                path: path.trim(),
                title: title.trim(),
                content,
              },
              { onSuccess: () => router.back(), onError: onReviewError },
            ),
        },
      ],
    );
  }, [accept, content, detail.data?.currentRevisionNumber, onReviewError, path, proposal, title]);

  const onReject = useCallback(() => {
    if (!proposal) return;
    Alert.alert(
      "Reject Agent proposal?",
      "The current Wiki page will not change. The rejection remains auditable.",
      [
        { text: "Cancel", style: "cancel" },
        {
          text: "Reject",
          style: "destructive",
          onPress: () =>
            reject.mutate(
              { proposalId: proposal.id, reason: "Rejected from mobile review" },
              { onSuccess: () => router.back(), onError: onReviewError },
            ),
        },
      ],
    );
  }, [onReviewError, proposal, reject]);

  const canAccept =
    proposal !== undefined &&
    canReviewWikiProposal(proposal.status) &&
    seeded &&
    path.trim().length > 0 &&
    !accept.isPending &&
    !reject.isPending;
  const headerRight = useCallback(
    () => (
      <Pressable
        onPress={onAccept}
        disabled={!canAccept}
        accessibilityRole="button"
        accessibilityLabel="Accept reviewed Wiki proposal"
        className={canAccept ? "px-1 py-1" : "px-1 py-1 opacity-40"}
      >
        <Text className="text-base font-semibold text-brand">
          {accept.isPending ? "Accepting…" : "Accept"}
        </Text>
      </Pressable>
    ),
    [accept.isPending, canAccept, onAccept],
  );

  if (detail.isLoading || proposals.isLoading) {
    return <Text className="flex-1 p-4 text-sm text-muted-foreground">Loading…</Text>;
  }
  if (detail.error || proposals.error || !proposal) {
    return (
      <WikiErrorState
        message="Failed to load this Agent proposal."
        onRetry={refreshAfterConflict}
      />
    );
  }
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
        <View className="gap-1 rounded-md bg-secondary/50 p-3">
          <Text className="text-xs font-medium text-muted-foreground">Agent rationale</Text>
          <Text className="text-sm text-foreground" selectable>
            {proposal.rationale || "No rationale provided."}
          </Text>
          <Text className="text-xs text-muted-foreground">
            Based on revision {proposal.baseRevisionNumber} · {proposal.evidenceRefs.length} evidence reference{proposal.evidenceRefs.length === 1 ? "" : "s"}
          </Text>
        </View>
        <View className="gap-1.5">
          <Text className="text-xs text-muted-foreground">Reviewed path</Text>
          <TextField
            value={path}
            onChangeText={setPath}
            autoCapitalize="none"
            autoCorrect={false}
            accessibilityLabel="Reviewed Wiki page path"
          />
        </View>
        <View className="gap-1.5">
          <Text className="text-xs text-muted-foreground">Reviewed title</Text>
          <TextField
            value={title}
            onChangeText={setTitle}
            accessibilityLabel="Reviewed Wiki page title"
          />
        </View>
        <View className="gap-1.5">
          <Text className="text-xs text-muted-foreground">Reviewed Markdown</Text>
          <AutosizeTextArea
            value={content}
            onChangeText={setContent}
            minHeight={280}
            maxHeight={560}
            className="rounded-md border border-border bg-secondary/50 px-3 py-3 font-mono"
            accessibilityLabel="Reviewed Wiki page Markdown"
          />
        </View>
        {canReviewWikiProposal(proposal.status) ? (
          <Button
            variant="outline"
            disabled={accept.isPending || reject.isPending}
            onPress={onReject}
            accessibilityLabel="Reject Agent proposal"
          >
            <Text>Reject proposal</Text>
          </Button>
        ) : (
          <Text className="text-sm text-muted-foreground">
            This proposal has already been {proposal.status}.
          </Text>
        )}
      </ScrollView>
    </KeyboardAvoidingView>
  );
}
