import { useMemo, useRef, useState } from "react";
import { Alert, Pressable, ScrollView, View } from "react-native";
import SegmentedControl from "@react-native-segmented-control/segmented-control";
import { router, useLocalSearchParams } from "expo-router";
import { useQuery } from "@tanstack/react-query";
import { AutosizeTextArea } from "@/components/ui/autosize-textarea";
import { Button } from "@/components/ui/button";
import { Text } from "@/components/ui/text";
import { useReviewRoomSynthesis } from "@/data/mutations/rooms";
import { roomDetailOptions } from "@/data/queries/rooms";
import { useWorkspaceStore } from "@/data/workspace-store";
import {
  RoomIdempotencyKeys,
  roomErrorMessage,
  roomReviewCorrection,
} from "@/lib/room-interactions";

const ACTIONS = ["accept", "reject", "correct"] as const;

export default function ReviewRoomSynthesisSheet() {
  const { id, cycleId, memoryRevisionId } = useLocalSearchParams<{
    id: string;
    cycleId: string;
    memoryRevisionId: string;
  }>();
  const wsId = useWorkspaceStore((state) => state.currentWorkspaceId);
  const detail = useQuery(roomDetailOptions(wsId, id));
  const review = useReviewRoomSynthesis(id, cycleId);
  const operationKeys = useRef(new RoomIdempotencyKeys()).current;
  const [selectedIndex, setSelectedIndex] = useState(0);
  const [correction, setCorrection] = useState("");
  const revision = useMemo(
    () =>
      detail.data?.memory_revisions.find(
        (candidate) => candidate.id === memoryRevisionId,
      ),
    [detail.data, memoryRevisionId],
  );
  const action = ACTIONS[selectedIndex] ?? "accept";
  const needsCorrection = action === "correct";
  const canSubmit =
    !!revision &&
    revision.review_status === "pending" &&
    (!needsCorrection || correction.trim().length > 0) &&
    !review.isPending;

  const performSubmit = () => {
    if (!canSubmit || !detail.data || !revision) return;
    const correctedSynthesis = roomReviewCorrection(
      action,
      revision.synthesis,
      correction,
    );
    const fingerprint = JSON.stringify({
      cycleId,
      revisionId: revision.id,
      action,
      expectedMemoryVersion: detail.data.room.memory_version,
      correction: correctedSynthesis,
    });
    review.mutate(
      {
        action,
        expected_memory_version: detail.data.room.memory_version,
        correction: correctedSynthesis,
        idempotency_key: operationKeys.keyFor(`review-${action}`, fingerprint),
      },
      {
        onSuccess: () => {
          operationKeys.complete(`review-${action}`, fingerprint);
          router.back();
        },
        onError: (error) =>
          Alert.alert("Review not saved", roomErrorMessage(error)),
      },
    );
  };

  const submit = () => {
    if (action !== "reject") {
      performSubmit();
      return;
    }
    Alert.alert(
      "Request another synthesis?",
      "This outcome will be rejected and the facilitator can produce a new synthesis from the Room transcript.",
      [
        { text: "Keep outcome", style: "cancel" },
        { text: "Request synthesis", style: "destructive", onPress: performSubmit },
      ],
    );
  };

  return (
    <View className="flex-1 bg-background">
      <View className="flex-row items-center justify-between px-4 pt-4 pb-2">
        <Text className="text-base font-semibold text-foreground">
          Review synthesis
        </Text>
        <Pressable
          onPress={submit}
          disabled={!canSubmit}
          hitSlop={6}
          className={canSubmit ? "px-3 py-1.5" : "px-3 py-1.5 opacity-40"}
        >
          <Text className="text-sm font-semibold text-primary">
            {review.isPending ? "Saving…" : "Submit"}
          </Text>
        </Pressable>
      </View>
      <ScrollView
        className="flex-1"
        contentContainerClassName="px-4 pt-3 pb-8 gap-4"
        keyboardShouldPersistTaps="handled"
      >
        {!revision ? (
          <Text className="text-sm text-destructive">
            This synthesis is no longer available. Dismiss and refresh the Room.
          </Text>
        ) : revision.review_status !== "pending" ? (
          <View className="gap-3">
            <Text className="text-sm text-muted-foreground">
              This synthesis has already been reviewed.
            </Text>
            <Button variant="outline" onPress={() => router.back()}>
              <Text>Done</Text>
            </Button>
          </View>
        ) : (
          <>
            <Text className="text-sm text-foreground" selectable>
              {revision.synthesis.summary || "No summary provided"}
            </Text>
            <SegmentedControl
              values={["Accept", "Request synthesis", "Correct"]}
              selectedIndex={selectedIndex}
              onChange={(event) =>
                setSelectedIndex(event.nativeEvent.selectedSegmentIndex)
              }
              accessibilityLabel="Synthesis review decision"
            />
            {needsCorrection ? (
              <View className="gap-1.5">
                <Text className="text-xs font-medium text-muted-foreground">
                  Corrected summary
                </Text>
                <AutosizeTextArea
                  value={correction}
                  onChangeText={setCorrection}
                  placeholder="State the corrected outcome"
                  minHeight={120}
                  maxHeight={240}
                  className="rounded-md border border-border bg-secondary/50 px-3 py-2"
                  autoFocus
                />
                <Text className="text-xs text-muted-foreground">
                  Facts, decisions, questions, recommendations, and citations
                  remain unchanged.
                </Text>
              </View>
            ) : action === "reject" ? (
              <Text className="text-sm text-muted-foreground">
                Request a fresh synthesis from the existing Room transcript.
                No change request text is stored by the server.
              </Text>
            ) : (
              <Text className="text-sm text-muted-foreground">
                Accepted memory becomes the context for future Room cycles.
              </Text>
            )}
          </>
        )}
      </ScrollView>
    </View>
  );
}
