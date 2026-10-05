import { useRef } from "react";
import { createSafeId } from "@multica/core/utils";

export interface IdempotencyRegistry {
  keyFor: (fingerprint: string) => string;
  complete: (fingerprint: string) => void;
  clear: () => void;
}

export function operationFingerprint(
  operation: string,
  payload: unknown,
): string {
  return `${operation}:${stableStringify(payload)}`;
}

export function createIdempotencyRegistry(
  createKey: () => string = createSafeId,
): IdempotencyRegistry {
  const keys = new Map<string, string>();
  return {
    keyFor(fingerprint) {
      const existing = keys.get(fingerprint);
      if (existing) return existing;
      const key = createKey();
      keys.set(fingerprint, key);
      return key;
    },
    complete(fingerprint) {
      keys.delete(fingerprint);
    },
    clear() {
      keys.clear();
    },
  };
}

export function useIdempotencyRegistry(): IdempotencyRegistry {
  const registry = useRef<IdempotencyRegistry | null>(null);
  registry.current ??= createIdempotencyRegistry();
  return registry.current;
}

function stableStringify(value: unknown): string {
  if (Array.isArray(value)) {
    return `[${value.map(stableStringify).join(",")}]`;
  }
  if (value !== null && typeof value === "object") {
    const record = value as Record<string, unknown>;
    return `{${Object.keys(record)
      .sort()
      .map((key) => `${JSON.stringify(key)}:${stableStringify(record[key])}`)
      .join(",")}}`;
  }
  return JSON.stringify(value) ?? "undefined";
}
