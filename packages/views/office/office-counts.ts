import type { OfficeSnapshot } from "@multica/core/office";

interface OfficeSubjectCount {
  readonly shown: number;
  readonly total: number;
}

export interface OfficeSnapshotCounts {
  readonly agents: OfficeSubjectCount;
  readonly squads: OfficeSubjectCount;
  readonly issues: OfficeSubjectCount;
}

function subjectCount(total: number, overflow: number): OfficeSubjectCount {
  return {
    shown: Math.max(0, total - overflow),
    total,
  };
}

export function officeSnapshotCounts(
  snapshot: OfficeSnapshot,
): OfficeSnapshotCounts {
  return {
    agents: subjectCount(snapshot.agents.length, snapshot.overflow.agents),
    squads: subjectCount(snapshot.squads.length, snapshot.overflow.squads),
    issues: subjectCount(
      snapshot.activeIssues.length,
      snapshot.overflow.activeIssues,
    ),
  };
}
