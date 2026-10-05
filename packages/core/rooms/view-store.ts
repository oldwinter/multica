"use client";

import { create } from "zustand";
import { createJSONStorage, persist } from "zustand/middleware";
import { defaultStorage } from "../platform/storage";

export type RoomDetailTab = "transcript" | "outcome" | "activity";

interface RoomViewState {
  readonly detailTab: RoomDetailTab;
  readonly setDetailTab: (tab: RoomDetailTab) => void;
}

export const useRoomViewStore = create<RoomViewState>()(
  persist(
    (set) => ({
      detailTab: "transcript",
      setDetailTab: (detailTab) => set({ detailTab }),
    }),
    {
      name: "multica_room_view",
      storage: createJSONStorage(() => defaultStorage),
      partialize: ({ detailTab }) => ({ detailTab }),
      merge: (persisted, current) => {
        const candidate = (persisted as Partial<RoomViewState> | undefined)?.detailTab;
        return {
          ...current,
          detailTab:
            candidate === "transcript" || candidate === "outcome" || candidate === "activity"
              ? candidate
              : "transcript",
        };
      },
    },
  ),
);
