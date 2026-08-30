import type { OfficeSubjectRef } from "@multica/core/office";
import {
  AnimatedSprite,
  Application,
  Assets,
  Container,
  Graphics,
  Polygon,
  Rectangle,
  Texture,
  type Ticker,
} from "pixi.js";
import type { OfficeDecorElement, OfficeWorldPack } from "../worlds/types";
import {
  fitCamera,
  isWorldPointVisible,
  panCamera,
  zoomCameraAt,
  type CameraState,
} from "./camera";
import type {
  OfficeEffect,
  OfficeAgentSceneState,
  OfficeSceneEntity,
  OfficeScenePlan,
  OfficeScenePort,
} from "./contracts";
import { OFFICE_ACTOR_HOME, sampleOfficeAgentMotion } from "./motion";

interface AgentMotion {
  readonly entityKey: string;
  readonly state: OfficeAgentSceneState;
  readonly staticAlpha: number;
}

interface EntityNode {
  readonly container: Container;
  readonly graphics: Graphics;
  readonly actor: AnimatedSprite | null;
  agentMotion: AgentMotion | null;
}

interface RunningEffect {
  readonly graphic: Graphics;
  readonly from: { readonly x: number; readonly y: number };
  readonly to: { readonly x: number; readonly y: number };
  readonly durationMs: number;
  elapsedMs: number;
}

function subjectOf(entity: OfficeSceneEntity): OfficeSubjectRef | null {
  if (entity.kind === "agent") return { kind: "agent", id: entity.id };
  if (entity.kind === "squad") return { kind: "squad", id: entity.id };
  if (entity.kind === "issue") return { kind: "issue", id: entity.id };
  return null;
}

function regionFor(pack: OfficeWorldPack, entity: OfficeSceneEntity) {
  if (entity.kind === "overflow") return null;
  return pack.hitRegions.find((region) => region.role === entity.kind) ?? null;
}

function colorAt(pack: OfficeWorldPack, index: number) {
  return (
    pack.palette[index % pack.palette.length] ?? pack.lighting.light.ambient
  );
}

type SignalColor =
  | "assignment"
  | "execution"
  | "offline"
  | "online"
  | "queued"
  | "selection"
  | "unknown"
  | "unstable"
  | "working";

const SIGNAL_COLOR_INDEX = {
  studio: {
    assignment: 3,
    execution: 4,
    offline: 5,
    online: 2,
    queued: 4,
    selection: 4,
    unknown: 11,
    unstable: 4,
    working: 3,
  },
  expedition: {
    assignment: 3,
    execution: 10,
    offline: 9,
    online: 4,
    queued: 10,
    selection: 10,
    unknown: 11,
    unstable: 10,
    working: 3,
  },
} satisfies Record<OfficeWorldPack["id"], Record<SignalColor, number>>;

function signalColor(pack: OfficeWorldPack, role: SignalColor) {
  return colorAt(pack, SIGNAL_COLOR_INDEX[pack.id][role]);
}

function drawPolygon(graphics: Graphics, points: readonly number[], color: string) {
  const startX = points[0];
  const startY = points[1];
  if (startX === undefined || startY === undefined) return;
  graphics.moveTo(startX, startY);
  for (let index = 2; index + 1 < points.length; index += 2) {
    graphics.lineTo(points[index] ?? startX, points[index + 1] ?? startY);
  }
  graphics.closePath().fill(color);
}

function drawDecorElement(
  graphics: Graphics,
  element: OfficeDecorElement,
  pack: OfficeWorldPack,
) {
  const color = colorAt(pack, element.color);
  switch (element.kind) {
    case "rect":
      graphics.rect(element.x, element.y, element.width, element.height).fill(color);
      return;
    case "circle":
      graphics.circle(element.x, element.y, element.radius).fill(color);
      return;
    case "polygon":
      drawPolygon(graphics, element.points, color);
      return;
    case "line": {
      const startX = element.points[0];
      const startY = element.points[1];
      if (startX === undefined || startY === undefined) return;
      graphics.moveTo(startX, startY);
      for (let index = 2; index + 1 < element.points.length; index += 2) {
        graphics.lineTo(
          element.points[index] ?? startX,
          element.points[index + 1] ?? startY,
        );
      }
      graphics.stroke({ color, width: element.width });
      return;
    }
    default: {
      const exhaustive: never = element;
      return exhaustive;
    }
  }
}

