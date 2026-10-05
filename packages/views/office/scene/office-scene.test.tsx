import { act, render, waitFor } from "@testing-library/react";
import { renderToString } from "react-dom/server";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { OfficeSceneCommit, OfficeSceneHandle } from "./contracts";

const createOfficeScene = vi.fn();

vi.mock("./create-office-scene", () => ({ createOfficeScene }));

import { OfficeScene } from "./office-scene";

function commit(world: "studio" | "expedition" = "studio"): OfficeSceneCommit {
  return {
    world,
    snapshot: {
      agents: [],
      squads: [],
      activeIssues: [],
      overflow: { agents: 0, squads: 0, activeIssues: 0 },
    },
    selected: null,
    selectedSquadAgentIds: [],
    mode: "replace",
    effects: [],
    reducedMotion: false,
    motionFrozen: false,
  };
}

describe("OfficeScene", () => {
  beforeEach(() => {
    createOfficeScene.mockReset();
  });

  it("does not initialize the renderer during SSR", () => {
    const html = renderToString(
      <OfficeScene
        commit={commit()}
        onSelect={() => {}}
        onStatus={() => {}}
        onCameraControlsChange={() => {}}
      />,
    );

    expect(html).toContain("data-office-scene");
    expect(createOfficeScene).not.toHaveBeenCalled();
  });

  it("reconciles the latest commit and destroys the private handle on unmount", async () => {
    const handle: OfficeSceneHandle = {
      reconcile: vi.fn(),
      fit: vi.fn(),
      zoomIn: vi.fn(),
      zoomOut: vi.fn(),
      destroy: vi.fn(),
    };
    createOfficeScene.mockResolvedValue(handle);
    const onSelect = vi.fn();
    const onStatus = vi.fn();
    const onCameraControlsChange = vi.fn();
    const view = render(
      <OfficeScene
        commit={commit()}
        onSelect={onSelect}
        onStatus={onStatus}
        onCameraControlsChange={onCameraControlsChange}
      />,
    );
    await waitFor(() => expect(createOfficeScene).toHaveBeenCalledOnce());
    await waitFor(() => expect(handle.reconcile).toHaveBeenCalledWith(commit()));
    const controls = onCameraControlsChange.mock.calls.at(-1)?.[0];
    expect(controls).toEqual({
      fit: expect.any(Function),
      zoomIn: expect.any(Function),
      zoomOut: expect.any(Function),
    });
    controls?.fit();
    controls?.zoomIn();
    controls?.zoomOut();
    expect(handle.fit).toHaveBeenCalledOnce();
    expect(handle.zoomIn).toHaveBeenCalledOnce();
    expect(handle.zoomOut).toHaveBeenCalledOnce();

    view.rerender(
      <OfficeScene
        commit={commit("expedition")}
        onSelect={onSelect}
        onStatus={onStatus}
        onCameraControlsChange={onCameraControlsChange}
      />,
    );
    expect(handle.reconcile).toHaveBeenLastCalledWith(commit("expedition"));
    view.unmount();
    expect(onCameraControlsChange).toHaveBeenLastCalledWith(null);
    expect(handle.destroy).toHaveBeenCalledOnce();
  });

  it("reports unsupported fallback if dynamic initialization rejects", async () => {
    createOfficeScene.mockRejectedValue(new Error("renderer unavailable"));
    const onStatus = vi.fn();
    await act(async () => {
      render(
        <OfficeScene
          commit={commit()}
          onSelect={() => {}}
          onStatus={onStatus}
          onCameraControlsChange={() => {}}
        />,
      );
    });

    await waitFor(() =>
      expect(onStatus).toHaveBeenCalledWith({
        kind: "fallback",
        reason: "unsupported",
      }),
    );
    expect(document.querySelector("[data-office-scene]")).not.toBeNull();
  });
});
