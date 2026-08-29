import {
  ArrowLeft,
  ArrowUpRight,
  Bot,
  CircleHelp,
  Flag,
  LoaderCircle,
  RefreshCw,
  UserRound,
  Users,
} from "lucide-react";
import type {
  OfficeAgent,
  OfficeInspector,
  OfficeIssue,
  OfficeSnapshot,
  OfficeSquad,
  OfficeSquadMembers,
} from "@multica/core/office";
import { useWorkspacePaths } from "@multica/core/paths";
import { ActorAvatar } from "@multica/ui/components/common/actor-avatar";
import { Button } from "@multica/ui/components/ui/button";
import { useT } from "../i18n";
import { AppLink } from "../navigation";
import { OfficePresence } from "./office-presence";

function initials(name: string): string {
  return name
    .trim()
    .split(/\s+/)
    .map((part) => part[0] ?? "")
    .join("")
    .slice(0, 2)
    .toUpperCase();
}

export function OfficeInspectorPanel({
  inspector,
  snapshot,
  onBack,
}: {
  readonly inspector: Exclude<OfficeInspector, { readonly kind: "closed" }>;
  readonly snapshot: OfficeSnapshot;
  readonly onBack: () => void;
}) {
  const { t } = useT("office");
  return (
    <div
      className="flex min-h-0 flex-1 flex-col bg-surface"
      data-testid="office-inspector"
      onKeyDown={(event) => {
        if (event.key !== "Escape") return;
        event.preventDefault();
        event.stopPropagation();
        onBack();
      }}
    >
      <div className="flex min-h-11 shrink-0 items-center border-b border-surface-border px-2">
        <Button
          type="button"
          variant="ghost"
          className="min-h-11 justify-start px-2"
          onClick={onBack}
        >
          <ArrowLeft aria-hidden="true" />
          {t(($) => $.inspector.back)}
        </Button>
      </div>
      <div className="min-h-0 flex-1 overflow-y-auto p-4">
        {inspector.kind === "agent" ? (
          <AgentInspector agent={inspector.agent} snapshot={snapshot} />
        ) : null}
        {inspector.kind === "squad" ? (
          <SquadInspector
            squad={inspector.squad}
            members={inspector.members}
            snapshot={snapshot}
          />
        ) : null}
        {inspector.kind === "issue" ? (
          <IssueInspector issue={inspector.issue} snapshot={snapshot} />
        ) : null}
        {inspector.kind === "missing" ? (
          <div>
            <CircleHelp
              className="size-6 text-muted-foreground"
              aria-hidden="true"
            />
            <h2 className="mt-3 text-title-sm font-semibold text-foreground">
              {t(($) => $.inspector.missing_title)}
            </h2>
            <p className="mt-1 text-body text-muted-foreground">
              {t(($) => $.inspector.missing_body)}
            </p>
            <p className="sr-only" role="status" aria-live="polite">
              {t(($) => $.a11y.selection_disappeared)}
            </p>
          </div>
        ) : null}
      </div>
    </div>
  );
}

