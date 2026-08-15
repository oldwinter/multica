import { useCallback, useLayoutEffect, useRef, useState } from "react";

const FOLLOW_THRESHOLD_PX = 64;

export function useRoomTranscriptScroll(roomId: string, entryCount: number) {
  const scrollRef = useRef<HTMLDivElement>(null);
  const followsLatestRef = useRef(true);
  const previousRoomIdRef = useRef<string | null>(null);
  const previousEntryCountRef = useRef<number | null>(null);
  const [unseenEntryCount, setUnseenEntryCount] = useState(0);

  useLayoutEffect(() => {
    const roomChanged = previousRoomIdRef.current !== roomId;
    const previousEntryCount = previousEntryCountRef.current;
    const entriesChanged = previousEntryCount !== entryCount;
    previousRoomIdRef.current = roomId;
    previousEntryCountRef.current = entryCount;

    if (roomChanged) {
      followsLatestRef.current = true;
      setUnseenEntryCount(0);
    } else if (
      entriesChanged &&
      !followsLatestRef.current &&
      previousEntryCount !== null &&
      entryCount > previousEntryCount
    ) {
      setUnseenEntryCount((count) => count + entryCount - previousEntryCount);
    }

    const scrollContainer = scrollRef.current;
    if ((!roomChanged && !entriesChanged) || !followsLatestRef.current || !scrollContainer) return;

    scrollContainer.scrollTop = Math.max(
      0,
      scrollContainer.scrollHeight - scrollContainer.clientHeight,
    );
    setUnseenEntryCount(0);
  }, [entryCount, roomId]);

  const onScroll = useCallback(() => {
    const scrollContainer = scrollRef.current;
    if (!scrollContainer) return;
    const followsLatest =
      scrollContainer.scrollHeight -
        scrollContainer.clientHeight -
        scrollContainer.scrollTop <=
      FOLLOW_THRESHOLD_PX;
    followsLatestRef.current = followsLatest;
    if (followsLatest) setUnseenEntryCount(0);
  }, []);

  const scrollToLatest = useCallback(() => {
    const scrollContainer = scrollRef.current;
    if (!scrollContainer) return;
    scrollContainer.scrollTop = Math.max(
      0,
      scrollContainer.scrollHeight - scrollContainer.clientHeight,
    );
    followsLatestRef.current = true;
    scrollContainer.focus({ preventScroll: true });
    setUnseenEntryCount(0);
  }, []);

  return { scrollRef, onScroll, unseenEntryCount, scrollToLatest };
}
