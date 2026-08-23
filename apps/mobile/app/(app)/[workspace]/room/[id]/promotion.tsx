import { useEffect, useMemo, useRef, useState } from "react";
import type { ReactNode } from "react";
import { Alert, ScrollView, View } from "react-native";
import { router, useLocalSearchParams } from "expo-router";
import { useQuery } from "@tanstack/react-query";
import { AutosizeTextArea } from "@/components/ui/autosize-textarea";
import { Button } from "@/components/ui/button";
import { TextField } from "@/components/ui/text-field";
import { Text } from "@/components/ui/text";
import {
  usePromoteRoomRecommendation,
  useRejectRoomRecommendation,
} from "@/data/mutations/rooms";
import { roomDetailOptions } from "@/data/queries/rooms";
import { useWorkspaceStore } from "@/data/workspace-store";
import {
  canPromoteRoomRevision,
  RoomIdempotencyKeys,
  recommendationStatus,
  roomErrorMessage,
} from "@/lib/room-interactions";

export default function RoomPromotionSheet() {
  const { id, memoryRevisionId, recommendationKey } = useLocalSearchParams<{
    id: string;
    memoryRevisionId: string;
    recommendationKey: string;
  }>();
  const wsId = useWorkspaceStore((state) => state.currentWorkspaceId);
  const detail = useQuery(roomDetailOptions(wsId, id));
  const promote = usePromoteRoomRecommendation(id);
  const reject = useRejectRoomRecommendation(id);
  const operationKeys = useRef(new RoomIdempotencyKeys()).current;
  const revision = useMemo(
    () =>
      detail.data?.memory_revisions.find(
        (candidate) => candidate.id === memoryRevisionId,
      ),
    [detail.data, memoryRevisionId],
  );
  const recommendation = useMemo(
    () =>
      revision?.synthesis.recommendations.find(
        (candidate) => candidate.key === recommendationKey,
      ),
    [revision, recommendationKey],
  );
  const existingStatus = detail.data
    ? recommendationStatus(
        detail.data.recommendation_reviews,
        memoryRevisionId,
        recommendationKey,
      )
    : null;
  const [title, setTitle] = useState("");
  const [body, setBody] = useState("");
  const [rationale, setRationale] = useState("");
  const [seededKey, setSeededKey] = useState<string | null>(null);

  useEffect(() => {
    if (!recommendation || seededKey === recommendation.key) return;
    setTitle(recommendation.title);
    setBody(recommendation.body);
    setRationale(recommendation.rationale);
    setSeededKey(recommendation.key);
  }, [recommendation, seededKey]);

  const reviewed = existingStatus === "approved" || existingStatus === "rejected";
  const synthesisAccepted = revision
    ? canPromoteRoomRevision(revision.review_status)
    : false;
  const promotionKind =
    recommendation?.kind === "issue" ||
    recommendation?.kind === "wiki" ||
    recommendation?.kind === "decision"
      ? recommendation.kind
      : null;
  const canApprove =
    !!recommendation &&
    !!promotionKind &&
    synthesisAccepted &&
    !reviewed &&
    title.trim().length > 0 &&
    body.trim().length > 0 &&
    !promote.isPending &&
    !reject.isPending;

  const approve = () => {
    if (!canApprove || !recommendation || !promotionKind) return;
    const fingerprint = JSON.stringify({
      memoryRevisionId,
      recommendationKey: recommendation.key,
      kind: promotionKind,
      title: title.trim(),
      body: body.trim(),
      rationale: rationale.trim(),
      citationEntryIds: recommendation.citation_entry_ids,
    });
    promote.mutate(
      {
        kind: promotionKind,
        memory_revision_id: memoryRevisionId,
        recommendation_key: recommendation.key,
        idempotency_key: operationKeys.keyFor(
          "recommendation-approve",
          fingerprint,
        ),
        title: title.trim(),
        body: body.trim(),
        rationale: rationale.trim() || undefined,
        citation_entry_ids: recommendation.citation_entry_ids,
      },
      {
        onSuccess: () => {
          operationKeys.complete("recommendation-approve", fingerprint);
          router.back();
        },
        onError: (error) =>
          Alert.alert("Recommendation not promoted", roomErrorMessage(error)),
      },
    );
  };

  const confirmReject = () => {
    if (!recommendation || reviewed || reject.isPending) return;
    const fingerprint = `${memoryRevisionId}:${recommendation.key}`;
    Alert.alert(
      "Reject recommendation?",
      "The synthesis remains accepted, but this artifact will not be created.",
      [
        { text: "Keep", style: "cancel" },
        {
          text: "Reject",
          style: "destructive",
          onPress: () =>
            reject.mutate(
              {
                memoryRevisionId,
                recommendationKey: recommendation.key,
                idempotencyKey: operationKeys.keyFor(
                  "recommendation-reject",
                  fingerprint,
                ),
              },
              {
                onSuccess: () => {
                  operationKeys.complete("recommendation-reject", fingerprint);
                  router.back();
                },
                onError: (error) =>
                  Alert.alert("Recommendation not rejected", roomErrorMessage(error)),
              },
            ),
        },
      ],
    );
  };

  return (
    <View className="flex-1 bg-background">
      <View className="px-4 pt-4 pb-2 gap-1">
        <Text className="text-base font-semibold text-foreground">
          Review recommendation
        </Text>
        <Text className="text-xs text-muted-foreground capitalize">
          {recommendation?.kind ?? "Artifact"}
        </Text>
      </View>
      <ScrollView
        className="flex-1"
        contentContainerClassName="px-4 pt-3 pb-8 gap-4"
        keyboardShouldPersistTaps="handled"
      >
        {!recommendation ? (
          <Text className="text-sm text-destructive">
            This recommendation is no longer available. Dismiss and refresh the
            Room.
          </Text>
        ) : recommendation.kind === "unknown" ? (
          <View className="gap-3">
            <Text className="text-sm text-muted-foreground">
              This recommendation uses an unsupported artifact type. Refresh
              the app before reviewing it.
            </Text>
            <Button variant="outline" onPress={() => router.back()}>
              <Text>Back to Room</Text>
            </Button>
          </View>
        ) : reviewed ? (
          <View className="gap-3">
            <Text className="text-sm text-muted-foreground capitalize">
              This recommendation is already {existingStatus}.
            </Text>
            <Button variant="outline" onPress={() => router.back()}>
              <Text>Done</Text>
            </Button>
          </View>
        ) : !synthesisAccepted ? (
          <View className="gap-3">
            <Text className="text-sm text-muted-foreground">
              Review and accept the synthesis before promoting an artifact.
            </Text>
            <Button variant="outline" onPress={() => router.back()}>
              <Text>Back to Room</Text>
            </Button>
          </View>
        ) : (
          <>
            <Field label="Title">
              <TextField value={title} onChangeText={setTitle} />
            </Field>
            <Field label="Body">
              <AutosizeTextArea
                value={body}
                onChangeText={setBody}
                minHeight={140}
                maxHeight={280}
                className="rounded-md border border-border bg-secondary/50 px-3 py-2"
              />
            </Field>
            <Field label="Rationale">
              <AutosizeTextArea
                value={rationale}
                onChangeText={setRationale}
                minHeight={72}
                maxHeight={160}
                className="rounded-md border border-border bg-secondary/50 px-3 py-2"
              />
            </Field>
            <Text className="text-xs text-muted-foreground">
              {recommendation.citation_entry_ids.length} cited transcript source
              {recommendation.citation_entry_ids.length === 1 ? "" : "s"}
            </Text>
            <Button onPress={approve} disabled={!canApprove}>
              <Text>{promote.isPending ? "Promoting…" : "Approve and create"}</Text>
            </Button>
            <Button
              variant="outline"
              onPress={confirmReject}
              disabled={reject.isPending || promote.isPending}
            >
              <Text>{reject.isPending ? "Rejecting…" : "Reject recommendation"}</Text>
            </Button>
          </>
        )}
      </ScrollView>
    </View>
  );
}

function Field({
  label,
  children,
}: {
  label: string;
  children: ReactNode;
}) {
  return (
    <View className="gap-1.5">
      <Text className="text-xs font-medium text-muted-foreground">{label}</Text>
      {children}
    </View>
  );
}