function AgentInspector({
  agent,
  snapshot,
}: {
  readonly agent: OfficeAgent;
  readonly snapshot: OfficeSnapshot;
}) {
  const { t } = useT("office");
  const paths = useWorkspacePaths();
  return (
    <div className="min-w-0">
      <div className="flex min-w-0 items-start gap-3">
        <ActorAvatar
          name={agent.name}
          initials={initials(agent.name)}
          avatarUrl={agent.avatarUrl}
          isAgent
          size="xl"
        />
        <div className="min-w-0 flex-1">
          <div className="text-caption text-muted-foreground">
            {t(($) => $.inspector.agent_title)}
          </div>
          <h2 className="break-words text-title-sm font-semibold text-foreground">
            {agent.name}
          </h2>
        </div>
      </div>

      {agent.description ? (
        <InspectorSection title={t(($) => $.inspector.description)}>
          <p className="whitespace-pre-wrap break-words text-body text-muted-foreground">
            {agent.description}
          </p>
        </InspectorSection>
      ) : null}

      <OfficePresence
        availability={agent.availability}
        workload={agent.workload}
      />

      <InspectorSection title={t(($) => $.inspector.active_issues)}>
        {agent.activeIssueIds.length > 0 ? (
          <ul className="space-y-1">
            {agent.activeIssueIds.map((issueId) => {
              const issue = snapshot.activeIssues.find(
                (candidate) => candidate.id === issueId,
              );
              return (
                <li key={issueId}>
                  <InspectorLink href={paths.issueDetail(issueId)}>
                    <span className="min-w-0 break-all font-mono">
                      {issue?.kind === "resolved"
                        ? issue.identifier
                        : issueId}
                    </span>
                  </InspectorLink>
                </li>
              );
            })}
          </ul>
        ) : (
          <p className="text-body text-muted-foreground">
            {t(($) => $.inspector.none)}
          </p>
        )}
      </InspectorSection>

      <InspectorLink href={paths.agentDetail(agent.id)} command>
        {t(($) => $.inspector.open_agent)}
      </InspectorLink>
    </div>
  );
}

function SquadInspector({
  squad,
  members,
  snapshot,
}: {
  readonly squad: OfficeSquad;
  readonly members: OfficeSquadMembers;
  readonly snapshot: OfficeSnapshot;
}) {
  const { t } = useT("office");
  const paths = useWorkspacePaths();
  const leader = snapshot.agents.find(
    (agent) => agent.id === squad.leaderAgentId,
  );
  return (
    <div className="min-w-0">
      <div className="flex min-w-0 items-start gap-3">
        <ActorAvatar
          name={squad.name}
          initials={initials(squad.name)}
          avatarUrl={squad.avatarUrl}
          isSquad
          size="xl"
        />
        <div className="min-w-0 flex-1">
          <div className="text-caption text-muted-foreground">
            {t(($) => $.inspector.squad_title)}
          </div>
          <h2 className="break-words text-title-sm font-semibold text-foreground">
            {squad.name}
          </h2>
        </div>
      </div>

      {squad.description ? (
        <InspectorSection title={t(($) => $.inspector.description)}>
          <p className="whitespace-pre-wrap break-words text-body text-muted-foreground">
            {squad.description}
          </p>
        </InspectorSection>
      ) : null}

      <dl className="mt-4 grid grid-cols-1 gap-3 border-y border-surface-border py-3 text-body">
        <div>
          <dt className="text-caption text-muted-foreground">
            {t(($) => $.inspector.leader)}
          </dt>
          <dd className="mt-0.5 text-foreground">
            {leader ? (
              <AppLink
                href={paths.agentDetail(leader.id)}
                className="break-words font-medium text-brand outline-none hover:underline focus-visible:ring-2 focus-visible:ring-ring"
              >
                {leader.name}
              </AppLink>
            ) : (
              t(($) => $.roster.leader_unknown)
            )}
          </dd>
        </div>
        <div>
          <dt className="text-caption text-muted-foreground">
            {t(($) => $.inspector.member_count)}
          </dt>
          <dd className="mt-0.5 font-mono tabular-nums text-foreground">
            {squad.memberCount}
          </dd>
        </div>
      </dl>

      <InspectorSection title={t(($) => $.inspector.member_preview)}>
        <ul className="space-y-1">
          {squad.memberPreview.map((member) => (
            <li
              key={`${member.kind}:${member.id}`}
              className="flex min-w-0 items-center gap-2 text-body"
            >
              {member.kind === "agent" ? (
                <Bot className="size-4 shrink-0 text-muted-foreground" aria-hidden="true" />
              ) : member.kind === "member" ? (
                <UserRound className="size-4 shrink-0 text-muted-foreground" aria-hidden="true" />
              ) : (
                <CircleHelp className="size-4 shrink-0 text-muted-foreground" aria-hidden="true" />
              )}
              <span className="min-w-0 flex-1 break-all font-mono text-caption text-foreground">
                {member.id}
              </span>
              <span className="min-w-0 break-words text-caption text-muted-foreground">
                {member.role}
              </span>
            </li>
          ))}
        </ul>
      </InspectorSection>

      <InspectorSection title={t(($) => $.inspector.members)}>
        <SquadMembers members={members} snapshot={snapshot} />
      </InspectorSection>

      <InspectorLink href={paths.squadDetail(squad.id)} command>
        {t(($) => $.inspector.open_squad)}
      </InspectorLink>
    </div>
  );
}

