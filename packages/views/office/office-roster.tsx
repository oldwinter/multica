import { useEffect, useLayoutEffect, useMemo, useRef } from "react";
import {
  defaultRangeExtractor,
  useVirtualizer,
  type VirtualItem,
} from "@tanstack/react-virtual";
import {
  ArrowUpRight,
  Bot,
  Flag,
  ShieldQuestion,
  UserRound,
} from "lucide-react";
import type {
  OfficeAgent,
  OfficeIssue,
  OfficeSnapshot,
  OfficeSquad,
  OfficeSubjectRef,
} from "@multica/core/office";
import { useWorkspacePaths } from "@multica/core/paths";
import { ActorAvatar } from "@multica/ui/components/common/actor-avatar";
import { cn } from "@multica/ui/lib/utils";
import { useT } from "../i18n";
import { AppLink } from "../navigation";
import { OfficePresence } from "./office-presence";
import { officeSnapshotCounts } from "./office-counts";
import { OfficeCompactIdentifier } from "./office-identity";

export type OfficeRosterTab = "agents" | "squads" | "issues";

const ROSTER_TABS: readonly OfficeRosterTab[] = [
  "agents",
  "squads",
  "issues",
];

const VIRTUALIZATION_THRESHOLD = 12;
const VIRTUAL_OVERSCAN = 4;
const INITIAL_VIRTUAL_RECT = { width: 300, height: 560 };
const ROW_HEIGHT_ESTIMATE: Record<OfficeRosterTab, number> = {
  agents: 112,
  squads: 152,
  issues: 132,
};

function initials(name: string): string {
  return name
    .trim()
    .split(/\s+/)
    .map((part) => part[0] ?? "")
    .join("")
    .slice(0, 2)
    .toUpperCase();
}

export function officeSubjectKey(subject: OfficeSubjectRef): string {
  return `${subject.kind}:${subject.id}`;
}

export function officeRosterTabForSubject(
  subject: OfficeSubjectRef,
): OfficeRosterTab {
  switch (subject.kind) {
    case "agent":
      return "agents";
    case "squad":
      return "squads";
    case "issue":
      return "issues";
    default: {
      const exhaustive: never = subject;
      return exhaustive;
    }
  }
}

function subjectsForTab(
  tab: OfficeRosterTab,
  snapshot: OfficeSnapshot,
): readonly OfficeSubjectRef[] {
  switch (tab) {
    case "agents":
      return snapshot.agents.map((agent) => ({ kind: "agent", id: agent.id }));
    case "squads":
      return snapshot.squads.map((squad) => ({ kind: "squad", id: squad.id }));
    case "issues":
      return snapshot.activeIssues.map((issue) => ({ kind: "issue", id: issue.id }));
    default: {
      const exhaustive: never = tab;
      return exhaustive;
    }
  }
}

function tabLabel(
  tab: OfficeRosterTab,
  count: number,
  t: ReturnType<typeof useT<"office">>["t"],
): string {
  switch (tab) {
    case "agents":
      return `${t(($) => $.roster.tabs.agents)} ${count}`;
    case "squads":
      return `${t(($) => $.roster.tabs.squads)} ${count}`;
    case "issues":
      return `${t(($) => $.roster.tabs.issues)} ${count}`;
    default: {
      const exhaustive: never = tab;
      return exhaustive;
    }
  }
}

function emptyLabel(
  tab: OfficeRosterTab,
  t: ReturnType<typeof useT<"office">>["t"],
) {
  switch (tab) {
    case "agents":
      return t(($) => $.roster.empty.agents);
    case "squads":
      return t(($) => $.roster.empty.squads);
    case "issues":
      return t(($) => $.roster.empty.issues);
    default: {
      const exhaustive: never = tab;
      return exhaustive;
    }
  }
}

export interface OfficeRosterHandle {
  readonly setActiveTab: (tab: OfficeRosterTab) => void;
  readonly restoreFocus: (subject: OfficeSubjectRef) => void;
}

