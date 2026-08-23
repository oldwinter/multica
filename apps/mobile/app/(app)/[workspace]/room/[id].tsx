import { useCallback, useRef } from "react";
import {
  ActivityIndicator,
  Alert,
  KeyboardAvoidingView,
  Linking,
  Platform,
  RefreshControl,
  ScrollView,
  View,
} from "react-native";
import { SafeAreaView } from "react-native-safe-area-context";
import { Stack, router, useLocalSearchParams } from "expo-router";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { RoomComposer } from "@/components/room/room-composer";
import { RoomDetailSections } from "@/components/room/room-detail-sections";
import { Button } from "@/components/ui/button";
import { IconButton } from "@/components/ui/icon-button";
import { Text } from "@/components/ui/text";
import {
  useCancelRoomCycle,
  usePostRoomMessage,
  useRetryRoomSynthesis,
  useSetRoomStatus,
} from "@/data/mutations/rooms";
import {
  roomDetailOptions,
  roomKeys,
  roomPreflightOptions,
  roomUsageOptions,
} from "@/data/queries/rooms";
import { useRoomRealtime } from "@/data/realtime/use-room-realtime";
import {
  roomDraftKey,
  useRoomDraftsStore,
} from "@/data/stores/room-drafts-store";
import type { RoomArtifact, RoomRecommendation } from "@/data/rooms-types";
import { useWorkspaceStore } from "@/data/workspace-store";
import {
  createRoomIdempotencyKey,
  RoomIdempotencyKeys,
  roomErrorMessage,
  roomMessageWasSaved,
} from "@/lib/room-interactions";
import { latestRoomCycle } from "@/lib/room-selectors";

