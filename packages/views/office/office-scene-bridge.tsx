"use client";

import {
  useCallback,
  useEffect,
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import { useWorkspaceId } from "@multica/core/hooks";
import {
  advanceOfficeContinuity,
  createOfficeContinuityState,
  useOfficeTaskCache,
} from "@multica/core/office";
import { useWSReconnect } from "@multica/core/realtime";
import { OfficeScene } from "./scene";
import type {
  OfficeRendererStatus,
  OfficeSceneCommit,
} from "./scene";
import type { OfficeSceneSlotProps } from "./scene-slot";

type ContinuityInput = Parameters<typeof advanceOfficeContinuity>[1];
type ContinuityState = ReturnType<typeof createOfficeContinuityState>;
type ContinuityCommit = ReturnType<typeof advanceOfficeContinuity>["commit"];

interface SceneInput extends OfficeSceneSlotProps {
  readonly continuity: ContinuityInput;
}

interface BridgeFrame {
  readonly input: SceneInput;
  readonly continuityState: ContinuityState;
  readonly commit: OfficeSceneCommit;
}

type ReconnectGate =
  | { readonly kind: "idle" }
  | {
      readonly kind: "pending";
      readonly baselineUpdatedAt: number;
      readonly sawFetch: boolean;
    };

function sameContinuityInput(
  previous: ContinuityInput,
  current: ContinuityInput,
): boolean {
  return (
    previous.workspaceId === current.workspaceId &&
    previous.world === current.world &&
    previous.foreground === current.foreground &&
    previous.stale === current.stale &&
    previous.reconnectEpoch === current.reconnectEpoch &&
    previous.recoveryEpoch === current.recoveryEpoch &&
    previous.reducedMotion === current.reducedMotion &&
    previous.observations === current.observations
  );
}

function sameSubject(
  previous: OfficeSceneSlotProps["selected"],
  current: OfficeSceneSlotProps["selected"],
): boolean {
  return (
    previous === current ||
    (previous !== null &&
      current !== null &&
      previous.kind === current.kind &&
      previous.id === current.id)
  );
}

function sameSceneInput(previous: SceneInput, current: SceneInput): boolean {
  return (
    sameContinuityInput(previous.continuity, current.continuity) &&
    previous.snapshot === current.snapshot &&
    sameSubject(previous.selected, current.selected) &&
    previous.selectedSquadAgentIds === current.selectedSquadAgentIds
  );
}

function sceneCommit(
  input: SceneInput,
  motion: ContinuityCommit,
): OfficeSceneCommit {
  return {
    world: input.world,
    snapshot: input.snapshot,
    selected: input.selected,
    selectedSquadAgentIds: input.selectedSquadAgentIds,
    mode: motion.mode,
    effects: motion.effects,
    reducedMotion: input.reducedMotion,
    motionFrozen: input.motionFrozen,
  };
}

function initialFrame(input: SceneInput): BridgeFrame {
  const result = advanceOfficeContinuity(
    createOfficeContinuityState(),
    input.continuity,
  );
  return {
    input,
    continuityState: result.state,
    commit: sceneCommit(input, result.commit),
  };
}

function useBridgeFrame(input: SceneInput): OfficeSceneCommit {
  const [frame, setFrame] = useState(() => initialFrame(input));

  useLayoutEffect(() => {
    setFrame((current) => {
      if (sameSceneInput(current.input, input)) return current;
      if (sameContinuityInput(current.input.continuity, input.continuity)) {
        return {
          input,
          continuityState: current.continuityState,
          commit: sceneCommit(input, { mode: "transition", effects: [] }),
        };
      }
      const result = advanceOfficeContinuity(
        current.continuityState,
        input.continuity,
      );
      return {
        input,
        continuityState: result.state,
        commit: sceneCommit(input, result.commit),
      };
    });
  }, [input]);

  return frame.commit;
}

function useDocumentForeground(): boolean {
  const [foreground, setForeground] = useState(
    () => typeof document === "undefined" || !document.hidden,
  );
  useEffect(() => {
    const update = () => setForeground(!document.hidden);
    document.addEventListener("visibilitychange", update);
    return () => document.removeEventListener("visibilitychange", update);
  }, []);
  return foreground;
}

export function OfficeSceneBridge(props: OfficeSceneSlotProps) {
  const wsId = useWorkspaceId();
  const taskCache = useOfficeTaskCache(wsId);
  const taskCacheRef = useRef(taskCache);
  taskCacheRef.current = taskCache;
  const foreground = useDocumentForeground();
  const [reconnectEpoch, setReconnectEpoch] = useState(0);
  const [reconnectGate, setReconnectGate] = useState<ReconnectGate>({
    kind: "idle",
  });
  const [recoveryEpoch, setRecoveryEpoch] = useState(0);
  const handleReconnect = useCallback(() => {
    const current = taskCacheRef.current;
    setReconnectEpoch((epoch) => epoch + 1);
    setReconnectGate({
      kind: "pending",
      baselineUpdatedAt: current.dataUpdatedAt,
      sawFetch: current.isFetching,
    });
  }, []);
  useWSReconnect(handleReconnect);
  useEffect(() => {
    setReconnectGate((current) => {
      if (current.kind === "idle") return current;
      if (taskCache.isFetching) {
        return current.sawFetch ? current : { ...current, sawFetch: true };
      }
      if (
        current.sawFetch ||
        taskCache.dataUpdatedAt !== current.baselineUpdatedAt
      ) {
        return { kind: "idle" };
      }
      return current;
    });
  }, [taskCache.dataUpdatedAt, taskCache.isFetching]);
  const reconnecting = reconnectGate.kind === "pending";
  const input = useMemo<SceneInput>(
    () => ({
      ...props,
      motionFrozen: props.motionFrozen || reconnecting,
      continuity: {
        workspaceId: wsId,
        world: props.world,
        foreground,
        stale: props.motionFrozen || reconnecting,
        reconnectEpoch,
        recoveryEpoch,
        reducedMotion: props.reducedMotion,
        observations: taskCache.observations,
      },
    }),
    [
      foreground,
      props,
      reconnectEpoch,
      reconnecting,
      recoveryEpoch,
      taskCache.observations,
      wsId,
    ],
  );
  const commit = useBridgeFrame(input);
  const handleStatus = useCallback(
    (status: OfficeRendererStatus) => {
      switch (status.kind) {
        case "ready":
          props.onWorldReady(status.world);
          return;
        case "recovering":
          setRecoveryEpoch((epoch) => epoch + 1);
          return;
        case "world-switch-failed":
          props.onWorldSwitchFailure(status.retainedWorld);
          return;
        case "fallback":
          props.onRendererFallback();
          return;
        default: {
          const exhaustive: never = status;
          return exhaustive;
        }
      }
    },
    [props],
  );

  return (
    <OfficeScene
      commit={commit}
      onSelect={props.onSelect}
      onStatus={handleStatus}
      onCameraControlsChange={props.onCameraControlsChange}
    />
  );
}