export function OfficeRoster({
  snapshot,
  activeTab,
  onActiveTabChange,
  onSelect,
  rovingSubject,
  onRovingSubjectChange,
  resolveStatusLabel,
  restoreFocusSubject,
  onFocusRestored,
}: {
  readonly snapshot: OfficeSnapshot;
  readonly activeTab: OfficeRosterTab;
  readonly onActiveTabChange: (tab: OfficeRosterTab) => void;
  readonly onSelect: (subject: OfficeSubjectRef) => void;
  readonly rovingSubject: OfficeSubjectRef | null;
  readonly onRovingSubjectChange: (subject: OfficeSubjectRef) => void;
  readonly resolveStatusLabel: (statusKey: string) => string;
  readonly restoreFocusSubject: OfficeSubjectRef | null;
  readonly onFocusRestored: () => void;
}) {
  const { t } = useT("office");
  const tabRefs = useRef<Record<OfficeRosterTab, HTMLButtonElement | null>>({
    agents: null,
    squads: null,
    issues: null,
  });
  const scrollRef = useRef<HTMLDivElement | null>(null);
  const visibleRowRefs = useRef(new Map<string, HTMLButtonElement>());
  const pendingRowFocusRef = useRef<OfficeSubjectRef | null>(null);
  const completedRestoreKeyRef = useRef<string | null>(null);
  const subjects = useMemo(
    () => subjectsForTab(activeTab, snapshot),
    [activeTab, snapshot],
  );
  const firstSubject = subjects[0] ?? null;
  const activeRovingSubject =
    rovingSubject && officeRosterTabForSubject(rovingSubject) === activeTab
      ? rovingSubject
      : firstSubject;
  const activeRovingIndex = activeRovingSubject
    ? subjects.findIndex(
        (subject) =>
          officeSubjectKey(subject) === officeSubjectKey(activeRovingSubject),
      )
    : -1;
  const virtualized = subjects.length > VIRTUALIZATION_THRESHOLD;
  const rowVirtualizer = useVirtualizer({
    count: virtualized ? subjects.length : 0,
    getScrollElement: () => scrollRef.current,
    estimateSize: () => ROW_HEIGHT_ESTIMATE[activeTab],
    getItemKey: (index) => {
      const subject = subjects[index];
      return subject ? officeSubjectKey(subject) : index;
    },
    rangeExtractor: (range) => {
      const indexes = defaultRangeExtractor(range);
      if (activeRovingIndex < 0 || indexes.includes(activeRovingIndex)) {
        return indexes;
      }
      return [...indexes, activeRovingIndex].sort((left, right) => left - right);
    },
    overscan: VIRTUAL_OVERSCAN,
    initialRect: INITIAL_VIRTUAL_RECT,
  });
  const virtualItems = rowVirtualizer.getVirtualItems();

  useEffect(() => {
    if (!virtualized || activeRovingIndex <= 0) return;
    const timeout = window.setTimeout(() => {
      rowVirtualizer.scrollToIndex(activeRovingIndex, { align: "auto" });
    }, 0);
    return () => window.clearTimeout(timeout);
  }, [activeRovingIndex, activeTab, rowVirtualizer, virtualized]);

  useLayoutEffect(() => {
    const pending = pendingRowFocusRef.current;
    if (!pending) return;
    const element = visibleRowRefs.current.get(officeSubjectKey(pending));
    if (!element) return;
    pendingRowFocusRef.current = null;
    element.focus();
  }, [activeRovingSubject, virtualItems]);

  useLayoutEffect(() => {
    if (!restoreFocusSubject) {
      completedRestoreKeyRef.current = null;
      return;
    }
    if (virtualized) return;
    const key = officeSubjectKey(restoreFocusSubject);
    if (completedRestoreKeyRef.current === key) return;
    const element = visibleRowRefs.current.get(key);
    if (!element) return;
    completedRestoreKeyRef.current = key;
    element.focus();
    onFocusRestored();
  }, [onFocusRestored, restoreFocusSubject, virtualItems, virtualized]);

  useEffect(() => {
    if (!virtualized || !restoreFocusSubject) return;
    const key = officeSubjectKey(restoreFocusSubject);
    if (completedRestoreKeyRef.current === key) return;
    const timeout = window.setTimeout(() => {
      const element = visibleRowRefs.current.get(key);
      if (!element) return;
      completedRestoreKeyRef.current = key;
      element.focus();
      onFocusRestored();
    }, 0);
    return () => window.clearTimeout(timeout);
  }, [onFocusRestored, restoreFocusSubject, virtualItems, virtualized]);

  const snapshotCounts = officeSnapshotCounts(snapshot);
  const counts: Record<OfficeRosterTab, number> = {
    agents: snapshotCounts.agents.total,
    squads: snapshotCounts.squads.total,
    issues: snapshotCounts.issues.total,
  };

  const moveTabFocus = (
    event: React.KeyboardEvent<HTMLButtonElement>,
    current: OfficeRosterTab,
  ) => {
    const currentIndex = ROSTER_TABS.indexOf(current);
    let targetIndex: number | null = null;
    if (event.key === "ArrowRight") {
      targetIndex = (currentIndex + 1) % ROSTER_TABS.length;
    } else if (event.key === "ArrowLeft") {
      targetIndex =
        (currentIndex - 1 + ROSTER_TABS.length) % ROSTER_TABS.length;
    } else if (event.key === "Home") {
      targetIndex = 0;
    } else if (event.key === "End") {
      targetIndex = ROSTER_TABS.length - 1;
    }
    if (targetIndex === null) return;
    event.preventDefault();
    const target = ROSTER_TABS[targetIndex];
    if (!target) return;
    onActiveTabChange(target);
    tabRefs.current[target]?.focus();
  };

  const moveRowFocus = (
    event: React.KeyboardEvent<HTMLButtonElement>,
    subject: OfficeSubjectRef,
  ) => {
    const currentIndex = subjects.findIndex(
      (candidate) => officeSubjectKey(candidate) === officeSubjectKey(subject),
    );
    if (currentIndex < 0 || subjects.length === 0) return;
    let targetIndex: number | null = null;
    if (event.key === "ArrowDown") {
      targetIndex = (currentIndex + 1) % subjects.length;
    } else if (event.key === "ArrowUp") {
      targetIndex = (currentIndex - 1 + subjects.length) % subjects.length;
    } else if (event.key === "Home") {
      targetIndex = 0;
    } else if (event.key === "End") {
      targetIndex = subjects.length - 1;
    }
    if (targetIndex === null) return;
    event.preventDefault();
    const target = subjects[targetIndex];
    if (!target) return;
    pendingRowFocusRef.current = target;
    if (virtualized) {
      rowVirtualizer.scrollToIndex(targetIndex, {
        align:
          event.key === "Home"
            ? "start"
            : event.key === "End"
              ? "end"
              : "auto",
      });
    }
    onRovingSubjectChange(target);
    if (!virtualized) {
      visibleRowRefs.current.get(officeSubjectKey(target))?.focus();
      pendingRowFocusRef.current = null;
    }
  };

  const registerVisibleRow = (
    subject: OfficeSubjectRef,
    element: HTMLButtonElement | null,
  ) => {
    const key = officeSubjectKey(subject);
    if (element) visibleRowRefs.current.set(key, element);
    else visibleRowRefs.current.delete(key);
  };

  const visibleRows: readonly {
    readonly index: number;
    readonly virtualItem: VirtualItem | null;
  }[] = virtualized
    ? virtualItems.map((virtualItem) => ({
        index: virtualItem.index,
        virtualItem,
      }))
    : subjects.map((_, index) => ({ index, virtualItem: null }));

  const renderRow = ({
    index,
    virtualItem,
  }: {
    readonly index: number;
    readonly virtualItem: VirtualItem | null;
  }) => {
    const interaction = {
      onRegister: registerVisibleRow,
      onFocus: onRovingSubjectChange,
      onKeyDown: moveRowFocus,
      onSelect,
    };
    const layout = {
      virtualItem,
      measureElement: rowVirtualizer.measureElement,
    };
    if (activeTab === "agents") {
      const agent = snapshot.agents[index];
      if (!agent) return null;
      return (
        <AgentRosterRow
          key={agent.id}
          {...interaction}
          {...layout}
          agent={agent}
          snapshot={snapshot}
          tabIndex={
            activeRovingSubject?.kind === "agent" &&
            activeRovingSubject.id === agent.id
              ? 0
              : -1
          }
        />
      );
    }
    if (activeTab === "squads") {
      const squad = snapshot.squads[index];
      if (!squad) return null;
      return (
        <SquadRosterRow
          key={squad.id}
          {...interaction}
          {...layout}
          squad={squad}
          snapshot={snapshot}
          tabIndex={
            activeRovingSubject?.kind === "squad" &&
            activeRovingSubject.id === squad.id
              ? 0
              : -1
          }
        />
      );
    }
    const issue = snapshot.activeIssues[index];
    if (!issue) return null;
    return (
      <IssueRosterRow
        key={issue.id}
        {...interaction}
        {...layout}
        issue={issue}
        snapshot={snapshot}
        resolveStatusLabel={resolveStatusLabel}
        tabIndex={
          activeRovingSubject?.kind === "issue" &&
          activeRovingSubject.id === issue.id
            ? 0
            : -1
        }
      />
    );
  };

  return (
    <div
      className="flex min-h-0 flex-1 flex-col max-md:pe-chat-launcher"
      data-testid="office-roster"
    >
      <div
        role="tablist"
        aria-label={t(($) => $.roster.title)}
        className="grid grid-cols-3 border-b border-surface-border px-1"
      >
        {ROSTER_TABS.map((tab) => (
          <button
            key={tab}
            ref={(element) => {
              tabRefs.current[tab] = element;
            }}
            type="button"
            role="tab"
            id={`office-tab-${tab}`}
            aria-controls={`office-tabpanel-${tab}`}
            aria-selected={activeTab === tab}
            tabIndex={activeTab === tab ? 0 : -1}
            className={cn(
              "relative min-h-11 min-w-0 break-words px-1 text-center text-caption font-medium text-muted-foreground outline-none hover:bg-surface-hover hover:text-foreground focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-ring",
              activeTab === tab &&
                "text-foreground after:absolute after:inset-x-2 after:bottom-0 after:h-px after:bg-brand",
            )}
            onClick={() => onActiveTabChange(tab)}
            onKeyDown={(event) => moveTabFocus(event, tab)}
          >
            {tabLabel(tab, counts[tab], t)}
          </button>
        ))}
      </div>

      <div
        ref={scrollRef}
        role="tabpanel"
        id={`office-tabpanel-${activeTab}`}
        aria-labelledby={`office-tab-${activeTab}`}
        className="min-h-0 flex-1 overflow-y-auto"
      >
        {subjects.length === 0 ? (
          <p className="p-4 text-body text-muted-foreground">
            {emptyLabel(activeTab, t)}
          </p>
        ) : (
          <ul
            className={virtualized ? "relative" : "divide-y divide-surface-border/70"}
            style={
              virtualized
                ? { height: `${rowVirtualizer.getTotalSize()}px` }
                : undefined
            }
          >
            {visibleRows.map(renderRow)}
          </ul>
        )}
      </div>
    </div>
  );
}

