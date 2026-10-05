import { cn } from "@multica/ui/lib/utils";

const VISIBLE_ID_PREFIX_LENGTH = 9;

export function OfficeCompactIdentifier({
  id,
  className,
}: {
  readonly id: string;
  readonly className?: string;
}) {
  const visibleId =
    id.length > VISIBLE_ID_PREFIX_LENGTH
      ? `${id.slice(0, VISIBLE_ID_PREFIX_LENGTH)}...`
      : id;
  return (
    <span
      aria-label={id}
      title={id}
      className={cn(
        "inline-block max-w-24 truncate align-bottom font-mono",
        className,
      )}
    >
      {visibleId}
    </span>
  );
}
