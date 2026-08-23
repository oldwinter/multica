// @vitest-environment node

import { readFileSync } from "node:fs";
import type { WSEventType } from "@multica/core/types";
import { describe, expect, it, vi } from "vitest";
import type { WSClient } from "@/data/realtime/ws-client";
import { subscribeRoomEvents } from "./room-events";

const fixtures = JSON.parse(readFileSync(
  new URL("../../../../server/internal/room/testdata/realtime_events.json", import.meta.url).pathname,
  "utf8",
)) as Record<WSEventType, unknown>;

describe("Mobile Room realtime contract", () => {
  it("registers every bounded server event through typed subscriptions", () => {
    const handlers = new Map<string, (payload: unknown) => void>();
    const ws = {
      on: vi.fn((event: string, handler: (payload: unknown) => void) => {
        handlers.set(event, handler);
        return vi.fn();
      }),
    } as unknown as WSClient;
    const onSignal = vi.fn();

    const unsubscribers = subscribeRoomEvents(ws, onSignal);
    for (const [event, payload] of Object.entries(fixtures)) {
      handlers.get(event)?.(payload);
    }

    expect(unsubscribers).toHaveLength(9);
    expect(onSignal).toHaveBeenCalledTimes(9);
    expect(onSignal).toHaveBeenCalledWith({
      roomId: "00000000-0000-0000-0000-000000000001",
      listChanged: true,
    });
  });
});