interface RowInteractionProps {
  readonly tabIndex: number;
  readonly onRegister: (
    subject: OfficeSubjectRef,
    element: HTMLButtonElement | null,
  ) => void;
  readonly onFocus: (subject: OfficeSubjectRef) => void;
  readonly onKeyDown: (
    event: React.KeyboardEvent<HTMLButtonElement>,
    subject: OfficeSubjectRef,
  ) => void;
  readonly onSelect: (subject: OfficeSubjectRef) => void;
}

interface RowLayoutProps {
  readonly virtualItem: VirtualItem | null;
  readonly measureElement: (element: Element | null) => void;
}

function RosterRowFrame({
  virtualItem,
  measureElement,
  children,
}: RowLayoutProps & { readonly children: React.ReactNode }) {
  return (
    <li
      ref={virtualItem ? measureElement : undefined}
      data-index={virtualItem?.index}
      data-testid="office-roster-row"
      className={
        virtualItem
          ? "absolute left-0 top-0 w-full border-b border-surface-border/70"
          : undefined
      }
      style={
        virtualItem
          ? { transform: `translateY(${virtualItem.start}px)` }
          : undefined
      }
    >
      {children}
    </li>
  );
}

function RosterSelectionButton({
  subject,
  label,
  tabIndex,
  onRegister,
  onFocus,
  onKeyDown,
  onSelect,
  children,
}: RowInteractionProps & {
  readonly subject: OfficeSubjectRef;
  readonly label: string;
  readonly children: React.ReactNode;
}) {
  return (
    <button
      ref={(element) => onRegister(subject, element)}
      type="button"
      aria-label={label}
      tabIndex={tabIndex}
      data-office-row-key={officeSubjectKey(subject)}
      className="flex min-h-11 w-full min-w-0 flex-col gap-2 px-3 py-3 text-left outline-none transition-colors hover:bg-surface-hover focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-ring"
      onFocus={() => onFocus(subject)}
      onKeyDown={(event) => onKeyDown(event, subject)}
      onClick={() => onSelect(subject)}
    >
      {children}
    </button>
  );
}

