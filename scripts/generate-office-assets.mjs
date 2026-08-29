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
    layoutVersion: 2,
    map: { width: 72, height: 60, tileSize: 16 },
    palette: [
      "#22282d",
      "#d8d3c6",
      "#4f705b",
      "#e66f61",
      "#e7b553",
      "#8799a2",
      "#f2eee4",
      "#11161a",
      "#3c6585",
      "#b9b4aa",
      "#789174",
      "#b8cbd0",
      "#343a3f",
      "#ebe7dc",
    ],
    lighting: {
      light: { ambient: "#f4f1e8", overlayAlpha: 0.04 },
      dark: { ambient: "#111519", overlayAlpha: 0.28 },
    },
    visuals: {
      actorSilhouette: "compact-human-operator",
      stationStyle: "cantilever-desk-pod",
      props: [
        "dispatch-console",
        "collaboration-table",
        "project-wall",
        "planter",
        "server-rack",
        "storage-bank",
        "glass-rail",
      ],
      backdropColor: 9,
      decor: makeStudioDecor(),
    },
    artBrief:
      "Original top-down modern workplace with concrete bays, graphite desks, green planters, coral signals, amber task packets, and cool-metal service rails.",
  },
  expedition: {
    layoutVersion: 4,
    map: { width: 78, height: 64, tileSize: 16 },
    palette: [
      "#1c2922",
      "#516b48",
      "#315c87",
      "#eb7768",
      "#8ea392",
      "#343a3e",
      "#d8c98f",
      "#0d1518",
      "#47788d",
      "#737873",
      "#e4b95f",
      "#efe2ae",
      "#24415f",
      "#6d875b",
    ],
    lighting: {
      light: { ambient: "#d7c887", overlayAlpha: 0.06 },
      dark: { ambient: "#0c1318", overlayAlpha: 0.34 },
    },
    visuals: {
      actorSilhouette: "faceted-field-companion",
      stationStyle: "radial-stone-workbench",
      props: [
        "route-beacon",
        "canvas-shelter",
        "stone-workbench",
        "field-map",
        "supply-crate",
        "survey-standard",
        "water-channel",
      ],
      backdropColor: 0,
      decor: makeExpeditionDecor(),
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
    hex.length === 8 ? Number.parseInt(hex.slice(6, 8), 16) : 255,
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

function rect(x, y, width, height, color) {
  return { kind: "rect", x, y, width, height, color };
}

function circle(x, y, radius, color) {
  return { kind: "circle", x, y, radius, color };
}

function polygon(points, color) {
  return { kind: "polygon", points, color };
}

function line(points, width, color) {
  return { kind: "line", points, width, color };
}

function studioAgentPositions() {
  const positions = [];
  for (const [originX, originY] of [
    [140, 190],
    [570, 190],
    [140, 560],
    [570, 650],
  ]) {
    for (let row = 0; row < 3; row += 1) {
      for (let column = 0; column < 3; column += 1) {
        positions.push({ x: originX + column * 110, y: originY + row * 95 });
      }
    }
  }
  positions.push(
    { x: 950, y: 440 },
    { x: 1040, y: 440 },
    { x: 950, y: 610 },
    { x: 1040, y: 610 },
  );
  return positions;
}

function expeditionAgentPositions() {
  const offsets = [
    [-78, -34],
    [-18, -72],
    [48, -54],
    [88, 2],
    [52, 62],
    [-12, 76],
    [-74, 52],
    [2, 4],
  ];
  const positions = [];
  for (const [centerX, centerY] of [
    [250, 260],
    [610, 310],
    [980, 270],
    [360, 720],
    [880, 760],
  ]) {
    for (const [offsetX, offsetY] of offsets) {
      positions.push({ x: centerX + offsetX, y: centerY + offsetY });
    }
  }
  return positions;
}

function addStudioDesk(decor, x, y, index) {
  const surface = index % 3 === 0 ? 11 : 13;
  decor.push(
    rect(x - 45, y - 45, 90, 60, surface),
    rect(x - 38, y - 32, 76, 16, 0),
    rect(x - 30, y - 28, 32, 6, 5),
    rect(x + 9, y - 28, 21, 6, index % 4 === 0 ? 8 : 2),
    rect(x - 5, y - 10, 10, 10, 9),
  );
}

function addPlanter(decor, x, y, scale = 1) {
  decor.push(
    rect(x - 13 * scale, y + 4 * scale, 26 * scale, 18 * scale, 0),
    rect(x - 9 * scale, y + 7 * scale, 18 * scale, 8 * scale, 9),
    circle(x - 7 * scale, y, 10 * scale, 2),
    circle(x + 5 * scale, y - 5 * scale, 12 * scale, 10),
    circle(x + 10 * scale, y + 3 * scale, 8 * scale, 2),
  );
}

function makeStudioDecor() {
  const decor = [
    rect(0, 0, 1152, 960, 1),
    rect(0, 0, 1152, 54, 0),
    rect(30, 18, 1092, 8, 5),
    rect(52, 30, 120, 10, 8),
    rect(184, 30, 42, 10, 3),
    rect(238, 30, 190, 10, 12),
    rect(440, 30, 42, 10, 4),
    rect(494, 30, 260, 10, 8),
    rect(766, 30, 58, 10, 3),
    rect(836, 30, 286, 10, 12),
    rect(42, 70, 28, 828, 9),
    rect(438, 86, 92, 792, 13),
    rect(72, 438, 818, 82, 13),
    rect(84, 118, 354, 304, 9),
    rect(542, 118, 334, 304, 9),
    rect(84, 530, 354, 318, 9),
    rect(542, 620, 334, 242, 9),
    rect(86, 124, 350, 6, 5),
    rect(544, 124, 330, 6, 8),
    rect(86, 536, 350, 6, 2),
    rect(544, 626, 330, 6, 4),
    rect(462, 154, 42, 232, 1),
    rect(462, 178, 42, 6, 5),
    rect(462, 250, 42, 6, 5),
    rect(462, 322, 42, 6, 5),
    rect(458, 444, 400, 156, 9),
    rect(476, 462, 364, 120, 13),
    rect(526, 488, 264, 68, 0),
    rect(542, 500, 232, 44, 8),
    rect(558, 512, 62, 8, 11),
    rect(632, 512, 38, 8, 4),
    rect(682, 512, 76, 8, 3),
    rect(895, 88, 227, 238, 12),
    rect(911, 106, 195, 202, 0),
    rect(926, 122, 75, 72, 8),
    rect(1011, 122, 79, 72, 11),
    rect(926, 204, 164, 88, 6),
    rect(938, 218, 46, 8, 3),
    rect(994, 218, 82, 8, 4),
    rect(938, 238, 110, 8, 5),
    rect(938, 258, 62, 8, 2),
    rect(890, 348, 232, 466, 7),
    rect(910, 366, 192, 430, 12),
    rect(924, 382, 74, 126, 0),
    rect(1008, 382, 78, 126, 0),
    rect(924, 526, 74, 126, 0),
    rect(1008, 526, 78, 126, 0),
    rect(932, 394, 58, 10, 8),
    rect(1018, 394, 60, 10, 8),
    rect(932, 538, 58, 10, 2),
    rect(1018, 538, 60, 10, 3),
    rect(924, 680, 162, 96, 0),
    rect(938, 696, 134, 12, 5),
    rect(938, 720, 48, 40, 8),
    rect(998, 720, 74, 40, 4),
    rect(70, 786, 248, 112, 0),
    rect(84, 800, 220, 84, 12),
    rect(102, 816, 122, 46, 8),
    rect(236, 816, 50, 46, 5),
    rect(112, 830, 34, 8, 4),
    rect(154, 830, 54, 8, 3),
    rect(334, 870, 532, 28, 12),
    rect(350, 880, 92, 10, 5),
    rect(454, 880, 92, 10, 2),
    rect(558, 880, 92, 10, 8),
    rect(662, 880, 92, 10, 4),
    rect(766, 880, 84, 10, 3),
  ];

  studioAgentPositions().forEach((position, index) =>
    addStudioDesk(decor, position.x, position.y, index),
  );
  for (const [x, y, scale] of [
    [94, 100, 1],
    [410, 405, 1],
    [548, 430, 1],
    [846, 404, 1],
    [94, 516, 1],
    [416, 826, 1],
    [850, 844, 1],
    [1090, 840, 1],
  ]) {
    addPlanter(decor, x, y, scale);
  }
  for (let index = 0; index < 8; index += 1) {
    decor.push(
      rect(80 + index * 96, 454, 50, 6, index % 2 === 0 ? 5 : 9),
      rect(80 + index * 96, 498, 50, 6, index % 3 === 0 ? 2 : 9),
    );
  }
  return decor;
}

function addFieldWorkbench(decor, x, y, index) {
  const stone = index % 2 === 0 ? 9 : 5;
  decor.push(
    polygon(
      [x - 42, y - 28, x - 12, y - 42, x + 35, y - 30, x + 46, y + 2, x + 18, y + 22, x - 34, y + 17],
      stone,
    ),
    rect(x - 31, y - 24, 62, 12, 6),
    rect(x - 19, y - 20, 38, 5, index % 3 === 0 ? 2 : 4),
    rect(x - 7, y - 8, 14, 10, 0),
  );
}

function addBeacon(decor, x, y, height = 54) {
  decor.push(
    rect(x - 3, y - height, 6, height, 6),
    polygon([x, y - height - 18, x + 16, y - height, x, y - height + 10, x - 16, y - height], 3),
    rect(x - 8, y - 10, 16, 10, 5),
    rect(x - 2, y - height - 8, 4, 8, 10),
  );
}

function makeExpeditionDecor() {
  const decor = [
    rect(0, 0, 1248, 1024, 0),
    polygon([0, 744, 176, 696, 332, 760, 430, 884, 374, 1024, 0, 1024], 8),
    polygon([0, 802, 150, 758, 286, 806, 368, 912, 334, 1024, 0, 1024], 12),
    line([8, 778, 168, 732, 318, 790, 394, 886], 8, 11),
    line([250, 260, 610, 310, 980, 270, 880, 760, 360, 720, 250, 260], 34, 9),
    line([250, 260, 610, 310, 980, 270, 880, 760, 360, 720, 250, 260], 18, 6),
    polygon([96, 134, 222, 104, 342, 142, 392, 244, 342, 354, 198, 386, 94, 314, 58, 220], 5),
    polygon([450, 156, 610, 112, 752, 174, 804, 310, 746, 424, 588, 458, 454, 384, 410, 260], 5),
    polygon([842, 112, 1020, 96, 1152, 174, 1178, 306, 1104, 390, 930, 414, 824, 330, 798, 218], 5),
    polygon([168, 578, 348, 526, 498, 600, 530, 756, 438, 858, 248, 872, 128, 754], 5),
    polygon([700, 580, 900, 520, 1088, 598, 1142, 768, 1044, 894, 820, 908, 678, 790], 5),
    polygon([130, 122, 246, 54, 356, 132, 318, 214, 166, 210], 2),
    polygon([146, 128, 246, 72, 246, 198, 172, 198], 12),
    line([246, 66, 246, 210], 8, 6),
    polygon([882, 430, 1010, 356, 1136, 444, 1082, 526, 920, 522], 2),
    polygon([900, 438, 1010, 376, 1010, 512, 930, 510], 12),
    line([1010, 370, 1010, 532], 8, 6),
    polygon([520, 486, 706, 468, 780, 566, 700, 650, 518, 630, 466, 548], 9),
    polygon([542, 504, 688, 496, 742, 558, 678, 620, 536, 604, 498, 546], 6),
    rect(558, 522, 166, 62, 12),
    rect(576, 536, 130, 34, 2),
    line([590, 544, 620, 558, 650, 548, 684, 562], 5, 3),
    rect(62, 448, 86, 62, 6),
    rect(70, 456, 32, 22, 2),
    rect(110, 456, 30, 22, 3),
    rect(82, 486, 58, 16, 10),
    rect(1090, 470, 92, 74, 6),
    rect(1098, 478, 36, 28, 2),
    rect(1142, 478, 32, 28, 3),
    rect(1098, 514, 76, 22, 10),
    rect(524, 864, 86, 62, 6),
    rect(532, 872, 32, 22, 1),
    rect(572, 872, 30, 22, 2),
    rect(544, 902, 58, 16, 10),
    line([76, 420, 188, 362, 302, 410], 8, 3),
    line([1024, 570, 1112, 620, 1182, 706], 8, 3),
    line([438, 880, 474, 936, 532, 968], 8, 3),
  ];

  expeditionAgentPositions().forEach((position, index) =>
    addFieldWorkbench(decor, position.x, position.y, index),
  );
  for (const [x, y, height] of [
    [66, 186, 62],
    [446, 92, 52],
    [780, 118, 66],
    [1190, 224, 58],
    [1160, 838, 72],
    [742, 952, 56],
    [188, 934, 64],
    [84, 628, 52],
  ]) {
    addBeacon(decor, x, y, height);
  }
  for (const [x, y, color] of [
    [430, 184, 13],
    [790, 416, 1],
    [1138, 360, 13],
    [574, 744, 1],
    [1082, 930, 13],
    [222, 512, 1],
  ]) {
    decor.push(
      polygon([x - 22, y + 8, x - 12, y - 16, x + 7, y - 23, x + 24, y - 4, x + 16, y + 18, x - 8, y + 24], color),
      polygon([x - 8, y + 4, x, y - 10, x + 12, y + 6, x + 4, y + 16], 4),
    );
  }
  for (let index = 0; index < 10; index += 1) {
    const x = 96 + index * 108;
    const y = 554 + (index % 3) * 16;
    decor.push(
      polygon([x - 7, y, x, y - 10, x + 7, y, x, y + 10], 3),
      rect(x - 2, y + 10, 4, 18, 6),
    );
  }
  return decor;
}

function drawCircle(canvas, centerX, centerY, radius, color) {
  const radiusSquared = radius * radius;
  for (let y = Math.floor(centerY - radius); y <= Math.ceil(centerY + radius); y += 1) {
    for (let x = Math.floor(centerX - radius); x <= Math.ceil(centerX + radius); x += 1) {
      const dx = x + 0.5 - centerX;
      const dy = y + 0.5 - centerY;
      if (dx * dx + dy * dy <= radiusSquared) canvas.fillRect(x, y, 1, 1, color);
    }
  }
}

function drawPolygon(canvas, points, color) {
  const ys = points.filter((_, index) => index % 2 === 1);
  const top = Math.floor(Math.min(...ys));
  const bottom = Math.ceil(Math.max(...ys));
  for (let y = top; y < bottom; y += 1) {
    const scanY = y + 0.5;
    const intersections = [];
    for (let index = 0; index < points.length; index += 2) {
      const next = (index + 2) % points.length;
      const x1 = points[index];
      const y1 = points[index + 1];
      const x2 = points[next];
      const y2 = points[next + 1];
      if ((y1 > scanY) === (y2 > scanY)) continue;
      intersections.push(x1 + ((scanY - y1) * (x2 - x1)) / (y2 - y1));
    }
    intersections.sort((left, right) => left - right);
    for (let index = 0; index + 1 < intersections.length; index += 2) {
      const left = Math.floor(intersections[index]);
      const right = Math.ceil(intersections[index + 1]);
      canvas.fillRect(left, y, Math.max(1, right - left), 1, color);
    }
  }
}

function drawLine(canvas, points, width, color) {
  const radius = Math.max(0, Math.floor(width / 2));
  for (let index = 0; index + 3 < points.length; index += 2) {
    const x1 = points[index];
    const y1 = points[index + 1];
    const x2 = points[index + 2];
    const y2 = points[index + 3];
    const steps = Math.max(1, Math.ceil(Math.max(Math.abs(x2 - x1), Math.abs(y2 - y1))));
    for (let step = 0; step <= steps; step += 1) {
      const progress = step / steps;
      const x = Math.round(x1 + (x2 - x1) * progress);
      const y = Math.round(y1 + (y2 - y1) * progress);
      canvas.fillRect(x - radius, y - radius, radius * 2 + 1, radius * 2 + 1, color);
    }
  }
}

function drawDecor(canvas, decor, palette) {
  for (const element of decor) {
    const color = palette[element.color];
    if (element.kind === "rect") {
      canvas.fillRect(element.x, element.y, element.width, element.height, color);
    } else if (element.kind === "circle") {
      drawCircle(canvas, element.x, element.y, element.radius, color);
    } else if (element.kind === "polygon") {
      drawPolygon(canvas, element.points, color);
    } else {
      drawLine(canvas, element.points, element.width, color);
    }
  }
}

function drawStudioPreviewActor(canvas, x, y, variant, palette) {
  const body = [8, 2, 5, 3][variant % 4];
  canvas.fillRect(x - 11, y - 28, 22, 19, palette[body]);
  canvas.fillRect(x - 8, y - 40, 16, 13, palette[6]);
  canvas.fillRect(x - 9 + (variant % 2) * 3, y - 43, 12, 5, palette[0]);
  canvas.fillRect(x - 9, y - 9, 7, 8, palette[0]);
  canvas.fillRect(x + 2, y - 9, 7, 8, palette[0]);
  canvas.fillRect(x - 2, y - 23, 4, 10, palette[4]);
}

function drawExpeditionPreviewActor(canvas, x, y, variant, palette) {
  const body = [2, 1, 4, 6][variant % 4];
  const accent = [3, 10, 8, 11][variant % 4];
  drawPolygon(canvas, [x - 16, y - 23, x, y - 34, x + 18, y - 19, x + 12, y, x - 12, y], palette[body]);
  drawPolygon(canvas, [x - 13, y - 31, x - 8, y - 45, x - 2, y - 32], palette[accent]);
  drawPolygon(canvas, [x + 4, y - 33, x + 12, y - 46, x + 15, y - 27], palette[accent]);
  drawPolygon(canvas, [x + 14, y - 15, x + 27, y - 9, x + 18, y - 3], palette[accent]);
  canvas.fillRect(x - 6, y - 23, 4, 4, palette[7]);
  canvas.fillRect(x + 7, y - 23, 4, 4, palette[7]);
}

function blitFitted(source, target, padding) {
  const scale = Math.min(
    (target.width - padding * 2) / source.width,
    (target.height - padding * 2) / source.height,
  );
  const width = Math.max(1, Math.floor(source.width * scale));
  const height = Math.max(1, Math.floor(source.height * scale));
  const offsetX = Math.floor((target.width - width) / 2);
  const offsetY = Math.floor((target.height - height) / 2);
  for (let y = 0; y < height; y += 1) {
    for (let x = 0; x < width; x += 1) {
      const sourceX = Math.min(source.width - 1, Math.floor(x / scale));
      const sourceY = Math.min(source.height - 1, Math.floor(y / scale));
      const offset = (sourceY * source.width + sourceX) * 4;
      const color = `#${[0, 1, 2]
        .map((channel) => source.pixels[offset + channel].toString(16).padStart(2, "0"))
        .join("")}`;
      target.fillRect(offsetX + x, offsetY + y, 1, 1, color);
    }
  }
}

function drawPoster(world) {
  const spec = worldSpecs[world];
  const width = spec.map.width * spec.map.tileSize;
  const height = spec.map.height * spec.map.tileSize;
  const scene = createCanvas(width, height, spec.palette[spec.visuals.backdropColor]);
  drawDecor(scene, spec.visuals.decor, spec.palette);
  const positions = world === "studio" ? studioAgentPositions() : expeditionAgentPositions();
  positions.filter((_, index) => index % 3 === 0).forEach((position, index) => {
    if (world === "studio") {
      drawStudioPreviewActor(scene, position.x, position.y, index % 4, spec.palette);
    } else {
      drawExpeditionPreviewActor(scene, position.x, position.y, index % 4, spec.palette);
    }
  });
  const poster = createCanvas(192, 108, spec.palette[spec.visuals.backdropColor]);
  blitFitted(scene, poster, 4);
  return scaleCanvas(poster, 4);
}

function drawStudioAtlasFrame(canvas, x, y, variant, clipIndex, frame, palette) {
  const bob = clipIndex === 3 ? frame * 2 : clipIndex === 0 ? frame : 0;
  const bodyIndex = clipIndex === 5 ? 5 : [8, 2, 5, 3][variant];
  const accentIndex = [4, 3, 11, 10][variant];
  canvas.fillRect(x + 10, y + 43, 28, 3, palette[0]);
  canvas.fillRect(x + 14, y + 21 + bob, 20, 17, palette[bodyIndex]);
  canvas.fillRect(x + 18, y + 9 + bob, 13, 13, palette[6]);
  if (variant === 0) {
    canvas.fillRect(x + 17, y + 7 + bob, 15, 5, palette[0]);
  } else if (variant === 1) {
    canvas.fillRect(x + 16, y + 8 + bob, 17, 4, palette[0]);
    canvas.fillRect(x + 16, y + 12 + bob, 4, 7, palette[0]);
  } else if (variant === 2) {
    canvas.fillRect(x + 17, y + 6 + bob, 13, 6, palette[0]);
    canvas.fillRect(x + 28, y + 10 + bob, 6, 3, palette[accentIndex]);
  } else {
    canvas.fillRect(x + 16, y + 8 + bob, 4, 9, palette[0]);
    canvas.fillRect(x + 20, y + 5 + bob, 5, 7, palette[0]);
    canvas.fillRect(x + 25, y + 7 + bob, 7, 5, palette[0]);
  }
  canvas.fillRect(x + 20, y + 15 + bob, 3, 3, palette[7]);
  canvas.fillRect(x + 27, y + 15 + bob, 3, 3, palette[7]);
  canvas.fillRect(x + 20, y + 25 + bob, 10, 4, palette[accentIndex]);
  if (clipIndex === 2) {
    canvas.fillRect(x + 8, y + 25 + bob + frame * 2, 8, 5, palette[bodyIndex]);
    canvas.fillRect(x + 33, y + 25 + bob + (1 - frame) * 2, 8, 5, palette[bodyIndex]);
    canvas.fillRect(x + 20, y + 34, 10, 5, palette[4]);
  } else {
    canvas.fillRect(x + 9, y + 23 + bob, 6, 12, palette[bodyIndex]);
    canvas.fillRect(x + 34, y + 23 + bob, 6, 12, palette[bodyIndex]);
  }
  const stride = clipIndex === 3 ? frame * 3 : 0;
  canvas.fillRect(x + 15 - stride, y + 37, 8, 7, palette[0]);
  canvas.fillRect(x + 27 + stride, y + 37, 8, 7, palette[0]);
  if (clipIndex === 4) {
    canvas.fillRect(x + 8 + frame * 27, y + 8, 5, 5, palette[4]);
    canvas.fillRect(x + 36 - frame * 27, y + 15, 4, 4, palette[3]);
  } else if (clipIndex === 6) {
    canvas.fillRect(x + 36, y + 8 + frame * 4, 7, 7, palette[2]);
  } else if (clipIndex === 7) {
    canvas.fillRect(x + 37, y + 8 + frame * 4, 6, 9, palette[3]);
  }
}

function drawExpeditionAtlasFrame(canvas, x, y, variant, clipIndex, frame, palette) {
  const bob = clipIndex === 3 ? frame * 2 : clipIndex === 0 ? frame : 0;
  const bodyIndex = clipIndex === 5 ? 9 : [2, 1, 4, 6][variant];
  const accentIndex = [3, 10, 8, 11][variant];
  canvas.fillRect(x + 9, y + 43, 30, 3, palette[5]);
  if (variant === 0) {
    drawPolygon(canvas, [x + 11, y + 28, x + 3, y + 18, x + 8, y + 34], palette[accentIndex]);
  } else if (variant === 1) {
    drawPolygon(canvas, [x + 10, y + 31, x + 1, y + 29, x + 7, y + 40, x + 15, y + 36], palette[accentIndex]);
  } else if (variant === 2) {
    drawPolygon(canvas, [x + 11, y + 28, x + 2, y + 20, x + 4, y + 37, x + 15, y + 36], palette[accentIndex]);
  } else {
    drawPolygon(canvas, [x + 12, y + 31, x + 2, y + 23, x + 3, y + 33, x + 8, y + 41], palette[accentIndex]);
  }
  drawPolygon(
    canvas,
    [x + 10, y + 30 + bob, x + 16, y + 17 + bob, x + 32, y + 16 + bob, x + 40, y + 29 + bob, x + 34, y + 40, x + 16, y + 40],
    palette[bodyIndex],
  );
  const leftEarTop = variant % 2 === 0 ? 4 : 8;
  const rightEarTop = variant < 2 ? 5 : 10;
  drawPolygon(canvas, [x + 14, y + 20 + bob, x + 14, y + leftEarTop + bob, x + 22, y + 17 + bob], palette[accentIndex]);
  drawPolygon(canvas, [x + 29, y + 17 + bob, x + 36, y + rightEarTop + bob, x + 36, y + 23 + bob], palette[accentIndex]);
  drawPolygon(canvas, [x + 17, y + 18 + bob, x + 25, y + 13 + bob, x + 34, y + 18 + bob, x + 31, y + 30 + bob, x + 19, y + 30 + bob], palette[bodyIndex]);
  canvas.fillRect(x + 19, y + 20 + bob, 4, 4, palette[7]);
  canvas.fillRect(x + 29, y + 20 + bob, 4, 4, palette[7]);
  canvas.fillRect(x + 24, y + 27 + bob, 5, 3, palette[accentIndex]);
  const stride = clipIndex === 3 ? frame * 3 : 0;
  canvas.fillRect(x + 15 - stride, y + 39, 8, 5, palette[5]);
  canvas.fillRect(x + 30 + stride, y + 39, 8, 5, palette[5]);
  if (clipIndex === 2) {
    canvas.fillRect(x + 18, y + 33 + frame, 16, 6, palette[6]);
    canvas.fillRect(x + 22, y + 34 + frame, 8, 3, palette[2]);
  } else if (clipIndex === 4) {
    canvas.fillRect(x + 6 + frame * 33, y + 10, 5, 5, palette[10]);
    canvas.fillRect(x + 39 - frame * 31, y + 17, 4, 4, palette[3]);
  } else if (clipIndex === 6) {
    drawPolygon(canvas, [x + 40, y + 7 + frame * 3, x + 45, y + 12 + frame * 3, x + 40, y + 17 + frame * 3, x + 35, y + 12 + frame * 3], palette[10]);
  } else if (clipIndex === 7) {
    drawPolygon(canvas, [x + 40, y + 7 + frame * 3, x + 45, y + 15 + frame * 3, x + 36, y + 15 + frame * 3], palette[3]);
  }
}

function drawAtlas(world) {
  const spec = worldSpecs[world];
  const atlas = createCanvas(384, 384, "#00000000");
  for (let variant = 0; variant < 4; variant += 1) {
    clipIds.forEach((clip, clipIndex) => {
      for (let frame = 0; frame < 2; frame += 1) {
        const index = variant * clipIds.length * 2 + clipIndex * 2 + frame;
        const x = (index % 8) * 48;
        const y = Math.floor(index / 8) * 48;
        if (world === "studio") {
          drawStudioAtlasFrame(atlas, x, y, variant, clipIndex, frame, spec.palette);
        } else {
          drawExpeditionAtlasFrame(atlas, x, y, variant, clipIndex, frame, spec.palette);
        }
      }
    });
  }
  return atlas;
}

function makeStudioAnchors() {
  const agentStations = studioAgentPositions().map((position, index) => ({
    id: `agent-${index + 1}`,
    ...position,
  }));
  const squadBoards = [
    [112, 92],
    [292, 92],
    [472, 92],
    [652, 92],
    [832, 92],
    [1012, 92],
    [112, 904],
    [292, 904],
    [472, 904],
    [652, 904],
    [832, 904],
    [1012, 904],
  ].map(([x, y], index) => ({ id: `squad-${index + 1}`, x, y }));
  const activeIssues = Array.from({ length: 48 }, (_, index) => ({
    id: `issue-${index + 1}`,
    x: 904 + (index % 6) * 38,
    y: 142 + Math.floor(index / 6) * 82,
  }));
  return {
    agentStations,
    squadBoards,
    activeIssues,
    dispatch: [{ id: "dispatch", x: 194, y: 838 }],
    overflow: [
      { id: "overflow-agents", x: 966, y: 862 },
      { id: "overflow-squads", x: 1022, y: 862 },
      { id: "overflow-issues", x: 1078, y: 862 },
    ],
    camera: [{ id: "camera", x: 576, y: 480 }],
  };
}

function makeExpeditionAnchors() {
  const agentStations = expeditionAgentPositions().map((position, index) => ({
    id: `agent-${index + 1}`,
    ...position,
  }));
  const squadBoards = [
    [66, 118],
    [246, 70],
    [446, 52],
    [780, 66],
    [1010, 76],
    [1190, 156],
    [1180, 884],
    [1002, 948],
    [742, 966],
    [470, 950],
    [188, 944],
    [70, 686],
  ].map(([x, y], index) => ({ id: `squad-${index + 1}`, x, y }));
  const route = [
    [112, 414],
    [220, 366],
    [328, 410],
    [440, 360],
    [544, 404],
    [650, 364],
    [756, 414],
    [862, 370],
    [968, 414],
    [1074, 374],
    [1168, 426],
    [1124, 560],
    [1040, 626],
    [952, 580],
    [868, 636],
    [782, 594],
    [698, 650],
    [612, 608],
    [528, 664],
    [442, 622],
    [356, 678],
    [270, 636],
    [184, 690],
    [98, 650],
  ];
  const activeIssues = Array.from({ length: 48 }, (_, index) => {
    const [x, y] = route[index % route.length];
    const band = Math.floor(index / route.length);
    return {
      id: `issue-${index + 1}`,
      x: x + band * 12,
      y: y + band * 24,
    };
  });
  return {
    agentStations,
    squadBoards,
    activeIssues,
    dispatch: [{ id: "dispatch", x: 622, y: 552 }],
    overflow: [
      { id: "overflow-agents", x: 548, y: 954 },
      { id: "overflow-squads", x: 622, y: 954 },
      { id: "overflow-issues", x: 696, y: 954 },
    ],
    camera: [{ id: "camera", x: 624, y: 512 }],
  };
}

function makeFrames(world) {
  const frames = {};
  for (let variant = 0; variant < 4; variant += 1) {
    clipIds.forEach((clip, clipIndex) => {
      for (let frame = 0; frame < 2; frame += 1) {
        const index = variant * clipIds.length * 2 + clipIndex * 2 + frame;
        frames[`${world}-v${variant}-${clip}-${frame}`] = {
          x: (index % 8) * 48,
          y: Math.floor(index / 8) * 48,
          width: 48,
          height: 48,
        };
      }
    });
  }
  return frames;
}

function makeClips(world) {
  return Object.fromEntries(
    clipIds.map((clip, index) => [
      clip,
      {
        variants: Array.from({ length: 4 }, (_, variant) => [
          `${world}-v${variant}-${clip}-0`,
          `${world}-v${variant}-${clip}-1`,
        ]),
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
      atlasSize: { width: 384, height: 384 },
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
    const posterCanvas = drawPoster(world);
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