function drawSegmentedAxis(
  graphics: Graphics,
  from: { readonly x: number; readonly y: number },
  to: { readonly x: number; readonly y: number },
) {
  const distance = Math.abs(to.x - from.x) + Math.abs(to.y - from.y);
  if (distance === 0) return;
  const dash = 12;
  const gap = 8;
  for (let offset = 0; offset < distance; offset += dash + gap) {
    const start = offset / distance;
    const end = Math.min(distance, offset + dash) / distance;
    graphics
      .moveTo(
        from.x + (to.x - from.x) * start,
        from.y + (to.y - from.y) * start,
      )
      .lineTo(
        from.x + (to.x - from.x) * end,
        from.y + (to.y - from.y) * end,
      );
  }
}

function drawSegmentedGridLine(
  graphics: Graphics,
  from: { readonly x: number; readonly y: number },
  to: { readonly x: number; readonly y: number },
) {
  const start = { x: Math.round(from.x), y: Math.round(from.y) };
  const end = { x: Math.round(to.x), y: Math.round(to.y) };
  const middleX = Math.round((start.x + end.x) / 16) * 8;
  const firstCorner = { x: middleX, y: start.y };
  const secondCorner = { x: middleX, y: end.y };
  drawSegmentedAxis(graphics, start, firstCorner);
  drawSegmentedAxis(graphics, firstCorner, secondCorner);
  drawSegmentedAxis(graphics, secondCorner, end);
}

function worldAssetUrls(pack: OfficeWorldPack): readonly string[] {
  return [pack.assets.atlas, pack.map.asset];
}

function motionDisabled(plan: OfficeScenePlan): boolean {
  return plan.reducedMotion || plan.motionFrozen;
}

export class PixiOfficeScenePort implements OfficeScenePort {
  readonly #host: HTMLElement;
  readonly #onSelect: (subject: OfficeSubjectRef) => void;
  readonly #contextLossHandlers = new Set<() => void>();
  readonly #entityNodes = new Map<string, EntityNode>();
  readonly #frameTextures = new Map<string, Texture>();
  readonly #effects: RunningEffect[] = [];
  readonly #domCleanups: Array<() => void> = [];
  #app: Application | null = null;
  #worldRoot: Container | null = null;
  #backgroundLayer: Container | null = null;
  #linkLayer: Container | null = null;
  #entityLayer: Container | null = null;
  #effectLayer: Container | null = null;
  #pack: OfficeWorldPack | null = null;
  #plan: OfficeScenePlan | null = null;
  #camera: CameraState = { x: 0, y: 0, scale: 1 };
  #targetCamera: CameraState = this.#camera;
  #cameraInitialized = false;
  #resizeObserver: ResizeObserver | null = null;
  #themeObserver: MutationObserver | null = null;
  #running = false;
  #destroyed = false;
  #motionElapsedMs = 0;
  #dragPointer: { readonly x: number; readonly y: number } | null = null;

  private constructor(input: {
    readonly host: HTMLElement;
    readonly onSelect: (subject: OfficeSubjectRef) => void;
  }) {
    this.#host = input.host;
    this.#onSelect = input.onSelect;
  }

  static async create(input: {
    readonly host: HTMLElement;
    readonly onSelect: (subject: OfficeSubjectRef) => void;
  }) {
    const port = new PixiOfficeScenePort(input);
    await port.#initializeApplication();
    return port;
  }

  async #initializeApplication() {
    const app = new Application();
    const bounds = this.#host.getBoundingClientRect();
    await app.init({
      antialias: false,
      autoDensity: true,
      autoStart: false,
      backgroundAlpha: 0,
      height: Math.max(1, Math.round(bounds.height)),
      preference: ["webgl"],
      resolution: Math.min(window.devicePixelRatio || 1, 2),
      roundPixels: true,
      sharedTicker: false,
      width: Math.max(1, Math.round(bounds.width)),
    });
    app.ticker.maxFPS = 30;
    app.ticker.add(this.#tick);
    app.stop();
    app.canvas.setAttribute("aria-hidden", "true");
    app.canvas.tabIndex = -1;
    app.canvas.style.display = "block";
    app.canvas.style.height = "100%";
    app.canvas.style.imageRendering = "pixelated";
    app.canvas.style.width = "100%";
    this.#host.replaceChildren(app.canvas);
    this.#app = app;
    this.#createLayers(app);
    this.#bindCanvas(app.canvas);
    this.#bindResize();
    this.#bindTheme();
  }

