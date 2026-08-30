import type { OfficeSubjectRef } from "@multica/core/office";
import type {} from "./pixi-unsafe-eval-types";
import type {
  OfficeRendererStatus,
  OfficeSceneHandle,
} from "./contracts";
import { OfficeSceneRuntime } from "./runtime";

function fallbackHandle(): OfficeSceneHandle {
  return {
    reconcile: () => {},
    fit: () => {},
    zoomIn: () => {},
    zoomOut: () => {},
    destroy: () => {},
  };
}

export async function createOfficeScene(input: {
  readonly host: HTMLElement;
  readonly onSelect: (subject: OfficeSubjectRef) => void;
  readonly onStatus: (status: OfficeRendererStatus) => void;
}): Promise<OfficeSceneHandle> {
  if (typeof window === "undefined" || typeof document === "undefined") {
    input.onStatus({ kind: "fallback", reason: "unsupported" });
    return fallbackHandle();
  }

  try {
    await import("pixi.js/unsafe-eval");
    const { createPixiScenePort } = await import("./pixi-port");
    const port = await createPixiScenePort({
      host: input.host,
      onSelect: input.onSelect,
    });
    const runtime = new OfficeSceneRuntime({
      host: input.host,
      port,
      onStatus: input.onStatus,
    });
    return {
      reconcile: (commit) => runtime.reconcile(commit),
      fit: () => runtime.fit(),
      zoomIn: () => runtime.zoomIn(),
      zoomOut: () => runtime.zoomOut(),
      destroy: () => runtime.destroy(),
    };
  } catch {
    input.onStatus({ kind: "fallback", reason: "unsupported" });
    return fallbackHandle();
  }
}
