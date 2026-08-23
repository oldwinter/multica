// @vitest-environment node

import { describe, expect, it } from "vitest";
import { createIdempotencyRegistry, operationFingerprint } from "./idempotency";

describe("Rooms idempotency registry", () => {
  it("reuses a review key after an unknown network result", async () => {
    let sequence = 0;
    const registry = createIdempotencyRegistry(() => `key-${++sequence}`);
    const fingerprint = operationFingerprint("review", {
      roomId: "room-1",
      cycleId: "cycle-3",
      action: "accept",
      expectedMemoryVersion: 3,
    });

    const firstKey = registry.keyFor(fingerprint);
    await expect(Promise.reject(new Error("network result unknown"))).rejects.toThrow();

    expect(registry.keyFor(fingerprint)).toBe(firstKey);
    registry.complete(fingerprint);
    expect(registry.keyFor(fingerprint)).toBe("key-2");
  });

  it("rotates when the promotion payload changes", () => {
    let sequence = 0;
    const registry = createIdempotencyRegistry(() => `key-${++sequence}`);
    const original = operationFingerprint("promote", { title: "Ship", body: "A" });
    const edited = operationFingerprint("promote", { title: "Ship", body: "B" });

    expect(registry.keyFor(original)).toBe("key-1");
    expect(registry.keyFor(edited)).toBe("key-2");
  });

  it("is stable across object key order", () => {
    expect(operationFingerprint("review", { action: "accept", version: 2 }))
      .toBe(operationFingerprint("review", { version: 2, action: "accept" }));
  });
});
