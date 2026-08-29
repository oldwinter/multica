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
import type { OfficeWorldPack } from "../worlds/types";
import {
  fitCamera,
  isWorldPointVisible,
  panCamera,
  zoomCameraAt,
  type CameraState,
} from "./camera";
import type {
  OfficeEffect,
  OfficeSceneEntity,
  OfficeScenePlan,
  OfficeScenePort,
} from "./contracts";

interface EntityNode {
  readonly container: Container;
  readonly graphics: Graphics;
  readonly actor: AnimatedSprite | null;
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
  return pack.palette[index % pack.palette.length] ?? "#ffffff";
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
      backgroundAlpha: 1,
      backgroundColor: "#111519",
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
    const atlas = await Assets.load<Texture>(pack.assets.atlas);
    await Assets.load(pack.map.asset);
    atlas.source.scaleMode = "nearest";
    const nextFrames = new Map<string, Texture>();
    for (const [name, frame] of Object.entries(pack.assets.frames)) {
      nextFrames.set(
        name,
        new Texture({
          source: atlas.source,
          frame: new Rectangle(frame.x, frame.y, frame.width, frame.height),
        }),
      );
    }

    const previousPack = this.#pack;
    this.#clearWorld();
    this.#pack = pack;
    for (const [name, texture] of nextFrames) this.#frameTextures.set(name, texture);
    this.#cameraInitialized = false;
    this.#drawWorld();
    if (previousPack && previousPack.id !== pack.id) {
      void Assets.unload([previousPack.assets.atlas, previousPack.map.asset]);
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
    geometry.rect(0, 0, width, height).fill(lighting.ambient);
    if (pack.id === "studio") {
      geometry.rect(48, 48, width - 96, height - 96).fill(colorAt(pack, 1));
      for (let x = 80; x < width - 64; x += 80) {
        geometry.moveTo(x, 64).lineTo(x, height - 64).stroke({ color: colorAt(pack, 5), alpha: 0.24, width: 1 });
      }
      geometry.rect(width * 0.63, 88, width * 0.31, height - 176).fill({ color: colorAt(pack, 0), alpha: 0.88 });
    } else {
      geometry.rect(48, 48, width - 96, height - 96).fill(colorAt(pack, 0));
      geometry.rect(72, height * 0.43, width - 144, height * 0.14).fill(colorAt(pack, 2));
      geometry.rect(width * 0.43, 72, width * 0.14, height - 144).fill(colorAt(pack, 2));
      for (let index = 0; index < 18; index += 1) {
        const x = 90 + ((index * 197) % Math.max(1, width - 180));
        const y = 90 + ((index * 113) % Math.max(1, height - 180));
        geometry.circle(x, y, 18 + (index % 4) * 5).fill({ color: colorAt(pack, 1), alpha: 0.72 });
      }
    }
    if (lighting.overlayAlpha > 0) {
      geometry.rect(0, 0, width, height).fill({ color: lighting.ambient, alpha: lighting.overlayAlpha });
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
      this.#updateEntityNode(node, entity, pack, plan.reducedMotion);
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
      actor = this.#animatedSprite(pack.clips.idle.frames);
      actor.anchor.set(0.5, 1);
      actor.position.set(0, 12);
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
    return { container, graphics, actor };
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
    reducedMotion: boolean,
  ) {
    node.container.position.set(entity.anchor.x, entity.anchor.y);
    const graphics = node.graphics.clear();
    if (entity.highlighted) {
      graphics.circle(0, 0, 27).stroke({ color: colorAt(pack, 4), width: 3 });
    }
    if (entity.kind === "agent") {
      if (pack.id === "studio") {
        graphics.rect(-24, 10, 48, 18).fill(colorAt(pack, 0));
        graphics.rect(-19, 14, 38, 8).fill(
          entity.state.stationLit ? colorAt(pack, 4) : colorAt(pack, 5),
        );
      } else {
        graphics.circle(0, 18, 23).fill(colorAt(pack, 5));
        graphics.circle(0, 18, 14).fill(
          entity.state.stationLit ? colorAt(pack, 3) : colorAt(pack, 1),
        );
      }
      const availabilityColor =
        entity.state.availability === "online"
          ? colorAt(pack, 2)
          : entity.state.availability === "unstable"
            ? colorAt(pack, 4)
            : entity.state.availability === "offline"
              ? colorAt(pack, 5)
              : colorAt(pack, 6);
      graphics.circle(-20, -18, 5).fill(availabilityColor);
      if (entity.state.workload === "working") {
        graphics.rect(14, -22, 10, 10).fill(colorAt(pack, 3));
      } else if (entity.state.workload === "queued") {
        graphics.circle(19, -17, 5).stroke({ color: colorAt(pack, 4), width: 2 });
      } else if (entity.state.workload === "unknown") {
        graphics.moveTo(15, -22).lineTo(23, -14).moveTo(23, -22).lineTo(15, -14).stroke({ color: colorAt(pack, 6), width: 2 });
      }
      const actor = node.actor;
      if (actor) {
        const clip = pack.clips[entity.state.clip];
        const textures = clip.frames.flatMap((name) => {
          const texture = this.#frameTextures.get(name);
          return texture ? [texture] : [];
        });
        if (textures.length > 0) actor.textures = textures;
        actor.animationSpeed = clip.fps / 60;
        actor.loop = clip.loop;
        if (reducedMotion) {
          actor.stop();
          actor.gotoAndStop(0);
        } else {
          actor.play();
        }
      }
    } else if (entity.kind === "squad") {
      if (pack.id === "studio") {
        graphics.rect(-28, -19, 56, 38).fill(colorAt(pack, 0));
        graphics.rect(-22, -12, 44, 8).fill(colorAt(pack, 2));
      } else {
        graphics.moveTo(0, -28).lineTo(24, 18).lineTo(-24, 18).closePath().fill(colorAt(pack, 2));
        graphics.rect(-3, -30, 6, 51).fill(colorAt(pack, 6));
      }
      graphics.rect(-20, 7, Math.min(40, Math.max(2, entity.memberCount * 2)), 4).fill(colorAt(pack, 4));
      for (let index = 0; index < entity.previewCount; index += 1) {
        graphics.circle(-10 + index * 10, 15, 3).fill(colorAt(pack, 3));
      }
    } else if (entity.kind === "issue") {
      const fill = entity.resolved ? colorAt(pack, 3) : colorAt(pack, 5);
      graphics.moveTo(0, -14).lineTo(14, 0).lineTo(0, 14).lineTo(-14, 0).closePath().fill(fill);
      graphics.circle(0, 0, 4).fill(colorAt(pack, 7));
    } else {
      graphics.rect(-20, -13, 40, 26).fill(colorAt(pack, 0));
      const bars = Math.min(8, Math.max(1, entity.count));
      for (let index = 0; index < bars; index += 1) {
        graphics.rect(-15 + index * 4, -7, 2, 14).fill(colorAt(pack, 4));
      }
    }
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
      graphics.moveTo(from.x, from.y).lineTo(to.x, to.y).stroke({
        alpha: link.kind === "assignment" ? 0.72 : 0.42,
        color: link.kind === "assignment" ? colorAt(pack, 3) : colorAt(pack, 2),
        width: link.kind === "assignment" ? 2 : 1,
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
    if (plan.reducedMotion) {
      this.#camera = this.#targetCamera;
      this.#applyCamera();
    }
  }

  playEffects(effects: readonly OfficeEffect[]): void {
    const pack = this.#pack;
    const layer = this.#effectLayer;
    if (!pack || !layer || this.#plan?.reducedMotion) return;
    for (const effect of effects) {
      if (this.#effects.length >= 16) break;
      const target = this.#entityNodes.get(`agent:${effect.agentId}`)?.container.position;
      const dispatch = pack.anchors.dispatch[0];
      if (!target || !dispatch) continue;
      const graphic = new Graphics();
      const color =
        effect.kind === "task-finished" && effect.outcome === "failed"
          ? colorAt(pack, 3)
          : colorAt(pack, 4);
      graphic.circle(0, 0, effect.kind === "task-started" ? 5 : 7).fill(color);
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
    if (!this.#plan?.reducedMotion) {
      for (const node of this.#entityNodes.values()) node.actor?.update(ticker);
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
    if (pack) void Assets.unload([pack.assets.atlas, pack.map.asset]);
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
