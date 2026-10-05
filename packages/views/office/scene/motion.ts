import type { OfficeAgentSceneState } from "./contracts";

export const OFFICE_ACTOR_HOME = { x: 0, y: 13 } as const;

const ACTOR_BASE_SCALE = 1.08;

function stableHash(value: string): number {
  let hash = 0x811c9dc5;
  for (let index = 0; index < value.length; index += 1) {
    hash ^= value.charCodeAt(index);
    hash = Math.imul(hash, 0x01000193);
  }
  return hash >>> 0;
}

function phaseFor(entityKey: string): number {
  return ((stableHash(entityKey) % 360) * Math.PI) / 180;
}

export function sampleOfficeAgentMotion(input: {
  readonly entityKey: string;
  readonly elapsedMs: number;
  readonly motionDisabled: boolean;
  readonly state: Pick<
    OfficeAgentSceneState,
    "ambientMotion" | "pulse" | "flicker"
  >;
}) {
  const neutral = {
    x: OFFICE_ACTOR_HOME.x,
    y: OFFICE_ACTOR_HOME.y,
    scale: ACTOR_BASE_SCALE,
    alphaMultiplier: 1,
  };
  if (input.motionDisabled) return neutral;

  const phase = phaseFor(input.entityKey);
  const ambientPhase = phase + (input.elapsedMs / 2_400) * Math.PI * 2;
  const xOffset = input.state.ambientMotion
    ? Math.round(Math.sin(ambientPhase) * 1.6)
    : 0;
  const yOffset = input.state.ambientMotion
    ? Math.round(Math.cos(ambientPhase * 0.75) * 1.2)
    : 0;
  const pulse = input.state.pulse
    ? (Math.sin(phase + (input.elapsedMs / 1_100) * Math.PI * 2) + 1) / 2
    : 0;
  const flickerStep = Math.floor(input.elapsedMs / 137);
  const flickerValues = [1, 0.96, 0.92, 0.88] as const;
  const alphaMultiplier = input.state.flicker
    ? (flickerValues[
        stableHash(`${input.entityKey}|${flickerStep}`) % flickerValues.length
      ] ?? 1)
    : 1;

  return {
    x: OFFICE_ACTOR_HOME.x + xOffset,
    y: OFFICE_ACTOR_HOME.y + yOffset,
    scale: Math.min(1.1, ACTOR_BASE_SCALE + pulse * 0.02),
    alphaMultiplier,
  };
}
