export interface CameraPoint {
  readonly x: number;
  readonly y: number;
}

export interface CameraState extends CameraPoint {
  readonly scale: number;
}

export interface CameraSize {
  readonly width: number;
  readonly height: number;
}

const MIN_SCALE = 0.35;
const MAX_SCALE = 4;
const FIT_FACTOR = 0.9375;

function clampScale(scale: number) {
  return Math.min(MAX_SCALE, Math.max(MIN_SCALE, scale));
}

export function fitCamera(input: {
  readonly viewport: CameraSize;
  readonly world: CameraSize;
}): CameraState {
  const widthScale = input.viewport.width / Math.max(1, input.world.width);
  const heightScale = input.viewport.height / Math.max(1, input.world.height);
  const scale = clampScale(Math.min(widthScale, heightScale) * FIT_FACTOR);
  return {
    x: (input.viewport.width - input.world.width * scale) / 2,
    y: (input.viewport.height - input.world.height * scale) / 2,
    scale,
  };
}

export function zoomCameraAt(
  camera: CameraState,
  pointer: CameraPoint,
  requestedScale: number,
): CameraState {
  const scale = clampScale(requestedScale);
  const worldX = (pointer.x - camera.x) / camera.scale;
  const worldY = (pointer.y - camera.y) / camera.scale;
  return {
    x: pointer.x - worldX * scale,
    y: pointer.y - worldY * scale,
    scale,
  };
}

export function panCamera(
  camera: CameraState,
  delta: CameraPoint,
): CameraState {
  return { x: camera.x + delta.x, y: camera.y + delta.y, scale: camera.scale };
}

export function isWorldPointVisible(input: {
  readonly point: CameraPoint;
  readonly camera: CameraState;
  readonly viewport: CameraSize;
  readonly margin: number;
}): boolean {
  const screenX = input.point.x * input.camera.scale + input.camera.x;
  const screenY = input.point.y * input.camera.scale + input.camera.y;
  return (
    screenX >= -input.margin &&
    screenY >= -input.margin &&
    screenX <= input.viewport.width + input.margin &&
    screenY <= input.viewport.height + input.margin
  );
}
