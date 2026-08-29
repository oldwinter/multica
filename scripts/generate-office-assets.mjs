#!/usr/bin/env node

import { createHash } from "node:crypto";
import { existsSync, mkdirSync, readFileSync, writeFileSync } from "node:fs";
import { dirname, join, relative, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { deflateSync } from "node:zlib";

const repoRoot = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const worldsRoot = join(repoRoot, "packages/views/office/worlds");
const checkOnly = process.argv.slice(2).includes("--check");
const clipIds = [
  "idle",
  "wait",
  "work",
  "walk",
  "unstable",
  "offline",
  "completion",
  "failure",
];

const worldSpecs = {
  studio: {
    layoutVersion: 1,
    map: { width: 80, height: 48, tileSize: 16 },
    palette: [
      "#20262d",
      "#e2e5df",
      "#5d7564",
      "#e46f61",
      "#e6b655",
      "#91a0a8",
      "#f4f1e8",
      "#111519",
    ],
    lighting: {
      light: { ambient: "#f4f1e8", overlayAlpha: 0.04 },
      dark: { ambient: "#111519", overlayAlpha: 0.28 },
    },
    visuals: {
      actorSilhouette: "compact-human-operator",
      stationStyle: "cantilever-desk-pod",
      props: ["dispatch-console", "project-board", "planter", "server-rack"],
    },
    artBrief:
      "Original top-down modern workplace with concrete bays, graphite desks, green planters, coral signals, amber task packets, and cool-metal service rails.",
  },
  expedition: {
    layoutVersion: 3,
    map: { width: 88, height: 52, tileSize: 16 },
    palette: [
      "#182320",
      "#526c47",
      "#264b78",
      "#eb7868",
      "#8ea092",
      "#30353b",
      "#d7c887",
      "#0c1318",
    ],
    lighting: {
      light: { ambient: "#d7c887", overlayAlpha: 0.06 },
      dark: { ambient: "#0c1318", overlayAlpha: 0.34 },
    },
    visuals: {
      actorSilhouette: "faceted-field-companion",
      stationStyle: "radial-stone-workbench",
      props: ["route-beacon", "canvas-shelter", "moss-crate", "survey-standard"],
    },
    artBrief:
      "Original field base of basalt and moss with cobalt canvas, coral route markers, stone tables, and small faceted geometric companions with diamond ears and angular tails.",
  },
};

function json(value) {
  return `${JSON.stringify(value, null, 2)}\n`;
}

function sha256(value) {
  return createHash("sha256").update(value).digest("hex");
}

function crc32(buffer) {
  let crc = 0xffffffff;
  for (const byte of buffer) {
    crc ^= byte;
    for (let bit = 0; bit < 8; bit += 1) {
      crc = (crc >>> 1) ^ (crc & 1 ? 0xedb88320 : 0);
    }
  }
  return (crc ^ 0xffffffff) >>> 0;
}

function pngChunk(type, data) {
  const name = Buffer.from(type, "ascii");
  const length = Buffer.alloc(4);
  length.writeUInt32BE(data.length);
  const checksum = Buffer.alloc(4);
  checksum.writeUInt32BE(crc32(Buffer.concat([name, data])));
  return Buffer.concat([length, name, data, checksum]);
}

function encodePng(width, height, pixels) {
  const header = Buffer.alloc(13);
  header.writeUInt32BE(width, 0);
  header.writeUInt32BE(height, 4);
  header[8] = 8;
  header[9] = 6;
  const scanlines = Buffer.alloc(height * (width * 4 + 1));
  for (let y = 0; y < height; y += 1) {
    const target = y * (width * 4 + 1);
    scanlines[target] = 0;
    Buffer.from(pixels.buffer, pixels.byteOffset + y * width * 4, width * 4).copy(
      scanlines,
      target + 1,
    );
  }
  return Buffer.concat([
    Buffer.from([137, 80, 78, 71, 13, 10, 26, 10]),
    pngChunk("IHDR", header),
    pngChunk("IDAT", deflateSync(scanlines, { level: 9 })),
    pngChunk("IEND", Buffer.alloc(0)),
  ]);
}

function parseColor(value) {
  const hex = value.startsWith("#") ? value.slice(1) : value;
  return [
    Number.parseInt(hex.slice(0, 2), 16),
    Number.parseInt(hex.slice(2, 4), 16),
    Number.parseInt(hex.slice(4, 6), 16),
    255,
  ];
}

function createCanvas(width, height, background) {
  const pixels = new Uint8Array(width * height * 4);
  const canvas = {
    width,
    height,
    pixels,
    fillRect(x, y, rectWidth, rectHeight, color) {
      const rgba = parseColor(color);
      const left = Math.max(0, Math.floor(x));
      const top = Math.max(0, Math.floor(y));
      const right = Math.min(width, Math.ceil(x + rectWidth));
      const bottom = Math.min(height, Math.ceil(y + rectHeight));
      for (let row = top; row < bottom; row += 1) {
        for (let column = left; column < right; column += 1) {
          const offset = (row * width + column) * 4;
          pixels.set(rgba, offset);
        }
      }
    },
  };
  canvas.fillRect(0, 0, width, height, background);
  return canvas;
}

function scaleCanvas(source, factor) {
  const output = createCanvas(
    source.width * factor,
    source.height * factor,
    "#000000",
  );
  for (let y = 0; y < source.height; y += 1) {
    for (let x = 0; x < source.width; x += 1) {
      const sourceOffset = (y * source.width + x) * 4;
      const color = `#${[0, 1, 2]
        .map((index) => source.pixels[sourceOffset + index].toString(16).padStart(2, "0"))
        .join("")}`;
      output.fillRect(x * factor, y * factor, factor, factor, color);
    }
  }
  return output;
}

function drawStudioPoster() {
  const scene = createCanvas(192, 108, "#20262d");
  scene.fillRect(5, 5, 182, 98, "#e2e5df");
  scene.fillRect(9, 9, 174, 16, "#91a0a8");
  for (let x = 13; x < 179; x += 20) scene.fillRect(x, 12, 13, 9, "#264b78");
  scene.fillRect(88, 29, 4, 67, "#91a0a8");
  scene.fillRect(133, 29, 4, 67, "#91a0a8");
  for (let row = 0; row < 4; row += 1) {
    for (let column = 0; column < 5; column += 1) {
      const x = 15 + column * 14;
      const y = 33 + row * 16;
      scene.fillRect(x, y, 11, 6, "#20262d");
      scene.fillRect(x + 2, y - 3, 7, 3, "#5d7564");
      scene.fillRect(x + 4, y + 7, 3, 4, "#e46f61");
    }
  }
  for (let row = 0; row < 6; row += 1) {
    scene.fillRect(98, 32 + row * 10, 28, 5, row % 2 ? "#5d7564" : "#20262d");
    scene.fillRect(129, 32 + row * 10, 2, 5, "#e6b655");
  }
  for (let row = 0; row < 4; row += 1) {
    for (let column = 0; column < 4; column += 1) {
      scene.fillRect(143 + column * 9, 36 + row * 13, 5, 5, "#e46f61");
      scene.fillRect(145 + column * 9, 41 + row * 13, 1, 5, "#20262d");
    }
  }
  scene.fillRect(72, 86, 12, 8, "#e6b655");
  scene.fillRect(75, 82, 6, 4, "#e46f61");
  return scaleCanvas(scene, 4);
}

function drawExpeditionPoster() {
  const scene = createCanvas(192, 108, "#182320");
  scene.fillRect(5, 5, 182, 98, "#30353b");
  for (let row = 0; row < 12; row += 1) {
    for (let column = 0; column < 20; column += 1) {
      if ((row * 7 + column * 3) % 5 === 0) {
        scene.fillRect(8 + column * 9, 8 + row * 8, 5, 4, "#526c47");
      }
    }
  }
  scene.fillRect(14, 44, 164, 13, "#264b78");
  scene.fillRect(72, 19, 48, 73, "#264b78");
  for (let index = 0; index < 10; index += 1) {
    const x = 28 + (index % 5) * 30;
    const y = 26 + Math.floor(index / 5) * 50;
    scene.fillRect(x, y, 16, 10, "#8ea092");
    scene.fillRect(x + 3, y - 3, 10, 3, "#d7c887");
    scene.fillRect(x + 6, y + 11, 4, 4, "#eb7868");
  }
  for (let row = 0; row < 4; row += 1) {
    for (let column = 0; column < 6; column += 1) {
      const x = 75 + column * 7;
      const y = 34 + row * 12;
      scene.fillRect(x + 2, y, 3, 2, "#d7c887");
      scene.fillRect(x, y + 2, 7, 5, row % 2 ? "#526c47" : "#eb7868");
      scene.fillRect(x + 5, y + 7, 4, 2, "#182320");
    }
  }
  for (let index = 0; index < 7; index += 1) {
    scene.fillRect(16 + index * 25, 51, 3, 3, "#eb7868");
    scene.fillRect(17 + index * 25, 54, 1, 8, "#d7c887");
  }
  return scaleCanvas(scene, 4);
}

function drawAtlas(world) {
  const spec = worldSpecs[world];
  const atlas = createCanvas(256, 128, spec.palette[7]);
  clipIds.forEach((clip, clipIndex) => {
    for (let frame = 0; frame < 2; frame += 1) {
      const index = clipIndex * 2 + frame;
      const x = (index % 8) * 32;
      const y = Math.floor(index / 8) * 32;
      atlas.fillRect(x + 1, y + 1, 30, 30, spec.palette[0]);
      if (world === "studio") {
        atlas.fillRect(x + 11, y + 6 + frame, 10, 8, spec.palette[6]);
        atlas.fillRect(x + 9, y + 14 + frame, 14, 11, spec.palette[2]);
        atlas.fillRect(x + 7, y + 25, 7, 4, spec.palette[5]);
        atlas.fillRect(x + 18, y + 25, 7, 4, spec.palette[5]);
      } else {
        atlas.fillRect(x + 8, y + 6, 5, 6, spec.palette[6]);
        atlas.fillRect(x + 19, y + 6, 5, 6, spec.palette[6]);
        atlas.fillRect(x + 7, y + 11 + frame, 18, 13, spec.palette[2]);
        atlas.fillRect(x + 11, y + 24, 5, 5, spec.palette[1]);
        atlas.fillRect(x + 20, y + 21, 7, 3, spec.palette[3]);
      }
      const signal = clipIndex === 7 ? spec.palette[3] : spec.palette[4];
      atlas.fillRect(x + 3 + frame * 24, y + 3, 4, 4, signal);
    }
  });
  for (let index = 0; index < 8; index += 1) {
    const x = (index % 8) * 32;
    const y = 64 + Math.floor(index / 8) * 32;
    atlas.fillRect(x + 2, y + 3, 28, 24, spec.palette[(index + 1) % 6]);
    atlas.fillRect(x + 6, y + 7, 20, 4, spec.palette[7]);
  }
  return atlas;
}

function makeStudioAnchors() {
  const agentStations = Array.from({ length: 40 }, (_, index) => ({
    id: `agent-${index + 1}`,
    x: 112 + (index % 8) * 82,
    y: 190 + Math.floor(index / 8) * 88,
  }));
  const squadBoards = Array.from({ length: 12 }, (_, index) => ({
    id: `squad-${index + 1}`,
    x: 120 + (index % 6) * 168,
    y: index < 6 ? 92 : 674,
  }));
  const activeIssues = Array.from({ length: 48 }, (_, index) => ({
    id: `issue-${index + 1}`,
    x: 830 + (index % 8) * 48,
    y: 176 + Math.floor(index / 8) * 72,
  }));
  return {
    agentStations,
    squadBoards,
    activeIssues,
    dispatch: [{ id: "dispatch", x: 720, y: 392 }],
    overflow: [
      { id: "overflow-agents", x: 704, y: 670 },
      { id: "overflow-squads", x: 760, y: 670 },
      { id: "overflow-issues", x: 816, y: 670 },
    ],
    camera: [{ id: "camera", x: 640, y: 384 }],
  };
}

function makeExpeditionAnchors() {
  const agentStations = Array.from({ length: 40 }, (_, index) => {
    const row = Math.floor(index / 10);
    const column = index % 10;
    return {
      id: `agent-${index + 1}`,
      x: 182 + column * 100 + (row % 2) * 48,
      y: 242 + row * 126 + (column % 3) * 8,
    };
  });
  const squadBoards = Array.from({ length: 12 }, (_, index) => ({
    id: `squad-${index + 1}`,
    x: 112 + (index % 4) * 384,
    y: 94 + Math.floor(index / 4) * 338,
  }));
  const activeIssues = Array.from({ length: 48 }, (_, index) => {
    const band = Math.floor(index / 12);
    return {
      id: `issue-${index + 1}`,
      x: 146 + (index % 12) * 102 + (band % 2) * 32,
      y: 182 + band * 174,
    };
  });
  return {
    agentStations,
    squadBoards,
    activeIssues,
    dispatch: [{ id: "dispatch", x: 704, y: 424 }],
    overflow: [
      { id: "overflow-agents", x: 640, y: 760 },
      { id: "overflow-squads", x: 704, y: 760 },
      { id: "overflow-issues", x: 768, y: 760 },
    ],
    camera: [{ id: "camera", x: 704, y: 416 }],
  };
}

function makeFrames(world) {
  const frames = {};
  clipIds.forEach((clip, clipIndex) => {
    for (let frame = 0; frame < 2; frame += 1) {
      const index = clipIndex * 2 + frame;
      frames[`${world}-${clip}-${frame}`] = {
        x: (index % 8) * 32,
        y: Math.floor(index / 8) * 32,
        width: 32,
        height: 32,
      };
    }
  });
  return frames;
}

function makeClips(world) {
  return Object.fromEntries(
    clipIds.map((clip, index) => [
      clip,
      {
        frames: [`${world}-${clip}-0`, `${world}-${clip}-1`],
        fps: 8 + (index % 5) * 2,
        loop: !["completion", "failure"].includes(clip),
      },
    ]),
  );
}

function makeMap(world, spec) {
  const pixelWidth = spec.map.width * spec.map.tileSize;
  const pixelHeight = spec.map.height * spec.map.tileSize;
  const collisionObjects =
    world === "studio"
      ? [
          { id: 3, x: 0, y: 0, width: pixelWidth, height: 48 },
          { id: 4, x: 0, y: pixelHeight - 48, width: pixelWidth, height: 48 },
        ]
      : [
          { id: 3, x: 0, y: 0, width: 64, height: pixelHeight },
          { id: 4, x: pixelWidth - 64, y: 0, width: 64, height: pixelHeight },
          { id: 5, x: 608, y: 336, width: 192, height: 144 },
        ];
  return {
    compressionlevel: -1,
    height: spec.map.height,
    infinite: false,
    layers: [
      {
        id: 1,
        name: "ground",
        type: "objectgroup",
        visible: true,
        objects: [{ id: 1, x: 0, y: 0, width: pixelWidth, height: pixelHeight }],
      },
      {
        id: 2,
        name: "walk",
        type: "objectgroup",
        visible: true,
        objects: [
          {
            id: 2,
            x: 64,
            y: 64,
            width: pixelWidth - 128,
            height: pixelHeight - 128,
          },
        ],
      },
      {
        id: 3,
        name: "collision",
        type: "objectgroup",
        visible: false,
        objects: collisionObjects,
      },
    ],
    nextlayerid: 4,
    nextobjectid: 10,
    orientation: "orthogonal",
    renderorder: "right-down",
    tiledversion: "1.11.2",
    tileheight: spec.map.tileSize,
    tilesets: [],
    tilewidth: spec.map.tileSize,
    type: "map",
    version: "1.10",
    width: spec.map.width,
  };
}

function makeManifest(world, spec) {
  const anchors = world === "studio" ? makeStudioAnchors() : makeExpeditionAnchors();
  return {
    id: world,
    contractVersion: 1,
    layoutVersion: spec.layoutVersion,
    map: {
      asset: "./assets/map.json",
      ...spec.map,
      layers: [
        { name: "ground", type: "objectgroup" },
        { name: "walk", type: "objectgroup" },
        { name: "collision", type: "objectgroup" },
      ],
    },
    assets: {
      atlas: "./assets/atlas.png",
      poster: "./assets/poster.png",
      atlasSize: { width: 256, height: 128 },
      posterSize: { width: 768, height: 432 },
      frames: makeFrames(world),
    },
    anchors,
    clips: makeClips(world),
    palette: spec.palette,
    lighting: spec.lighting,
    visuals: spec.visuals,
    hitRegions: [
      { role: "agent", polygon: [-18, -24, 18, -24, 22, 18, -22, 18] },
      { role: "squad", polygon: [-28, -20, 28, -20, 28, 20, -28, 20] },
      { role: "issue", polygon: [-14, -14, 14, -14, 14, 14, -14, 14] },
    ],
    provenance: "../PROVENANCE.json",
  };
}

function createOutputs() {
  const outputs = new Map();
  for (const [world, spec] of Object.entries(worldSpecs)) {
    const directory = join(worldsRoot, world);
    const map = Buffer.from(json(makeMap(world, spec)));
    const atlasCanvas = drawAtlas(world);
    const posterCanvas =
      world === "studio" ? drawStudioPoster() : drawExpeditionPoster();
    outputs.set(join(directory, "manifest.json"), Buffer.from(json(makeManifest(world, spec))));
    outputs.set(join(directory, "assets/map.json"), map);
    outputs.set(
      join(directory, "assets/atlas.png"),
      encodePng(atlasCanvas.width, atlasCanvas.height, atlasCanvas.pixels),
    );
    outputs.set(
      join(directory, "assets/poster.png"),
      encodePng(posterCanvas.width, posterCanvas.height, posterCanvas.pixels),
    );
  }

  const files = [];
  for (const [path, content] of outputs) {
    if (!path.endsWith("manifest.json")) {
      const world = relative(worldsRoot, path).split("/")[0];
      files.push({
        path: relative(worldsRoot, path).replaceAll("\\", "/"),
        world,
        author: "Multica design engineering",
        source: "Deterministic code-native generator; no external image input",
        creationDate: "2026-08-29",
        generator: "scripts/generate-office-assets.mjs",
        promptArtBrief: worldSpecs[world].artBrief,
        creationMethod:
          "Programmatic RGBA pixel composition and deterministic PNG encoding",
        ownership: "Original work owned by Multica",
        license: "LicenseRef-Multica-Proprietary",
        modificationNote: "Generated from checked-in numeric geometry and palette constants",
        attributionRequired: false,
        sha256: sha256(content),
      });
    }
  }
  files.sort((left, right) => left.path.localeCompare(right.path));
  outputs.set(
    join(worldsRoot, "PROVENANCE.json"),
    Buffer.from(json({ schemaVersion: 1, files })),
  );
  outputs.set(
    join(worldsRoot, "ATTRIBUTION.md"),
    Buffer.from(
      "# Office World Asset Attribution\n\n" +
        "Studio and Expedition assets are original deterministic works created for Multica by `scripts/generate-office-assets.mjs`. They use no copied third-party code, maps, characters, silhouettes, audio, or image assets.\n\n" +
        "No third-party attribution is required. Human design and IP review remains required before Expedition ships.\n",
    ),
  );
  return outputs;
}

const outputs = createOutputs();
const stale = [];
for (const [path, expected] of outputs) {
  if (checkOnly) {
    if (!existsSync(path) || !readFileSync(path).equals(expected)) {
      stale.push(relative(repoRoot, path));
    }
  } else {
    mkdirSync(dirname(path), { recursive: true });
    writeFileSync(path, expected);
  }
}

if (stale.length > 0) {
  process.stderr.write(`Office assets are stale:\n${stale.join("\n")}\n`);
  process.exitCode = 1;
} else {
  process.stdout.write(
    checkOnly
      ? `Office assets are reproducible (${outputs.size} files).\n`
      : `Generated ${outputs.size} Office world files.\n`,
  );
}
