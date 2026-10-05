import { matchLocale, type SupportedLocale } from "@multica/core/i18n";

/** Convert an i18next BCP-47 language tag into a locale the settings picker owns. */
export function resolveSettingsLocale(language: string): SupportedLocale {
  return matchLocale([language]);
}
