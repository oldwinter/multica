import type { OfficeWorldId } from "@multica/core/office";
import type { OfficeWorldPack } from "./types";

export { REQUIRED_MOTION_CLIPS } from "./types";
export type { OfficeWorldPack } from "./types";

const loaders: Record<OfficeWorldId, () => Promise<OfficeWorldPack>> = {
  studio: async () => (await import("./studio/pack")).loadStudioPack(),
  expedition: async () =>
    (await import("./expedition/pack")).loadExpeditionPack(),
};

export function loadOfficeWorldPack(
  world: OfficeWorldId,
): Promise<OfficeWorldPack> {
  return loaders[world]();
}
