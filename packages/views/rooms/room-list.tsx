"use client";

import { useMemo, useState } from "react";
import { MessageSquareText, Plus, Search } from "lucide-react";
import { filterRooms, type Room } from "@multica/core/rooms";
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
  readonly onSelect: (roomId: string) => void;
  readonly onCreate: () => void;
}

export function RoomList({
  rooms,
  selectedId,
  loading,
  onSelect,
  onCreate,
}: RoomListProps) {
  const { t } = useT("rooms");
  const timeAgo = useTimeAgo();
  const [query, setQuery] = useState("");
  const filteredRooms = useMemo(() => filterRooms(rooms, query), [rooms, query]);

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
                    <span className="mt-1 flex items-center justify-between gap-2 text-caption">
                      <span className="truncate">
                        {room.instructions || t(($) => $.detail.empty_transcript)}
                      </span>
                      <span className="shrink-0 font-mono tabular-nums">
                        {timeAgo(room.updated_at)}
                      </span>
                    </span>
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
