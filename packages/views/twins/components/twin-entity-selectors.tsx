"use client";

import { useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { Bot, CircleDot, FolderKanban, Search } from "lucide-react";
import { agentListOptions } from "@multica/core/workspace/queries";
import { projectListOptions } from "@multica/core/projects/queries";
import { twinIssueSelectorOptions } from "@multica/core/twins";
import type { Agent, Issue, Project } from "@multica/core/types";
import { Badge } from "@multica/ui/components/ui/badge";
import { Button } from "@multica/ui/components/ui/button";
import { ActorAvatar } from "../../common/actor-avatar";
import { matchesPinyin } from "../../editor/extensions/pinyin-match";
import {
  PickerEmpty,
  PickerItem,
  PropertyPicker,
} from "../../issues/components/pickers/property-picker";
import { ProjectIcon } from "../../projects/components/project-icon";
import { useT } from "../../i18n";

const TRIGGER_CLASS = "flex h-9 w-full min-w-0 items-center gap-2 rounded-md border border-input bg-background px-3 text-left text-body shadow-xs outline-none focus-visible:border-ring focus-visible:ring-2 focus-visible:ring-ring/30 disabled:cursor-not-allowed disabled:opacity-50";

export type TwinAgentSelection = Pick<Agent, "id" | "name" | "archived_at">;
export type TwinProjectSelection = Pick<Project, "id" | "title" | "status" | "icon">;
export type TwinIssueSelection = Pick<Issue, "id" | "identifier" | "title" | "project_id" | "status">;

interface SelectorProps<T> {
  wsId: string;
  value: T | null;
  onChange: (value: T | null) => void;
  disabled?: boolean;
  optional?: boolean;
  ariaLabel: string;
}

export function TwinAgentSelector({ wsId, value, onChange, disabled, ariaLabel }: SelectorProps<TwinAgentSelection>) {
  const { t } = useT("twins");
  const [open, setOpen] = useState(false);
  const [filter, setFilter] = useState("");
  const agentsQuery = useQuery(agentListOptions(wsId));
  const agents = agentsQuery.data ?? [];
  const normalized = filter.trim().toLowerCase();
  const filtered = agents.filter((agent) =>
    !normalized || agent.name.toLowerCase().includes(normalized) || matchesPinyin(agent.name, normalized),
  );

  return (
    <PropertyPicker
      open={open}
      onOpenChange={setOpen}
      width="w-72"
      align="start"
      searchable
      searchPlaceholder={t(($) => $.use.entity_search_agent)}
      onSearchChange={setFilter}
      triggerRender={<button type="button" className={TRIGGER_CLASS} aria-label={ariaLabel} disabled={disabled} />}
      trigger={value ? (
        <><ActorAvatar actorType="agent" actorId={value.id} size="sm" /><span className="truncate">{value.name}</span></>
      ) : (
        <><Bot className="size-4 shrink-0 text-muted-foreground" aria-hidden="true" /><span className="truncate text-muted-foreground">{t(($) => $.use.select_agent)}</span></>
      )}
    >
      {agentsQuery.isError ? <SelectorError onRetry={() => void agentsQuery.refetch()} /> : agentsQuery.isPending ? <SelectorLoading /> : filtered.length === 0 ? <PickerEmpty /> : filtered.map((agent) => (
        <PickerItem
          key={agent.id}
          selected={agent.id === value?.id}
          disabled={Boolean(agent.archived_at)}
          tooltip={agent.archived_at ? t(($) => $.use.archived_agent) : undefined}
          onClick={() => {
            onChange({ id: agent.id, name: agent.name, archived_at: agent.archived_at });
            setOpen(false);
          }}
        >
          <ActorAvatar actorType="agent" actorId={agent.id} size="sm" />
          <span className="min-w-0 flex-1 truncate">{agent.name}</span>
          {agent.archived_at ? <Badge variant="outline">{t(($) => $.use.archived)}</Badge> : null}
        </PickerItem>
      ))}
    </PropertyPicker>
  );
}

export function TwinProjectSelector({ wsId, value, onChange, disabled, optional, ariaLabel }: SelectorProps<TwinProjectSelection>) {
  const { t } = useT("twins");
  const [open, setOpen] = useState(false);
  const [filter, setFilter] = useState("");
  const projectsQuery = useQuery(projectListOptions(wsId));
  const projects = projectsQuery.data ?? [];
  const normalized = filter.trim().toLowerCase();
  const filtered = projects.filter((project) =>
    !normalized || project.title.toLowerCase().includes(normalized) || matchesPinyin(project.title, normalized),
  );

  return (
    <PropertyPicker
      open={open}
      onOpenChange={setOpen}
      width="w-72"
      align="start"
      searchable
      searchPlaceholder={t(($) => $.use.entity_search_project)}
      onSearchChange={setFilter}
      triggerRender={<button type="button" className={TRIGGER_CLASS} aria-label={ariaLabel} disabled={disabled} />}
      trigger={value ? (
        <><ProjectIcon project={value} size="sm" /><span className="truncate">{value.title}</span></>
      ) : (
        <><FolderKanban className="size-4 shrink-0 text-muted-foreground" aria-hidden="true" /><span className="truncate text-muted-foreground">{t(($) => $.use.select_project)}</span></>
      )}
    >
      {optional ? (
        <PickerItem emptyValue selected={!value} onClick={() => { onChange(null); setOpen(false); }}>
          <FolderKanban className="size-4 text-muted-foreground" aria-hidden="true" />
          <span className="text-muted-foreground">{t(($) => $.use.no_project)}</span>
        </PickerItem>
      ) : null}
      {projectsQuery.isError ? <SelectorError onRetry={() => void projectsQuery.refetch()} /> : projectsQuery.isPending ? <SelectorLoading /> : filtered.length === 0 ? <PickerEmpty /> : filtered.map((project) => (
        <PickerItem
          key={project.id}
          selected={project.id === value?.id}
          onClick={() => {
            onChange({ id: project.id, title: project.title, status: project.status, icon: project.icon });
            setOpen(false);
          }}
        >
          <ProjectIcon project={project} size="sm" />
          <span className="min-w-0 flex-1 truncate">{project.title}</span>
          <span className="text-caption text-muted-foreground">{project.status.replaceAll("_", " ")}</span>
        </PickerItem>
      ))}
    </PropertyPicker>
  );
}

export function TwinIssueSelector({ wsId, value, onChange, disabled, optional, ariaLabel }: SelectorProps<TwinIssueSelection>) {
  const { t } = useT("twins");
  const [open, setOpen] = useState(false);
  const [search, setSearch] = useState("");
  const query = useQuery(twinIssueSelectorOptions(wsId, search));
  const issues = useMemo(() => {
    const found = query.data ?? [];
    return value && !found.some((issue) => issue.id === value.id) ? [value, ...found] : found;
  }, [query.data, value]);

  return (
    <PropertyPicker
      open={open}
      onOpenChange={setOpen}
      width="w-80"
      align="start"
      searchable
      searchPlaceholder={t(($) => $.use.entity_search_issue)}
      onSearchChange={setSearch}
      triggerRender={<button type="button" className={TRIGGER_CLASS} aria-label={ariaLabel} disabled={disabled} />}
      trigger={value ? (
        <><CircleDot className="size-4 shrink-0 text-muted-foreground" aria-hidden="true" /><span className="shrink-0 text-caption text-muted-foreground">{value.identifier}</span><span className="truncate">{value.title}</span></>
      ) : (
        <><Search className="size-4 shrink-0 text-muted-foreground" aria-hidden="true" /><span className="truncate text-muted-foreground">{t(($) => $.use.select_issue)}</span></>
      )}
    >
      {optional ? (
        <PickerItem emptyValue selected={!value} onClick={() => { onChange(null); setOpen(false); }}>
          <CircleDot className="size-4 text-muted-foreground" aria-hidden="true" />
          <span className="text-muted-foreground">{t(($) => $.use.no_issue)}</span>
        </PickerItem>
      ) : null}
      {query.isError ? <SelectorError onRetry={() => void query.refetch()} /> : !search.trim() && !value ? (
        <div className="px-2 py-3 text-center text-body text-muted-foreground">{t(($) => $.use.issue_search_hint)}</div>
      ) : query.isFetching && issues.length === 0 ? (
        <div className="px-2 py-3 text-center text-body text-muted-foreground">{t(($) => $.use.searching)}</div>
      ) : issues.length === 0 ? <PickerEmpty /> : issues.map((issue) => (
        <PickerItem
          key={issue.id}
          selected={issue.id === value?.id}
          onClick={() => {
            onChange({
              id: issue.id,
              identifier: issue.identifier,
              title: issue.title,
              project_id: issue.project_id,
              status: issue.status,
            });
            setOpen(false);
          }}
        >
          <span className="shrink-0 text-caption text-muted-foreground">{issue.identifier}</span>
          <span className="min-w-0 flex-1 truncate">{issue.title}</span>
          <span className="text-caption text-muted-foreground">{issue.status.replaceAll("_", " ")}</span>
        </PickerItem>
      ))}
    </PropertyPicker>
  );
}

function SelectorError({ onRetry }: { onRetry: () => void }) {
  const { t } = useT("twins");
  return (
    <div className="grid gap-2 px-2 py-3 text-center text-body text-destructive" role="alert">
      <p>{t(($) => $.use.entity_load_error)}</p>
      <Button variant="outline" size="sm" onClick={onRetry}>{t(($) => $.use.entity_retry)}</Button>
    </div>
  );
}

function SelectorLoading() {
  const { t } = useT("twins");
  return <p role="status" className="px-2 py-3 text-center text-body text-muted-foreground">{t(($) => $.use.searching)}</p>;
}
