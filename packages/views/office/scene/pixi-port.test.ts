import { beforeEach, describe, expect, it, vi } from "vitest";

const pixi = vi.hoisted(() => ({
  appDestroy: vi.fn(),
  appInit: vi.fn(),
  load: vi.fn(),
  textureCreate: vi.fn(),
  unload: vi.fn().mockResolvedValue(undefined),
}));

vi.mock("pixi.js", () => {
  class PointLike {
    x = 0;
    y = 0;

    set(x: number, y = x) {
      this.x = x;
      this.y = y;
    }
  }

  class Container {
    readonly position = new PointLike();
    readonly scale = new PointLike();
    readonly children: Container[] = [];

    addChild(...children: Container[]) {
      this.children.push(...children);
    }

    removeChildren() {
      return this.children.splice(0);
    }

    destroy() {}
  }

  class Graphics extends Container {}

  class Texture {
    static readonly WHITE = new Texture({ source: { scaleMode: "nearest" } });
    readonly source: { scaleMode: string };

    constructor(input: { readonly source: { scaleMode: string } }) {
      this.source = input.source;
      pixi.textureCreate();
    }

    destroy() {}
  }

  class Application {
    readonly canvas = document.createElement("canvas");
    readonly renderer = { resize: vi.fn() };
    readonly screen = { width: 800, height: 600 };
    readonly stage = new Container();
    readonly ticker = { add: vi.fn(), maxFPS: 0 };

    async init(options: unknown) {
      pixi.appInit(options);
    }

    start() {}

    stop() {}

    destroy() {
      pixi.appDestroy();
    }
  }

  class AnimatedSprite extends Container {}
  class Polygon {}
  class Rectangle {}

  return {
    AnimatedSprite,
    Application,
    Assets: { load: pixi.load, unload: pixi.unload },
    Container,
    Graphics,
    Polygon,
    Rectangle,
    Texture,
  };
});

import { loadOfficeWorldPack } from "../worlds/world-packs";
import { createPixiScenePort } from "./pixi-port";

function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((done) => {
    resolve = done;
  });
  return { promise, resolve };
}

describe("Pixi office scene asset lifecycle", () => {
  beforeEach(() => {
    pixi.appDestroy.mockClear();
    pixi.appInit.mockClear();
    pixi.load.mockReset();
    pixi.textureCreate.mockClear();
    pixi.unload.mockClear();
  });

  it("keeps the canvas transparent over semantic CSS backgrounds", async () => {
    const host = document.createElement("div");
    const surface = document.createElement("div");
    surface.style.backgroundColor = "oklch(1 0 0)";
    surface.append(host);
    document.body.append(surface);

    const port = await createPixiScenePort({ host, onSelect: () => {} });

    expect(pixi.appInit).toHaveBeenCalledWith(
      expect.objectContaining({ backgroundAlpha: 0 }),
    );
    expect(pixi.appInit.mock.calls[0]?.[0]).not.toHaveProperty(
      "backgroundColor",
    );
    port.destroy();
  });

  it("abandons and unloads a world whose map asset resolves after destroy", async () => {
    const pack = await loadOfficeWorldPack("studio");
    const mapLoad = deferred<unknown>();
    const atlas = { source: { scaleMode: "linear" } };
    pixi.load.mockImplementation((url: string) =>
      url === pack.assets.atlas ? Promise.resolve(atlas) : mapLoad.promise,
    );
    const host = document.createElement("div");
    document.body.append(host);
    const port = await createPixiScenePort({ host, onSelect: () => {} });
    const textureCountBeforeInstall = pixi.textureCreate.mock.calls.length;

    const installing = port.installWorld(pack);
    await vi.waitFor(() =>
      expect(pixi.load).toHaveBeenCalledWith(pack.map.asset),
    );
    port.destroy();
    mapLoad.resolve({});

    await expect(installing).rejects.toThrow("disposed");
    expect(pixi.textureCreate.mock.calls).toHaveLength(
      textureCountBeforeInstall,
    );
    expect(pixi.unload).toHaveBeenCalledWith([
      pack.assets.atlas,
      pack.map.asset,
    ]);
    expect(pixi.appDestroy).toHaveBeenCalledOnce();
  });
});
