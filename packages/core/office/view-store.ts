"use client";

import { create } from "zustand";
import { createJSONStorage, persist } from "zustand/middleware";
import { defaultStorage } from "../platform/storage";
import {
  createWorkspaceAwareStorage,
  registerForWorkspaceRehydration,
} from "../platform/workspace-storage";
import type { OfficeWorldId } from "./types";

interface OfficeViewState {
  readonly world: OfficeWorldId;
  readonly setWorld: (world: OfficeWorldId) => void;
}

function persistedWorld(value: unknown): OfficeWorldId | null {
  if (typeof value !== "object" || value === null || !("world" in value)) {
    return null;
  }
  const world = value.world;
  return world === "studio" || world === "expedition" ? world : null;
}

export const useOfficeViewStore = create<OfficeViewState>()(
  persist(
    (set) => ({
      world: "studio",
      setWorld: (world) => set({ world }),
    }),
    {
      name: "multica_office_view",
      storage: createJSONStorage(() =>
        createWorkspaceAwareStorage(defaultStorage),
      ),
      partialize: ({ world }) => ({ world }),
      merge: (persisted, current) => ({
        ...current,
        world: persistedWorld(persisted) ?? "studio",
      }),
    },
  ),
);

registerForWorkspaceRehydration(() => useOfficeViewStore.persist.rehydrate());