function AgentRosterRow({
  agent,
  snapshot,
  virtualItem,
  measureElement,
  ...interaction
}: RowInteractionProps & RowLayoutProps & {
  readonly agent: OfficeAgent;
  readonly snapshot: OfficeSnapshot;
}) {
  const { t } = useT("office");
  const paths = useWorkspacePaths();
  const subject: OfficeSubjectRef = { kind: "agent", id: agent.id };
  return (
    <RosterRowFrame
      virtualItem={virtualItem}
      measureElement={measureElement}
    >
      <RosterSelectionButton
        {...interaction}
        subject={subject}
        label={t(($) => $.a11y.agent_row, { name: agent.name })}
      >
        <div className="flex min-w-0 items-center gap-2">
          <ActorAvatar
            name={agent.name}
            initials={initials(agent.name)}
            avatarUrl={agent.avatarUrl}
            isAgent
            size="md"
          />
          <span className="min-w-0 flex-1 break-words text-body font-semibold text-foreground">
            {agent.name}
          </span>
        </div>
        <OfficePresence
          availability={agent.availability}
          workload={agent.workload}
          compact
        />
      </RosterSelectionButton>
      {agent.activeIssueIds.length > 0 ? (
        <div className="flex flex-wrap gap-1 px-3 pb-3">
          {agent.activeIssueIds.map((issueId) => {
            const issue = snapshot.activeIssues.find(
              (candidate) => candidate.id === issueId,
            );
            const label =
              issue?.kind === "resolved" ? issue.identifier : issueId;
            return (
              <AppLink
                key={issueId}
                href={paths.issueDetail(issueId)}
                className="inline-flex min-h-7 items-center gap-1 rounded-md px-1.5 font-mono text-caption text-brand outline-none hover:bg-surface-hover focus-visible:ring-2 focus-visible:ring-ring"
              >
                {label}
                <ArrowUpRight className="size-3" aria-hidden="true" />
              </AppLink>
            );
          })}
        </div>
      ) : null}
    </RosterRowFrame>
  );
}

