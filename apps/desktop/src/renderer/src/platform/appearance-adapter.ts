import {
  applyCachedBrowserAppearanceBeforePaint,
  createBrowserAppearanceAdapter,
} from "@multica/views/appearance";

export const desktopAppearanceAdapter =
  createBrowserAppearanceAdapter("desktop");

export { applyCachedBrowserAppearanceBeforePaint };
