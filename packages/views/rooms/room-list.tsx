"use client";

import { useMemo, useState } from "react";
import { MessageSquareText, Plus, Search } from "lucide-react";
import { filterRooms, rankRoomsForValueReview, type Room } from "@multica/core/rooms";
import { Button } from "@multica/ui/components/ui/button";
import { Input } from "@multica/ui/components/ui/input";
import { Skeleton } from "@multica/ui/components/ui/skeleton";
import { cn } from "@multica/ui/lib/utils";
import { useT, useTimeAgo } from "../i18n";
import { roomStatusDotClass } from "./room-display";

interface RoomListProps {
  readonly rooms: readonly Room[];
  readonly selectedId: string;
  readonly loading: boolean;
  readonly showValueReview: boolean;
  readonly onSelect: (roomId: string) => void;
  readonly onCreate: () => void;
}

export function RoomList({
  rooms,
  selectedId,
  loading,
  showValueReview,
  onSelect,
  onCreate,
}: RoomListProps) {
  const { t } = useT("rooms");
  const timeAgo = useTimeAgo();
  const [query, setQuery] = useState("");
  const filteredRooms = useMemo(() => filterRooms(rooms, query), [rooms, query]);
  const valueReviewRooms = useMemo(
    () => (showValueReview ? rankRoomsForValueReview(rooms, Date.now(), 30, 3) : []),
    [rooms, showValueReview],
  );

  return (
    <aside
      data-testid="room-list"
      className="flex max-h-[30dvh] min-h-0 flex-col border-b border-surface-border bg-surface lg:max-h-none lg:border-r lg:border-b-0"
    >
      <div className="flex h-12 shrink-0 items-center justify-between border-b border-surface-border px-3">
        <div className="flex min-w-0 items-center gap-2">
          <MessageSquareText className="size-4 shrink-0 text-muted-foreground" aria-hidden="true" />
          <h2 className="truncate text-body font-medium text-foreground">
            {t(($) => $.page.title)}
          </h2>
          {rooms.length > 0 ? (
            <span className="font-mono text-caption tabular-nums text-muted-foreground">
              {rooms.length}
            </span>
          ) : null}
        </div>
        <Button
          type="button"
          size="icon-sm"
          variant="ghost"
          aria-label={t(($) => $.actions.new_room)}
          data-testid="room-create-open"
          onClick={onCreate}
        >
          <Plus aria-hidden="true" />
        </Button>
      </div>

      {rooms.length > 0 ? (
        <div className="relative border-b border-surface-border px-3 py-2">
          <Search
            className="pointer-events-none absolute left-5 top-1/2 size-3.5 -translate-y-1/2 text-muted-foreground"
            aria-hidden="true"
          />
          <Input
            type="search"
            value={query}
            onChange={(event) => setQuery(event.target.value)}
            placeholder={t(($) => $.list.search)}
            aria-label={t(($) => $.list.search)}
            className="pl-7"
          />
        </div>
      ) : null}

      {valueReviewRooms.length > 0 ? (
        <section className="border-b border-surface-border px-3 py-2" aria-labelledby="room-value-review-label">
          <h3 id="room-value-review-label" className="mb-1 text-caption font-medium text-muted-foreground">
            {t(($) => $.list.value_review)}
          </h3>
          <ol className="space-y-0.5">
            {valueReviewRooms.map((room) => (
              <li key={room.id}>
                <button
                  type="button"
                  className="flex w-full items-center gap-2 rounded-md px-1.5 py-1 text-left outline-none hover:bg-surface-hover focus-visible:ring-2 focus-visible:ring-ring"
                  onClick={() => onSelect(room.id)}
                >
                  <span className="min-w-0 flex-1 truncate text-caption font-medium text-foreground">
                    {room.title}
                  </span>
                  <span className="shrink-0 font-mono text-micro tabular-nums text-muted-foreground">
                    {t(($) => $.list.value_score, {
                      accepted: room.value?.accepted_outcomes ?? 0,
                      repeats: room.value?.repeat_run_count ?? 0,
                    })}
                  </span>
                </button>
              </li>
            ))}
          </ol>
        </section>
      ) : null}

      <div data-testid="room-list-scroll" className="min-h-0 overflow-y-auto p-2">
        {loading ? (
          <div className="space-y-2 p-1" aria-label={t(($) => $.states.loading)}>
            {[0, 1, 2].map((item) => (
              <Skeleton key={item} className="h-16 w-full rounded-md" />
            ))}
          </div>
        ) : rooms.length === 0 ? (
          <div className="px-3 py-8 text-center">
            <p className="text-body font-medium text-foreground">
              {t(($) => $.states.empty_title)}
            </p>
            <p className="mt-1 text-caption leading-5 text-muted-foreground">
              {t(($) => $.states.empty_description)}
            </p>
            <Button className="mt-4" size="sm" onClick={onCreate}>
              <Plus data-icon="inline-start" aria-hidden="true" />
              {t(($) => $.actions.new_room)}
            </Button>
          </div>
        ) : filteredRooms.length === 0 ? (
          <p className="px-3 py-8 text-center text-caption text-muted-foreground">
            {t(($) => $.list.no_results)}
          </p>
        ) : (
          <ul className="space-y-0.5">
            {filteredRooms.map((room) => {
              const selected = room.id === selectedId;
              return (
                <li key={room.id}>
                  <button
                    type="button"
                    data-testid={`room-list-item-${room.id}`}
                    className={cn(
                      "w-full rounded-md px-2.5 py-2 text-left outline-none transition-colors focus-visible:ring-2 focus-visible:ring-ring",
                      selected
                        ? "bg-surface-selected text-foreground"
                        : "text-muted-foreground hover:bg-surface-hover hover:text-foreground",
                    )}
                    onClick={() => onSelect(room.id)}
                  >
                    <span className="flex items-center justify-between gap-2">
                      <span className="min-w-0 truncate text-body font-medium">
                        {room.title}
                      </span>
                      <span
                        className={cn(
                          "size-1.5 shrink-0 rounded-full",
                          roomStatusDotClass(room.status),
                        )}
                        aria-label={t(($) => $.status[room.status])}
                      />
                    </span>
                    <RoomValueLine room={room} timeAgo={timeAgo} />
                  </button>
                </li>
              );
            })}
          </ul>
        )}
      </div>
    </aside>
  );
}