  #createLayers(app: Application) {
    const worldRoot = new Container();
    const backgroundLayer = new Container();
    const linkLayer = new Container();
    const entityLayer = new Container();
    const effectLayer = new Container();
    worldRoot.addChild(backgroundLayer, linkLayer, entityLayer, effectLayer);
    app.stage.addChild(worldRoot);
    this.#worldRoot = worldRoot;
    this.#backgroundLayer = backgroundLayer;
    this.#linkLayer = linkLayer;
    this.#entityLayer = entityLayer;
    this.#effectLayer = effectLayer;
  }

  #bindResize() {
    if (this.#resizeObserver || typeof ResizeObserver === "undefined") return;
    this.#resizeObserver = new ResizeObserver(() => this.#resize());
    this.#resizeObserver.observe(this.#host);
    this.#resize();
  }

  #bindTheme() {
    if (this.#themeObserver || typeof MutationObserver === "undefined") return;
    this.#themeObserver = new MutationObserver(() => this.#drawWorld());
    this.#themeObserver.observe(document.documentElement, {
      attributeFilter: ["class", "data-theme", "data-appearance"],
      attributes: true,
    });
  }

  #bindCanvas(canvas: HTMLCanvasElement) {
    const onContextLost = (event: Event) => {
      event.preventDefault();
      for (const handler of this.#contextLossHandlers) handler();
    };
    const onWheel = (event: WheelEvent) => {
      event.preventDefault();
      const bounds = canvas.getBoundingClientRect();
      const pointer = { x: event.clientX - bounds.left, y: event.clientY - bounds.top };
      const factor = event.deltaY < 0 ? 1.12 : 1 / 1.12;
      this.#camera = zoomCameraAt(
        this.#camera,
        pointer,
        this.#camera.scale * factor,
      );
      this.#targetCamera = this.#camera;
      this.#applyCamera();
    };
    const onPointerDown = (event: PointerEvent) => {
      if (event.button !== 0) return;
      this.#dragPointer = { x: event.clientX, y: event.clientY };
      canvas.setPointerCapture?.(event.pointerId);
    };
    const onPointerMove = (event: PointerEvent) => {
      const previous = this.#dragPointer;
      if (!previous) return;
      this.#camera = panCamera(this.#camera, {
        x: event.clientX - previous.x,
        y: event.clientY - previous.y,
      });
      this.#targetCamera = this.#camera;
      this.#dragPointer = { x: event.clientX, y: event.clientY };
      this.#applyCamera();
    };
    const endDrag = () => {
      this.#dragPointer = null;
    };
    const onDoubleClick = () => this.#fit();
    canvas.addEventListener("webglcontextlost", onContextLost);
    canvas.addEventListener("wheel", onWheel, { passive: false });
    canvas.addEventListener("pointerdown", onPointerDown);
    window.addEventListener("pointermove", onPointerMove);
    window.addEventListener("pointerup", endDrag);
    window.addEventListener("pointercancel", endDrag);
    canvas.addEventListener("dblclick", onDoubleClick);
    this.#domCleanups.push(() => {
      canvas.removeEventListener("webglcontextlost", onContextLost);
      canvas.removeEventListener("wheel", onWheel);
      canvas.removeEventListener("pointerdown", onPointerDown);
      window.removeEventListener("pointermove", onPointerMove);
      window.removeEventListener("pointerup", endDrag);
      window.removeEventListener("pointercancel", endDrag);
      canvas.removeEventListener("dblclick", onDoubleClick);
    });
  }

  #resize() {
    const app = this.#app;
    if (!app) return;
    const bounds = this.#host.getBoundingClientRect();
    app.renderer.resize(
      Math.max(1, Math.round(bounds.width)),
      Math.max(1, Math.round(bounds.height)),
      Math.min(window.devicePixelRatio || 1, 2),
    );
    if (this.#plan) this.#fit();
  }

  #fit() {
    const app = this.#app;
    const plan = this.#plan;
    if (!app || !plan) return;
    this.#camera = fitCamera({
      viewport: { width: app.screen.width, height: app.screen.height },
      world: { width: plan.width, height: plan.height },
    });
    this.#targetCamera = this.#camera;
    this.#cameraInitialized = true;
    this.#applyCamera();
  }

  fit(): void {
    this.#fit();
  }

  zoomIn(): void {
    this.#zoomBy(1.2);
  }

  zoomOut(): void {
    this.#zoomBy(1 / 1.2);
  }

  #zoomBy(factor: number) {
    const app = this.#app;
    if (!app) return;
    this.#camera = zoomCameraAt(
      this.#camera,
      { x: app.screen.width / 2, y: app.screen.height / 2 },
      this.#camera.scale * factor,
    );
    this.#targetCamera = this.#camera;
    this.#applyCamera();
  }

  #applyCamera() {
    const root = this.#worldRoot;
    if (!root) return;
    root.position.set(Math.round(this.#camera.x), Math.round(this.#camera.y));
    root.scale.set(this.#camera.scale);
    this.#updateCulling();
  }

  async installWorld(pack: OfficeWorldPack): Promise<void> {
    const nextFrames = new Map<string, Texture>();
    const loadedAssets: string[] = [];
    try {
      this.#throwIfDestroyed();
      const atlas = await Assets.load<Texture>(pack.assets.atlas);
      loadedAssets.push(pack.assets.atlas);
      this.#throwIfDestroyed();
      await Assets.load(pack.map.asset);
      loadedAssets.push(pack.map.asset);
      this.#throwIfDestroyed();
      atlas.source.scaleMode = "nearest";
      for (const [name, frame] of Object.entries(pack.assets.frames)) {
        nextFrames.set(
          name,
          new Texture({
            source: atlas.source,
            frame: new Rectangle(frame.x, frame.y, frame.width, frame.height),
          }),
        );
      }
    } catch (error) {
      for (const texture of nextFrames.values()) texture.destroy(false);
      await this.#unloadAssets(loadedAssets);
      throw error;
    }

    const previousPack = this.#pack;
    this.#clearWorld();
    this.#pack = pack;
    this.#plan = null;
    this.#motionElapsedMs = 0;
    for (const [name, texture] of nextFrames) this.#frameTextures.set(name, texture);
    this.#cameraInitialized = false;
    this.#drawWorld();
    if (previousPack && previousPack.id !== pack.id) {
      await this.#unloadAssets(worldAssetUrls(previousPack));
    }
  }

  #throwIfDestroyed() {
    if (this.#destroyed) throw new Error("Office renderer is disposed");
  }

  async #unloadAssets(assets: readonly string[]) {
    if (assets.length === 0) return;
    try {
      await Assets.unload([...assets]);
    } catch {
      // A failed unload must not turn a committed pack switch into a failure.
    }
  }

  #clearWorld() {
    this.cancelEffects();
    this.#entityLayer
      ?.removeChildren()
      .forEach((child) => child.destroy({ children: true }));
    this.#entityNodes.clear();
    this.#linkLayer?.removeChildren().forEach((child) => child.destroy());
    this.#backgroundLayer?.removeChildren().forEach((child) => child.destroy());
    for (const texture of this.#frameTextures.values()) texture.destroy(false);
    this.#frameTextures.clear();
  }

  #isDark() {
    const root = document.documentElement;
    return (
      root.classList.contains("dark") ||
      root.dataset.theme === "dark" ||
      root.dataset.appearance === "dark"
    );
  }

  #drawWorld() {
    const layer = this.#backgroundLayer;
    const pack = this.#pack;
    if (!layer || !pack) return;
    layer.removeChildren().forEach((child) => child.destroy());
    const width = pack.map.width * pack.map.tileSize;
    const height = pack.map.height * pack.map.tileSize;
    const lighting = this.#isDark() ? pack.lighting.dark : pack.lighting.light;
    const geometry = new Graphics();
    geometry
      .rect(-width * 2, -height * 2, width * 5, height * 5)
      .fill(colorAt(pack, pack.visuals.backdropColor));
    for (const element of pack.visuals.decor) {
      drawDecorElement(geometry, element, pack);
    }
    if (lighting.overlayAlpha > 0) {
      geometry
        .rect(-width * 2, -height * 2, width * 5, height * 5)
        .fill({ color: lighting.ambient, alpha: lighting.overlayAlpha });
    }
    layer.addChild(geometry);
  }

  apply(plan: OfficeScenePlan): void {
    const pack = this.#pack;
    const layer = this.#entityLayer;
    if (!pack || !layer) return;
    this.#plan = plan;
    const nextKeys = new Set(plan.entities.map((entity) => entity.key));
    for (const [key, node] of this.#entityNodes) {
      if (nextKeys.has(key)) continue;
      this.#entityNodes.delete(key);
      layer.removeChild(node.container);
      node.container.destroy({ children: true });
    }
    for (const entity of plan.entities) {
      let node = this.#entityNodes.get(entity.key);
      if (!node) {
        node = this.#createEntityNode(entity, pack);
        this.#entityNodes.set(entity.key, node);
        layer.addChild(node.container);
      }
      this.#updateEntityNode(node, entity, pack, motionDisabled(plan));
    }
    this.#drawLinks(plan);
    if (!this.#cameraInitialized) this.#fit();
    this.#frameSelection(plan);
    this.#updateCulling();
  }

  #createEntityNode(entity: OfficeSceneEntity, pack: OfficeWorldPack): EntityNode {
    const container = new Container();
    const graphics = new Graphics();
    container.addChild(graphics);
    let actor: AnimatedSprite | null = null;
    if (entity.kind === "agent") {
      const idleFrames =
        pack.clips.idle.variants[entity.visualVariant.silhouette] ??
        pack.clips.idle.variants[0] ??
        [];
      actor = this.#animatedSprite(idleFrames);
      actor.anchor.set(0.5, 1);
      actor.position.set(OFFICE_ACTOR_HOME.x, OFFICE_ACTOR_HOME.y);
      actor.scale.set(1.08);
      container.addChild(actor);
    }
    const subject = subjectOf(entity);
    const region = regionFor(pack, entity);
    if (subject && region) {
      container.eventMode = "static";
      container.cursor = "pointer";
      container.hitArea = new Polygon([...region.polygon]);
      container.on("pointertap", () => this.#onSelect(subject));
    }
    return { container, graphics, actor, agentMotion: null };
  }

  #animatedSprite(frameNames: readonly string[]) {
    const textures = frameNames.flatMap((name) => {
      const texture = this.#frameTextures.get(name);
      return texture ? [texture] : [];
    });
    const sprite = new AnimatedSprite({
      textures: textures.length > 0 ? textures : [Texture.WHITE],
      autoPlay: false,
      autoUpdate: false,
      loop: true,
    });
    return sprite;
  }

  #updateEntityNode(
    node: EntityNode,
    entity: OfficeSceneEntity,
    pack: OfficeWorldPack,
    disableMotion: boolean,
  ) {
    node.container.position.set(entity.anchor.x, entity.anchor.y);
    const graphics = node.graphics.clear();
    if (entity.highlighted) {
      const color = signalColor(pack, "selection");
      graphics.circle(0, -4, 36).stroke({ color, width: 3 });
      graphics.rect(-39, -35, 10, 3).fill(color);
      graphics.rect(29, -35, 10, 3).fill(color);
      graphics.rect(-39, 24, 10, 3).fill(color);
      graphics.rect(29, 24, 10, 3).fill(color);
    }
    if (entity.kind === "agent") {
      const bodyIndices =
        pack.id === "studio" ? [8, 2, 5, 3] : [2, 1, 4, 6];
      const accentIndices =
        pack.id === "studio" ? [4, 3, 11, 10] : [3, 10, 8, 11];
      const bodyColor = colorAt(
        pack,
        bodyIndices[entity.visualVariant.body] ?? bodyIndices[0] ?? 0,
      );
      const accentColor = colorAt(
        pack,
        accentIndices[entity.visualVariant.accent] ?? accentIndices[0] ?? 0,
      );
      if (pack.id === "studio") {
        graphics.rect(-31, 9, 62, 21).fill(colorAt(pack, 0));
        graphics.rect(-27, 13, 54, 10).fill(
          entity.state.stationLit ? signalColor(pack, "working") : bodyColor,
        );
        graphics.rect(-21, 16, 24, 4).fill(colorAt(pack, 7));
        graphics.rect(9, 16, 12, 4).fill(accentColor);
        graphics.rect(-25, 25, 7, 5).fill(colorAt(pack, 5));
        graphics.rect(18, 25, 7, 5).fill(colorAt(pack, 5));
      } else {
        drawPolygon(
          graphics,
          [-34, 11, -16, 1, 18, 3, 35, 14, 24, 31, -20, 30],
          colorAt(pack, 5),
        );
        graphics.rect(-24, 10, 48, 10).fill(colorAt(pack, 6));
        graphics.rect(-17, 13, 20, 4).fill(
          entity.state.stationLit ? signalColor(pack, "working") : bodyColor,
        );
        graphics.rect(8, 13, 10, 4).fill(accentColor);
        graphics.rect(-9, 23, 18, 6).fill(colorAt(pack, 9));
      }
      const availabilityColor = signalColor(pack, entity.state.availability);
      if (entity.state.availability === "online") {
        graphics.rect(-31, -31, 9, 9).fill(availabilityColor);
        graphics.rect(-28, -34, 3, 3).fill(availabilityColor);
      } else if (entity.state.availability === "unstable") {
        drawPolygon(graphics, [-32, -22, -27, -34, -20, -22], availabilityColor);
        graphics.rect(-28, -28, 3, 6).fill(colorAt(pack, 7));
      } else if (entity.state.availability === "offline") {
        graphics.circle(-26, -27, 7).stroke({ color: availabilityColor, width: 2 });
        graphics.moveTo(-32, -33).lineTo(-20, -21).stroke({ color: availabilityColor, width: 2 });
      } else {
        graphics.rect(-32, -33, 12, 12).stroke({ color: availabilityColor, width: 2 });
        graphics.rect(-28, -29, 4, 4).fill(availabilityColor);
        graphics.rect(-28, -22, 4, 3).fill(availabilityColor);
      }
      if (entity.state.workload === "working") {
        const count = Math.min(3, Math.max(1, entity.state.runningCount));
        graphics.rect(19, -33, 14, 11).fill(signalColor(pack, "working"));
        graphics.rect(22, -30, 8, 3).fill(colorAt(pack, 7));
        for (let index = 0; index < count; index += 1) {
          graphics.rect(20 + index * 5, -19, 3, 4).fill(signalColor(pack, "working"));
        }
      } else if (entity.state.workload === "queued") {
        const color = signalColor(pack, "queued");
        graphics.moveTo(26, -34).lineTo(34, -26).lineTo(26, -18).lineTo(18, -26).closePath().stroke({ color, width: 2 });
        const count = Math.min(3, Math.max(1, entity.state.queuedCount));
        for (let index = 0; index < count; index += 1) {
          graphics.rect(21 + index * 5, -15, 3, 3).fill(color);
        }
      } else if (entity.state.workload === "unknown") {
        const color = signalColor(pack, "unknown");
        graphics.moveTo(20, -32).lineTo(32, -20).moveTo(32, -32).lineTo(20, -20).stroke({ color, width: 3 });
      } else {
        graphics.rect(20, -25, 13, 3).fill(colorAt(pack, 5));
        graphics.rect(24, -29, 5, 3).fill(colorAt(pack, 5));
      }
      const actor = node.actor;
      if (actor) {
        const clip = pack.clips[entity.state.clip];
        const frameNames =
          clip.variants[entity.visualVariant.silhouette] ??
          clip.variants[0] ??
          [];
        const textures = frameNames.flatMap((name) => {
          const texture = this.#frameTextures.get(name);
          return texture ? [texture] : [];
        });
        if (textures.length > 0) actor.textures = textures;
        actor.animationSpeed = clip.fps / 60;
        actor.loop = clip.loop;
        node.agentMotion = {
          entityKey: entity.key,
          state: entity.state,
          staticAlpha: entity.state.availability === "offline" ? 0.55 : 1,
        };
        if (disableMotion) {
          actor.stop();
          actor.gotoAndStop(0);
        } else {
          actor.play();
        }
        this.#applyAgentMotion(node, disableMotion);
      }
    } else if (entity.kind === "squad") {
      node.agentMotion = null;
      if (pack.id === "studio") {
        graphics.rect(-31, -22, 62, 44).fill(colorAt(pack, 0));
        graphics.rect(-26, -17, 52, 24).fill(colorAt(pack, 8));
        graphics.rect(-21, -12, 22, 5).fill(colorAt(pack, 11));
        graphics.rect(7, -12, 14, 5).fill(colorAt(pack, 3));
        graphics.rect(-21, -2, 42, 4).fill(colorAt(pack, 4));
      } else {
        graphics.rect(-25, 16, 50, 8).fill(colorAt(pack, 5));
        graphics.rect(-3, -34, 6, 51).fill(colorAt(pack, 6));
        drawPolygon(graphics, [2, -31, 29, -18, 2, -4], colorAt(pack, 2));
        drawPolygon(graphics, [7, -25, 24, -18, 7, -10], colorAt(pack, 12));
        graphics.rect(-18, 8, 36, 8).fill(colorAt(pack, 9));
      }
      graphics.rect(-20, 9, Math.min(40, Math.max(2, entity.memberCount * 2)), 4).fill(signalColor(pack, "execution"));
      for (let index = 0; index < entity.previewCount; index += 1) {
        graphics.rect(-11 + index * 10, 16, 5, 5).fill(signalColor(pack, "assignment"));
      }
    } else if (entity.kind === "issue") {
      node.agentMotion = null;
      const fill = entity.resolved
        ? signalColor(pack, "assignment")
        : signalColor(pack, "offline");
      if (pack.id === "studio") {
        drawPolygon(graphics, [0, -15, 15, 0, 0, 15, -15, 0], fill);
        graphics.rect(-4, -4, 8, 8).fill(colorAt(pack, 7));
        graphics.rect(-2, -2, 4, 4).fill(colorAt(pack, 4));
      } else {
        graphics.rect(-2, -8, 4, 25).fill(colorAt(pack, 6));
        drawPolygon(graphics, [0, -18, 13, -6, 0, 6, -13, -6], fill);
        graphics.rect(-8, 15, 16, 5).fill(colorAt(pack, 5));
        graphics.rect(-2, -9, 4, 6).fill(colorAt(pack, 10));
      }
    } else {
      node.agentMotion = null;
      graphics.rect(-20, -13, 40, 26).fill(colorAt(pack, 0));
      const bars = Math.min(8, Math.max(1, entity.count));
      for (let index = 0; index < bars; index += 1) {
        graphics.rect(-15 + index * 4, -7, 2, 14).fill(colorAt(pack, 4));
      }
    }
  }

  #applyAgentMotion(node: EntityNode, disableMotion: boolean) {
    const actor = node.actor;
    const motion = node.agentMotion;
    if (!actor || !motion) return;
    const sample = sampleOfficeAgentMotion({
      entityKey: motion.entityKey,
      elapsedMs: this.#motionElapsedMs,
      motionDisabled: disableMotion,
      state: motion.state,
    });
    actor.position.set(sample.x, sample.y);
    actor.scale.set(sample.scale);
    actor.alpha = motion.staticAlpha * sample.alphaMultiplier;
  }

  #drawLinks(plan: OfficeScenePlan) {
    const layer = this.#linkLayer;
    const pack = this.#pack;
    if (!layer || !pack) return;
    layer.removeChildren().forEach((child) => child.destroy());
    const anchors = new Map(plan.entities.map((entity) => [entity.key, entity.anchor]));
    const graphics = new Graphics();
    for (const link of plan.links) {
      const from = anchors.get(link.from);
      const to = anchors.get(link.to);
      if (!from || !to) continue;
      drawSegmentedGridLine(graphics, from, to);
      graphics.stroke({
        alpha: 0.42,
        color: signalColor(pack, "assignment"),
        width: 2,
      });
    }
    layer.addChild(graphics);
  }

  #frameSelection(plan: OfficeScenePlan) {
    const app = this.#app;
    if (!app) return;
    const highlighted = plan.entities.filter((entity) => entity.highlighted);
    if (highlighted.length === 0) return;
    const center = highlighted.reduce(
      (sum, entity) => ({ x: sum.x + entity.anchor.x, y: sum.y + entity.anchor.y }),
      { x: 0, y: 0 },
    );
    center.x /= highlighted.length;
    center.y /= highlighted.length;
    this.#targetCamera = {
      x: app.screen.width / 2 - center.x * this.#camera.scale,
      y: app.screen.height / 2 - center.y * this.#camera.scale,
      scale: this.#camera.scale,
    };
    if (motionDisabled(plan)) {
      this.#camera = this.#targetCamera;
      this.#applyCamera();
    }
  }

  playEffects(effects: readonly OfficeEffect[]): void {
    const pack = this.#pack;
    const layer = this.#effectLayer;
    if (!pack || !layer || !this.#plan || motionDisabled(this.#plan)) return;
    for (const effect of effects) {
      if (this.#effects.length >= 16) break;
      const target = this.#entityNodes.get(`agent:${effect.agentId}`)?.container.position;
      const dispatch = pack.anchors.dispatch[0];
      if (!target || !dispatch) continue;
      const graphic = new Graphics();
      const color =
        effect.kind === "task-finished" && effect.outcome === "failed"
          ? signalColor(pack, "assignment")
          : signalColor(pack, "execution");
      if (pack.id === "studio") {
        graphic
          .rect(
            effect.kind === "task-started" ? -4 : -6,
            effect.kind === "task-started" ? -4 : -6,
            effect.kind === "task-started" ? 8 : 12,
            effect.kind === "task-started" ? 8 : 12,
          )
          .fill(color);
        graphic.rect(-2, -2, 4, 4).fill(colorAt(pack, 7));
      } else {
        drawPolygon(
          graphic,
          effect.kind === "task-started"
            ? [0, -6, 6, 0, 0, 6, -6, 0]
            : [0, -9, 9, 0, 0, 9, -9, 0],
          color,
        );
        graphic.rect(-2, -2, 4, 4).fill(colorAt(pack, 7));
      }
      layer.addChild(graphic);
      const staysAtTarget = effect.kind === "task-finished";
      this.#effects.push({
        graphic,
        from: staysAtTarget ? { x: target.x, y: target.y } : dispatch,
        to: { x: target.x, y: target.y },
        durationMs: staysAtTarget ? 520 : 760,
        elapsedMs: 0,
      });
    }
  }

  cancelEffects(): void {
    this.#effects.length = 0;
    this.#effectLayer?.removeChildren().forEach((child) => child.destroy());
  }

  readonly #tick = (ticker: Ticker) => {
    if (!this.#running) return;
    if (this.#plan && !motionDisabled(this.#plan)) {
      this.#motionElapsedMs += ticker.deltaMS;
      for (const node of this.#entityNodes.values()) {
        node.actor?.update(ticker);
        this.#applyAgentMotion(node, false);
      }
      this.#advanceEffects(ticker.deltaMS);
      const distance =
        Math.abs(this.#targetCamera.x - this.#camera.x) +
        Math.abs(this.#targetCamera.y - this.#camera.y);
      if (distance > 0.5) {
        this.#camera = {
          x: this.#camera.x + (this.#targetCamera.x - this.#camera.x) * 0.18,
          y: this.#camera.y + (this.#targetCamera.y - this.#camera.y) * 0.18,
          scale: this.#camera.scale + (this.#targetCamera.scale - this.#camera.scale) * 0.18,
        };
        this.#applyCamera();
      }
    }
  };

  #advanceEffects(deltaMs: number) {
    for (let index = this.#effects.length - 1; index >= 0; index -= 1) {
      const effect = this.#effects[index];
      if (!effect) continue;
      effect.elapsedMs += deltaMs;
      const progress = Math.min(1, effect.elapsedMs / effect.durationMs);
      effect.graphic.position.set(
        effect.from.x + (effect.to.x - effect.from.x) * progress,
        effect.from.y + (effect.to.y - effect.from.y) * progress,
      );
      effect.graphic.alpha = progress > 0.7 ? (1 - progress) / 0.3 : 1;
      if (progress >= 1) {
        effect.graphic.removeFromParent();
        effect.graphic.destroy();
        this.#effects.splice(index, 1);
      }
    }
  }

  #updateCulling() {
    const app = this.#app;
    if (!app) return;
    for (const node of this.#entityNodes.values()) {
      node.container.renderable = isWorldPointVisible({
        point: node.container.position,
        camera: this.#camera,
        viewport: { width: app.screen.width, height: app.screen.height },
        margin: 96,
      });
    }
  }

  pause(): void {
    this.#running = false;
    this.#app?.stop();
  }

  resume(): void {
    if (this.#destroyed) return;
    this.#running = true;
    this.#app?.start();
  }

  async rebuild(): Promise<void> {
    if (this.#destroyed) throw new Error("Office renderer is disposed");
    const pack = this.#pack;
    this.#destroyApplication();
    await this.#initializeApplication();
    if (pack) await this.installWorld(pack);
  }

  onContextLoss(handler: () => void): () => void {
    this.#contextLossHandlers.add(handler);
    return () => this.#contextLossHandlers.delete(handler);
  }

  #destroyApplication() {
    for (const cleanup of this.#domCleanups.splice(0)) cleanup();
    this.#clearWorld();
    this.#app?.destroy(
      { removeView: true },
      { children: true, context: true, texture: false, textureSource: false },
    );
    this.#app = null;
    this.#worldRoot = null;
    this.#backgroundLayer = null;
    this.#linkLayer = null;
    this.#entityLayer = null;
    this.#effectLayer = null;
  }

  destroy(): void {
    if (this.#destroyed) return;
    this.#destroyed = true;
    this.#resizeObserver?.disconnect();
    this.#themeObserver?.disconnect();
    this.#resizeObserver = null;
    this.#themeObserver = null;
    const pack = this.#pack;
    this.#destroyApplication();
    if (pack) void this.#unloadAssets(worldAssetUrls(pack));
    this.#pack = null;
    this.#contextLossHandlers.clear();
  }
}

export function createPixiScenePort(input: {
  readonly host: HTMLElement;
  readonly onSelect: (subject: OfficeSubjectRef) => void;
}): Promise<OfficeScenePort> {
  return PixiOfficeScenePort.create(input);
}