function SquadMembers({
  members,
  snapshot,
}: {
  readonly members: OfficeSquadMembers;
  readonly snapshot: OfficeSnapshot;
}) {
  const { t } = useT("office");
  const paths = useWorkspacePaths();
  if (members.kind === "loading") {
    return (
      <div className="flex min-h-11 items-center gap-2 text-body text-muted-foreground">
        <LoaderCircle
          className="size-4 animate-spin motion-reduce:animate-none"
          aria-hidden="true"
        />
        {t(($) => $.inspector.members_loading)}
      </div>
    );
  }
  if (members.kind === "unavailable") {
    return (
      <div>
        <p className="text-body text-muted-foreground">
          {t(($) => $.inspector.members_unavailable)}
        </p>
        <Button
          type="button"
          variant="outline"
          className="mt-2 min-h-11"
          onClick={() => void members.retry()}
        >
          <RefreshCw aria-hidden="true" />
          {t(($) => $.inspector.retry_members)}
        </Button>
      </div>
    );
  }
  return (
    <ul className="divide-y divide-surface-border/70">
      {members.members.map((member) => {
        const agent =
          member.kind === "agent"
            ? snapshot.agents.find((candidate) => candidate.id === member.id)
            : undefined;
        const label =
          member.name ??
          (member.kind === "unknown"
            ? t(($) => $.inspector.member_unknown)
            : member.id);
        const href =
          member.kind === "agent"
            ? paths.agentDetail(member.id)
            : member.kind === "member"
              ? paths.memberDetail(member.id)
              : null;
        return (
          <li key={`${member.kind}:${member.id}`} className="py-2.5">
            <div className="flex min-w-0 items-center gap-2">
              {member.kind === "agent" ? (
                <Bot className="size-4 shrink-0 text-muted-foreground" aria-hidden="true" />
              ) : member.kind === "member" ? (
                <UserRound className="size-4 shrink-0 text-muted-foreground" aria-hidden="true" />
              ) : (
                <CircleHelp className="size-4 shrink-0 text-muted-foreground" aria-hidden="true" />
              )}
              {href ? (
                <AppLink
                  href={href}
                  className="min-w-0 flex-1 break-words font-medium text-brand outline-none hover:underline focus-visible:ring-2 focus-visible:ring-ring"
                >
                  {label}
                </AppLink>
              ) : (
                <span className="min-w-0 flex-1 break-words font-medium text-foreground">
                  {label}
                </span>
              )}
              <span className="text-caption text-muted-foreground">
                {member.kind === "agent"
                  ? t(($) => $.inspector.member_agent)
                  : member.kind === "member"
                    ? t(($) => $.inspector.member_human)
                    : t(($) => $.inspector.member_unknown)}
              </span>
            </div>
            {agent ? (
              <div className="mt-1">
                <OfficePresence
                  availability={agent.availability}
                  workload={agent.workload}
                  compact
                />
              </div>
            ) : null}
            {member.activeIssueIds.length > 0 ? (
              <div className="mt-1 flex flex-wrap gap-1">
                {member.activeIssueIds.map((issueId) => (
                  <AppLink
                    key={issueId}
                    href={paths.issueDetail(issueId)}
                    className="rounded-md px-1 font-mono text-caption text-brand outline-none hover:bg-surface-hover focus-visible:ring-2 focus-visible:ring-ring"
                  >
                    {issueId}
                  </AppLink>
                ))}
              </div>
            ) : null}
          </li>
        );
      })}
    </ul>
  );
}

