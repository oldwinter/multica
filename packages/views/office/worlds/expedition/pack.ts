import manifest from "./manifest.json";
import { parseOfficeWorldPack } from "../pack-parser";

export function loadExpeditionPack() {
  return parseOfficeWorldPack(manifest, {
    atlas: new URL("./assets/atlas.png", import.meta.url).href,
    map: new URL("./assets/map.json", import.meta.url).href,
    poster: new URL("./assets/poster.png", import.meta.url).href,
    provenance: new URL("../PROVENANCE.json", import.meta.url).href,
  });
}
