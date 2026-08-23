import { ApiError } from "@multica/core/api";
import {
  parseWikiRevisionConflict,
  type WikiRevisionConflict,
} from "@multica/core/wiki";

export function wikiConflict(error: unknown): WikiRevisionConflict | null {
  if (
    !(error instanceof ApiError)
    || error.status !== 409
    || !error.body
    || typeof error.body !== "object"
  ) {
    return null;
  }
  return parseWikiRevisionConflict(error.body);
}
