import { useT } from "./use-t";

const MINUTE_MS = 60_000;
type CommonTranslator = ReturnType<typeof useT<"common">>["t"];

// Localized relative-time formatter. Returns a function so call-site usage
// stays terse: `const timeAgo = useTimeAgo(); ...timeAgo(dateStr)`.
export function useTimeAgo() {
  const { t } = useT("common");
  return (dateStr: string): string => formatTimeAgo(dateStr, t);
}

// Scheduled timestamps are normally in the future, while a delayed refresh
// can leave a stale timestamp behind. Keep both directions readable without
// changing the past-time copy used by existing callers.
export function useTimeUntil() {
  const { t } = useT("common");
  return (dateStr: string): string => {
    const diff = new Date(dateStr).getTime() - Date.now();
    if (diff <= 0) return formatTimeAgo(dateStr, t);
    if (diff < MINUTE_MS) return t(($) => $.time.soon);

    const minutes = Math.floor(diff / MINUTE_MS);
    if (minutes < 60) return t(($) => $.time.minutes_until, { count: minutes });
    const hours = Math.floor(minutes / 60);
    if (hours < 24) return t(($) => $.time.hours_until, { count: hours });
    const days = Math.floor(hours / 24);
    return t(($) => $.time.days_until, { count: days });
  };
}

function formatTimeAgo(dateStr: string, t: CommonTranslator): string {
  const diff = Date.now() - new Date(dateStr).getTime();
  const minutes = Math.floor(diff / MINUTE_MS);
  if (minutes < 1) return t(($) => $.time.just_now);
  if (minutes < 60) return t(($) => $.time.minutes_ago, { count: minutes });
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return t(($) => $.time.hours_ago, { count: hours });
  const days = Math.floor(hours / 24);
  return t(($) => $.time.days_ago, { count: days });
}