function RoomValueLine({
  room,
  timeAgo,
}: {
  readonly room: Room;
  readonly timeAgo: (value: string) => string;
}) {
  const { t } = useT("rooms");
  const value = room.value;
  const failurePhase =
    value?.last_run_phase === "failed" || value?.last_run_phase === "refused"
      ? value.last_run_phase
      : null;

  return (
    <span className="mt-1 block space-y-0.5 text-caption">
      <span className="flex items-center justify-between gap-2">
        <span className={cn("truncate", failurePhase ? "text-warning" : undefined)}>
          {failurePhase
            ? t(($) => $.phase[failurePhase])
            : value?.last_accepted_at
              ? t(($) => $.list.last_accepted, { time: timeAgo(value.last_accepted_at) })
              : room.objective}
        </span>
        <span className="shrink-0 font-mono tabular-nums">
          {value?.last_run_at ? timeAgo(value.last_run_at) : timeAgo(room.updated_at)}
        </span>
      </span>
      {value?.last_run_at || room.next_wake_at ? (
        <span className="flex items-center justify-between gap-2 text-micro text-muted-foreground">
          <span>
            {t(($) => $.list.last_cost, { count: value?.last_run_cost_ticks ?? 0 })}
          </span>
          {room.next_wake_at ? (
            <span className="truncate">
              {t(($) => $.list.next_scheduled, { time: timeAgo(room.next_wake_at) })}
            </span>
          ) : null}
        </span>
      ) : null}
    </span>
  );
}