function SquadRosterRow({
  squad,
  snapshot,
  virtualItem,
  measureElement,
  ...interaction
}: RowInteractionProps & RowLayoutProps & {
  readonly squad: OfficeSquad;
  readonly snapshot: OfficeSnapshot;
}) {
  const { t } = useT("office");
  const subject: OfficeSubjectRef = { kind: "squad", id: squad.id };
  const leader = snapshot.agents.find(
    (agent) => agent.id === squad.leaderAgentId,
  );
  return (
    <RosterRowFrame
      virtualItem={virtualItem}
      measureElement={measureElement}
    >
      <RosterSelectionButton
        {...interaction}
        subject={subject}
        label={t(($) => $.a11y.squad_row, { name: squad.name })}
      >
        <div className="flex min-w-0 items-center gap-2">
          <ActorAvatar
            name={squad.name}
            initials={initials(squad.name)}
            avatarUrl={squad.avatarUrl}
            isSquad
            size="md"
          />
          <span className="min-w-0 flex-1 break-words text-body font-semibold text-foreground">
            {squad.name}
          </span>
        </div>
        <div className="flex flex-wrap gap-x-3 gap-y-1 text-caption text-muted-foreground">
          <span>{t(($) => $.roster.members, { count: squad.memberCount })}</span>
          <span>
            {leader
              ? t(($) => $.roster.leader, { name: leader.name })
              : t(($) => $.roster.leader_unknown)}
          </span>
        </div>
        <div className="w-full">
          <div className="mb-1 text-caption font-medium text-foreground">
            {t(($) => $.roster.preview)}
          </div>
          <ul className="space-y-1">
            {squad.memberPreview.map((member) => {
              const agentName =
                member.kind === "agent"
                  ? snapshot.agents.find((agent) => agent.id === member.id)?.name
                  : undefined;
              return (
                <li
                  key={`${member.kind}:${member.id}`}
                  className="flex min-w-0 items-center gap-1.5 text-caption text-muted-foreground"
                >
                  {member.kind === "agent" ? (
                    <Bot className="size-3.5 shrink-0" aria-hidden="true" />
                  ) : member.kind === "member" ? (
                    <UserRound className="size-3.5 shrink-0" aria-hidden="true" />
                  ) : (
                    <ShieldQuestion className="size-3.5 shrink-0" aria-hidden="true" />
                  )}
                  {agentName ? (
                    <span className="min-w-0 break-words font-medium text-foreground">
                      {agentName}
                    </span>
                  ) : (
                    <OfficeCompactIdentifier
                      id={member.id}
                      className="min-w-0 text-caption"
                    />
                  )}
                  <span className="min-w-0 break-words">{member.role}</span>
                </li>
              );
            })}
          </ul>
        </div>
      </RosterSelectionButton>
    </RosterRowFrame>
  );
}