function IssueInspector({
  issue,
  snapshot,
}: {
  readonly issue: OfficeIssue;
  readonly snapshot: OfficeSnapshot;
}) {
  const { t } = useT("office");
  const paths = useWorkspacePaths();
  const assignedSquad =
    issue.kind === "resolved" && issue.assignedSquadId
      ? snapshot.squads.find((squad) => squad.id === issue.assignedSquadId)
      : null;
  const issueLabel = issue.kind === "resolved" ? issue.identifier : issue.id;
  return (
    <div className="min-w-0">
      <div className="flex items-start gap-3">
        <span className="inline-flex size-10 shrink-0 items-center justify-center rounded-md bg-warning/12 text-warning">
          <Flag className="size-5" aria-hidden="true" />
        </span>
        <div className="min-w-0 flex-1">
          <div className="text-caption text-muted-foreground">
            {t(($) => $.inspector.issue_title)}
          </div>
          {issue.kind === "resolved" ? (
            <>
              <div className="break-all font-mono text-caption font-semibold text-muted-foreground">
                {issue.identifier}
              </div>
              <h2 className="mt-0.5 break-words text-title-sm font-semibold text-foreground">
                {issue.title}
              </h2>
              <div className="mt-1 font-mono text-caption text-muted-foreground">
                {issue.status}
              </div>
            </>
          ) : (
            <>
              <h2 className="break-all text-title-sm font-semibold text-foreground">
                {t(($) => $.inspector.unresolved_title, { id: issue.id })}
              </h2>
              <p className="mt-1 text-body text-muted-foreground">
                {t(($) => $.inspector.unresolved_body)}
              </p>
            </>
          )}
        </div>
      </div>

      {issue.executingAgentIds.length > 0 ? (
        <InspectorSection title={t(($) => $.inspector.executing_agents)}>
          <ul className="space-y-1">
            {issue.executingAgentIds.map((agentId) => {
              const agent = snapshot.agents.find(
                (candidate) => candidate.id === agentId,
              );
              return (
                <li key={agentId}>
                  <InspectorLink href={paths.agentDetail(agentId)}>
                    <Bot className="size-4" aria-hidden="true" />
                    <span className="min-w-0 break-words">
                      {agent?.name ?? agentId}
                    </span>
                  </InspectorLink>
                </li>
              );
            })}
          </ul>
        </InspectorSection>
      ) : null}

      {issue.kind === "resolved" && issue.assignedSquadId ? (
        <InspectorSection title={t(($) => $.inspector.assigned_squad)}>
          <InspectorLink href={paths.squadDetail(issue.assignedSquadId)}>
            <Users className="size-4" aria-hidden="true" />
            <span className="min-w-0 break-words">
              {assignedSquad?.name ?? issue.assignedSquadId}
            </span>
          </InspectorLink>
        </InspectorSection>
      ) : null}

      <InspectorLink href={paths.issueDetail(issue.id)} command>
        {t(($) => $.inspector.open_issue)}
      </InspectorLink>
      <span className="sr-only">{issueLabel}</span>
    </div>
  );
}

function InspectorSection({
  title,
  children,
}: {
  readonly title: string;
  readonly children: React.ReactNode;
}) {
  return (
    <section className="mt-4 border-t border-surface-border pt-3">
      <h3 className="mb-2 text-caption font-semibold text-foreground">
        {title}
      </h3>
      {children}
    </section>
  );
}

function InspectorLink({
  href,
  command = false,
  children,
}: {
  readonly href: string;
  readonly command?: boolean;
  readonly children: React.ReactNode;
}) {
  return (
    <AppLink
      href={href}
      className={
        command
          ? "mt-4 inline-flex min-h-11 items-center gap-1.5 rounded-md border border-surface-border px-3 text-body font-medium text-foreground outline-none hover:bg-surface-hover focus-visible:ring-2 focus-visible:ring-ring"
          : "inline-flex min-h-8 min-w-0 items-center gap-1.5 rounded-md px-1.5 text-body font-medium text-brand outline-none hover:bg-surface-hover focus-visible:ring-2 focus-visible:ring-ring"
      }
    >
      {children}
      <ArrowUpRight className="size-3.5 shrink-0" aria-hidden="true" />
    </AppLink>
  );
}