export default function RoomDetailPage() {
  const { id } = useLocalSearchParams<{ id: string }>();
  const wsId = useWorkspaceStore((state) => state.currentWorkspaceId);
  const wsSlug = useWorkspaceStore((state) => state.currentWorkspaceSlug);
  const qc = useQueryClient();
  const operationKeys = useRef(new RoomIdempotencyKeys()).current;
  const detail = useQuery(roomDetailOptions(wsId, id));
  const preflight = useQuery(roomPreflightOptions(wsId, id));
  const scheduledPreflight = useQuery({
    ...roomPreflightOptions(wsId, id, "schedule"),
    enabled: Boolean(
      wsId && id && detail.data?.room.schedule_interval_minutes,
    ),
  });
  const usage = useQuery(roomUsageOptions(wsId, id));
  const postMessage = usePostRoomMessage(id);
  const setStatus = useSetRoomStatus(id);
  const latestCycleId = latestRoomCycle(detail.data?.cycles ?? [])?.id ?? "";
  const retrySynthesis = useRetryRoomSynthesis(id, latestCycleId);
  const cancelCycle = useCancelRoomCycle(id, latestCycleId);
  const draftKey = roomDraftKey(wsId, id);
  const body = useRoomDraftsStore(
    (state) => state.drafts[draftKey]?.body ?? "",
  );
  const idempotencyKey = useRoomDraftsStore(
    (state) => state.drafts[draftKey]?.idempotencyKey,
  );
  const setDraftBody = useRoomDraftsStore((state) => state.setBody);
  const markDraftSubmitted = useRoomDraftsStore((state) => state.markSubmitted);
  const clearDraft = useRoomDraftsStore((state) => state.clear);

  useRoomRealtime(id);

  const room = detail.data?.room;

  const refresh = useCallback(async () => {
    await Promise.all([
      detail.refetch(),
      preflight.refetch(),
      scheduledPreflight.refetch(),
      usage.refetch(),
      qc.invalidateQueries({ queryKey: roomKeys.list(wsId) }),
    ]);
  }, [detail, preflight, scheduledPreflight, usage, qc, wsId]);

  const confirmStatusChange = useCallback(() => {
    if (!room || room.status === "archived") return;
    const next = room.status === "paused" ? "active" : "paused";
    Alert.alert(
      next === "paused" ? "Pause Room?" : "Resume Room?",
      next === "paused"
        ? "New messages will be saved, but Agents will not run until you resume."
        : "Agents can respond to new messages and scheduled cycles again.",
      [
        { text: "Cancel", style: "cancel" },
        {
          text: next === "paused" ? "Pause" : "Resume",
          onPress: () =>
            setStatus.mutate(next, {
              onError: (error) =>
                Alert.alert("Room status unchanged", roomErrorMessage(error)),
            }),
        },
      ],
    );
  }, [room, setStatus]);

  const confirmArchive = useCallback(() => {
    if (!room || room.status === "archived") return;
    Alert.alert(
      "Archive Room?",
      "The transcript and accepted outcomes stay readable, but no new Agent work can run.",
      [
        { text: "Cancel", style: "cancel" },
        {
          text: "Archive",
          style: "destructive",
          onPress: () =>
            setStatus.mutate("archived", {
              onError: (error) =>
                Alert.alert("Room not archived", roomErrorMessage(error)),
            }),
        },
      ],
    );
  }, [room, setStatus]);

  const send = useCallback(async () => {
    const content = body.trim();
    if (!content) return;
    const key = idempotencyKey ?? createRoomIdempotencyKey("message");
    markDraftSubmitted(draftKey, content);
    try {
      await postMessage.mutateAsync({ body: content, idempotency_key: key });
      clearDraft(draftKey);
    } catch (error) {
      if (roomMessageWasSaved(error)) {
        clearDraft(draftKey);
        Alert.alert("Message saved", roomErrorMessage(error));
        return;
      }
      Alert.alert("Message not sent", roomErrorMessage(error));
    }
  }, [body, idempotencyKey, postMessage, clearDraft, draftKey, markDraftSubmitted]);

  const openReview = useCallback(
    (cycleId: string, memoryRevisionId: string) => {
      if (!wsSlug) return;
      router.push({
        pathname: "/[workspace]/room/[id]/review",
        params: { workspace: wsSlug, id, cycleId, memoryRevisionId },
      });
    },
    [wsSlug, id],
  );

  const openRecommendation = useCallback(
    (memoryRevisionId: string, recommendation: RoomRecommendation) => {
      if (!wsSlug || !recommendation.key) return;
      router.push({
        pathname: "/[workspace]/room/[id]/promotion",
        params: {
          workspace: wsSlug,
          id,
          memoryRevisionId,
          recommendationKey: recommendation.key,
        },
      });
    },
    [wsSlug, id],
  );

  const retry = useCallback(
    (cycleId: string) => {
      if (cycleId !== latestCycleId) return;
      const fingerprint = cycleId;
      retrySynthesis.mutate(operationKeys.keyFor("synthesis-retry", fingerprint), {
        onSuccess: () => operationKeys.complete("synthesis-retry", fingerprint),
        onError: (error) =>
          Alert.alert("Retry not started", roomErrorMessage(error)),
      });
    },
    [latestCycleId, retrySynthesis, operationKeys],
  );

  const cancel = useCallback(
    (cycleId: string) => {
      if (cycleId !== latestCycleId) return;
      Alert.alert(
        "Cancel this cycle?",
        "Completed contributions remain in the transcript. Accepted memory will not change.",
        [
          { text: "Keep running", style: "cancel" },
          {
            text: "Cancel cycle",
            style: "destructive",
            onPress: () => {
              const fingerprint = cycleId;
              cancelCycle.mutate(
                operationKeys.keyFor("cycle-cancel", fingerprint),
                {
                  onSuccess: () =>
                    operationKeys.complete("cycle-cancel", fingerprint),
                  onError: (error) =>
                    Alert.alert("Cycle not cancelled", roomErrorMessage(error)),
                },
              );
            },
          },
        ],
      );
    },
    [latestCycleId, cancelCycle, operationKeys],
  );

  const openArtifact = useCallback(
    (artifact: RoomArtifact) => {
      if (!artifact.target_id) return;
      if (artifact.kind === "issue" && wsSlug) {
        router.push(`/${wsSlug}/issue/${artifact.target_id}`);
        return;
      }
      if (artifact.kind === "wiki" && wsSlug && process.env.EXPO_PUBLIC_WEB_URL) {
        void Linking.openURL(
          `${process.env.EXPO_PUBLIC_WEB_URL}/${wsSlug}/wiki/${artifact.target_id}`,
        );
        return;
      }
      if (artifact.kind === "decision") {
        Alert.alert(
          "Decision recorded",
          "Decisions remain visible in this Room with their source citations.",
        );
      }
    },
    [wsSlug],
  );

  return (
    <SafeAreaView className="flex-1 bg-background" edges={["bottom"]}>
      <Stack.Screen
        options={{
          title: room?.title || "Room",
          headerBackTitle: "Rooms",
          headerRight:
            room && room.status !== "archived"
              ? () => (
                  <View className="flex-row gap-1">
                    <IconButton
                      name={room.status === "paused" ? "play" : "pause"}
                      onPress={confirmStatusChange}
                      disabled={setStatus.isPending}
                      accessibilityLabel={
                        room.status === "paused" ? "Resume Room" : "Pause Room"
                      }
                    />
                    <IconButton
                      name="archive-outline"
                      onPress={confirmArchive}
                      disabled={setStatus.isPending}
                      accessibilityLabel="Archive Room"
                    />
                  </View>
                )
              : undefined,
        }}
      />
      {detail.isLoading ? (
        <View className="flex-1 items-center justify-center">
          <ActivityIndicator />
        </View>
      ) : detail.error || !detail.data || !detail.data.room.id ? (
        <View className="flex-1 items-center justify-center px-6 gap-3">
          <Text className="text-sm text-destructive text-center">
            Failed to load Room: {detail.error?.message ?? "not found"}
          </Text>
          <Button variant="outline" onPress={() => void detail.refetch()}>
            <Text>Retry</Text>
          </Button>
        </View>
      ) : (
        <KeyboardAvoidingView
          className="flex-1"
          behavior={Platform.OS === "ios" ? "padding" : undefined}
          keyboardVerticalOffset={92}
        >
          <ScrollView
            className="flex-1"
            contentContainerClassName="pb-4"
            keyboardDismissMode="on-drag"
            refreshControl={
              <RefreshControl
                refreshing={detail.isRefetching}
                onRefresh={() => void refresh()}
              />
            }
          >
            <RoomDetailSections
              detail={detail.data}
              preflight={preflight.data}
              scheduledPreflight={scheduledPreflight.data}
              usage={usage.data}
              onReview={openReview}
              onRecommendation={openRecommendation}
              onRetrySynthesis={retry}
              onCancelCycle={cancel}
              onOpenArtifact={openArtifact}
            />
          </ScrollView>
          <RoomComposer
            value={body}
            onChangeText={(next) => setDraftBody(draftKey, next)}
            onSend={send}
            disabled={room?.status === "archived"}
            disabledReason={
              room?.status === "archived"
                ? "Archived Rooms are read-only."
                : room?.status === "paused"
                  ? "Paused: your reply will be saved without running Agents."
                  : undefined
            }
          />
        </KeyboardAvoidingView>
      )}
    </SafeAreaView>
  );
}