function IssueRosterRow({
  issue,
  snapshot,
  resolveStatusLabel,
  virtualItem,
  measureElement,
  ...interaction
}: RowInteractionProps & RowLayoutProps & {
  readonly issue: OfficeIssue;
  readonly snapshot: OfficeSnapshot;
  readonly resolveStatusLabel: (statusKey: string) => string;
}) {
  const { t } = useT("office");
  const paths = useWorkspacePaths();
  const subject: OfficeSubjectRef = { kind: "issue", id: issue.id };
  const label = issue.kind === "resolved" ? issue.identifier : issue.id;
  const assignedSquad =
    issue.kind === "resolved" && issue.assignedSquadId
      ? snapshot.squads.find((squad) => squad.id === issue.assignedSquadId)
      : null;
  return (
    <RosterRowFrame
      virtualItem={virtualItem}
      measureElement={measureElement}
    >
      <RosterSelectionButton
        {...interaction}
        subject={subject}
        label={t(($) => $.a11y.issue_row, { name: label })}
      >
        <div className="flex w-full min-w-0 items-start gap-2">
          <Flag className="mt-0.5 size-4 shrink-0 text-warning" aria-hidden="true" />
          <div className="min-w-0 flex-1">
            <div className="break-all font-mono text-caption font-semibold text-foreground">
              {label}
            </div>
            {issue.kind === "resolved" ? (
              <div className="mt-0.5 break-words text-body font-medium text-foreground">
                {issue.title}
              </div>
            ) : (
              <div className="mt-0.5 text-caption text-muted-foreground">
                {t(($) => $.roster.unresolved_issue)}
              </div>
            )}
          </div>
        </div>
        <div className="flex flex-wrap gap-x-3 gap-y-1 text-caption text-muted-foreground">
          {issue.kind === "resolved" ? (
            <span>{resolveStatusLabel(issue.status)}</span>
          ) : null}
          {issue.executingAgentIds.length > 0 ? (
            <span>
              {t(($) => $.roster.executing_agent_count, {
                count: issue.executingAgentIds.length,
              })}
            </span>
          ) : null}
          {assignedSquad ? <span>{assignedSquad.name}</span> : null}
        </div>
      </RosterSelectionButton>
      <div className="px-3 pb-3">
        <AppLink
          href={paths.issueDetail(issue.id)}
          className="inline-flex min-h-7 items-center gap-1 rounded-md px-1.5 text-caption font-medium text-brand outline-none hover:bg-surface-hover focus-visible:ring-2 focus-visible:ring-ring"
        >
          {t(($) => $.inspector.open_issue)}
          <ArrowUpRight className="size-3" aria-hidden="true" />
        </AppLink>
      </div>
    </RosterRowFrame>
  );
}
