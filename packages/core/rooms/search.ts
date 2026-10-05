import type { Room } from "./types";

export function filterRooms(
  rooms: readonly Room[],
  query: string,
): readonly Room[] {
  const terms = normalizeRoomSearch(query).split(/\s+/).filter(Boolean);
  if (terms.length === 0) return rooms;

  return rooms.filter((room) => {
    const haystack = normalizeRoomSearch([
      room.title,
      room.objective,
      room.instructions,
      room.memory.summary,
      ...room.memory.facts,
      ...room.memory.decisions,
      ...room.memory.open_questions,
    ].join("\n"));
    return terms.every((term) => haystack.includes(term));
  });
}

function normalizeRoomSearch(value: string): string {
  return value.normalize("NFKC").toLocaleLowerCase().trim();
}
