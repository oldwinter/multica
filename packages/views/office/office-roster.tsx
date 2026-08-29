import { useRef } from "react";
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

export type OfficeRosterTab = "agents" | "squads" | "issues";

const ROSTER_TABS: readonly OfficeRosterTab[] = [
  "agents",
  "squads",
  "issues",
];

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
  registerRow,
  rovingSubject,
  onRovingSubjectChange,
}: {
  readonly snapshot: OfficeSnapshot;
  readonly activeTab: OfficeRosterTab;
  readonly onActiveTabChange: (tab: OfficeRosterTab) => void;
  readonly onSelect: (subject: OfficeSubjectRef) => void;
  readonly registerRow: (
    subject: OfficeSubjectRef,
    element: HTMLButtonElement | null,
  ) => void;
  readonly rovingSubject: OfficeSubjectRef | null;
  readonly onRovingSubjectChange: (subject: OfficeSubjectRef) => void;
}) {
  const { t } = useT("office");
  const tabRefs = useRef<Record<OfficeRosterTab, HTMLButtonElement | null>>({
    agents: null,
    squads: null,
    issues: null,
  });
  const subjects = subjectsForTab(activeTab, snapshot);
  const firstSubject = subjects[0] ?? null;
  const activeRovingSubject =
    rovingSubject && officeRosterTabForSubject(rovingSubject) === activeTab
      ? rovingSubject
      : firstSubject;

  const counts: Record<OfficeRosterTab, number> = {
    agents: snapshot.agents.length + snapshot.overflow.agents,
    squads: snapshot.squads.length + snapshot.overflow.squads,
    issues:
      snapshot.activeIssues.length + snapshot.overflow.activeIssues,
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
    onRovingSubjectChange(target);
    const selector = `[data-office-row-key="${CSS.escape(officeSubjectKey(target))}"]`;
    document.querySelector<HTMLButtonElement>(selector)?.focus();
  };

  return (
    <div className="flex min-h-0 flex-1 flex-col" data-testid="office-roster">
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
          <ul className="divide-y divide-surface-border/70">
            {activeTab === "agents"
              ? snapshot.agents.map((agent) => (
                  <AgentRosterRow
                    key={agent.id}
                    agent={agent}
                    snapshot={snapshot}
                    tabIndex={
                      activeRovingSubject?.kind === "agent" &&
                      activeRovingSubject.id === agent.id
                        ? 0
                        : -1
                    }
                    onRegister={registerRow}
                    onFocus={onRovingSubjectChange}
                    onKeyDown={moveRowFocus}
                    onSelect={onSelect}
                  />
                ))
              : null}
            {activeTab === "squads"
              ? snapshot.squads.map((squad) => (
                  <SquadRosterRow
                    key={squad.id}
                    squad={squad}
                    snapshot={snapshot}
                    tabIndex={
                      activeRovingSubject?.kind === "squad" &&
                      activeRovingSubject.id === squad.id
                        ? 0
                        : -1
                    }
                    onRegister={registerRow}
                    onFocus={onRovingSubjectChange}
                    onKeyDown={moveRowFocus}
                    onSelect={onSelect}
                  />
                ))
              : null}
            {activeTab === "issues"
              ? snapshot.activeIssues.map((issue) => (
                  <IssueRosterRow
                    key={issue.id}
                    issue={issue}
                    snapshot={snapshot}
                    tabIndex={
                      activeRovingSubject?.kind === "issue" &&
                      activeRovingSubject.id === issue.id
                        ? 0
                        : -1
                    }
                    onRegister={registerRow}
                    onFocus={onRovingSubjectChange}
                    onKeyDown={moveRowFocus}
                    onSelect={onSelect}
                  />
                ))
              : null}
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
  ...interaction
}: RowInteractionProps & {
  readonly agent: OfficeAgent;
  readonly snapshot: OfficeSnapshot;
}) {
  const { t } = useT("office");
  const paths = useWorkspacePaths();
  const subject: OfficeSubjectRef = { kind: "agent", id: agent.id };
  return (
    <li>
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
    </li>
  );
}

function SquadRosterRow({
  squad,
  snapshot,
  ...interaction
}: RowInteractionProps & {
  readonly squad: OfficeSquad;
  readonly snapshot: OfficeSnapshot;
}) {
  const { t } = useT("office");
  const subject: OfficeSubjectRef = { kind: "squad", id: squad.id };
  const leader = snapshot.agents.find(
    (agent) => agent.id === squad.leaderAgentId,
  );
  return (
    <li>
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
            {squad.memberPreview.map((member) => (
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
                <span className="min-w-0 break-all font-mono">{member.id}</span>
                <span className="min-w-0 break-words">{member.role}</span>
              </li>
            ))}
          </ul>
        </div>
      </RosterSelectionButton>
    </li>
  );
}

function IssueRosterRow({
  issue,
  snapshot,
  ...interaction
}: RowInteractionProps & {
  readonly issue: OfficeIssue;
  readonly snapshot: OfficeSnapshot;
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
    <li>
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
            <span className="font-mono">{issue.status}</span>
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
    </li>
  );
}
