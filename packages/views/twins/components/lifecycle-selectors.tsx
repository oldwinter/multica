"use client";

import type { LMWikiRevision, TwinProposal, TwinVersion } from "@multica/core/twins";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@multica/ui/components/ui/select";
import { useT } from "../../i18n";

function revisionLabel(revision: LMWikiRevision): string {
  return `r${revision.revision_number} / ${revision.trigger_kind}`;
}

export function WikiRevisionSelector({
  revisions,
  value,
  onChange,
  disabled,
}: {
  revisions: readonly LMWikiRevision[];
  value: string;
  onChange: (id: string) => void;
  disabled: boolean;
}) {
  const { t } = useT("twins");
  const items = revisions.map((revision) => ({ value: revision.id, label: revisionLabel(revision) }));
  return (
    <label className="flex min-w-0 flex-col gap-2 text-label font-medium">
      {t(($) => $.selectors.wiki_revision)}
      <Select items={items} value={value} onValueChange={(next) => typeof next === "string" && onChange(next)}>
        <SelectTrigger disabled={disabled} className="w-full sm:w-72" aria-label={t(($) => $.selectors.wiki_revision)}><SelectValue /></SelectTrigger>
        <SelectContent>{items.map((item) => <SelectItem key={item.value} value={item.value}>{item.label}</SelectItem>)}</SelectContent>
      </Select>
    </label>
  );
}

function proposalLabel(proposal: TwinProposal): string {
  return `${proposal.kind} / ${proposal.id.slice(0, 8)}`;
}

function versionLabel(version: TwinVersion): string {
  return `v${version.version_number} / ${version.id.slice(0, 8)}`;
}

export function TwinHistorySelectors({
  proposals,
  versions,
  proposalId,
  versionId,
  onProposalChange,
  onVersionChange,
  disabled,
}: {
  proposals: readonly TwinProposal[];
  versions: readonly TwinVersion[];
  proposalId: string;
  versionId: string;
  onProposalChange: (id: string) => void;
  onVersionChange: (id: string) => void;
  disabled: boolean;
}) {
  const { t } = useT("twins");
  const proposalItems = proposals.map((proposal) => ({ value: proposal.id, label: proposalLabel(proposal) }));
  const versionItems = versions.map((version) => ({ value: version.id, label: versionLabel(version) }));
  return (
    <div className="grid gap-4 sm:grid-cols-2">
      <label className="flex min-w-0 flex-col gap-2 text-label font-medium">
        {t(($) => $.selectors.twin_proposal)}
        <Select items={proposalItems} value={proposalId} onValueChange={(next) => typeof next === "string" && onProposalChange(next)}>
          <SelectTrigger disabled={disabled} className="w-full" aria-label={t(($) => $.selectors.twin_proposal)}><SelectValue /></SelectTrigger>
          <SelectContent>{proposalItems.map((item) => <SelectItem key={item.value} value={item.value}>{item.label}</SelectItem>)}</SelectContent>
        </Select>
      </label>
      <label className="flex min-w-0 flex-col gap-2 text-label font-medium">
        {t(($) => $.selectors.twin_version)}
        <Select items={versionItems} value={versionId} onValueChange={(next) => typeof next === "string" && onVersionChange(next)}>
          <SelectTrigger disabled={disabled} className="w-full" aria-label={t(($) => $.selectors.twin_version)}><SelectValue /></SelectTrigger>
          <SelectContent>{versionItems.map((item) => <SelectItem key={item.value} value={item.value}>{item.label}</SelectItem>)}</SelectContent>
        </Select>
      </label>
    </div>
  );
}
