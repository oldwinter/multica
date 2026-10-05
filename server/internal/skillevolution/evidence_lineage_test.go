package skillevolution

import "testing"

func TestHeldOutSelectionCountsUniqueSourceCaseLineages(t *testing.T) {
	workspaceID, skillID := testUUID(), testUUID()
	training := lifecycleEvidenceRef(workspaceID, skillID, "training-review")
	sameTaskTwo := lifecycleEvidenceRef(workspaceID, skillID, "same-task-review-two")
	sameTaskThree := lifecycleEvidenceRef(workspaceID, skillID, "same-task-review-three")
	sameTaskTwo.SourceRevisionID = training.SourceRevisionID
	sameTaskThree.SourceRevisionID = training.SourceRevisionID

	heldOut, err := selectHeldOutEvidenceRefs(
		[]EvidenceRef{training, sameTaskTwo, sameTaskThree},
		[]ResolvedEvidence{{Ref: training}},
		MaxEvidenceRefs,
	)
	if err != nil || len(heldOut) != 0 {
		t.Fatalf("three reviews of one task held-out = (%+v, %v), want no independent case", heldOut, err)
	}

	differentTaskOne := lifecycleEvidenceRef(workspaceID, skillID, "different-task-one")
	differentTaskTwo := lifecycleEvidenceRef(workspaceID, skillID, "different-task-two")
	heldOut, err = selectHeldOutEvidenceRefs(
		[]EvidenceRef{training, sameTaskTwo, differentTaskOne, differentTaskTwo},
		[]ResolvedEvidence{{Ref: training}},
		MaxEvidenceRefs,
	)
	if err != nil || len(heldOut) != MinPassingReplaySamples || heldOut[0].SourceRevisionID == heldOut[1].SourceRevisionID {
		t.Fatalf("independent task lineages held-out = (%+v, %v), want %d unique cases", heldOut, err, MinPassingReplaySamples)
	}
}

func TestEvidenceCaseLineageKeysAreSourceOwned(t *testing.T) {
	workspaceID, skillID := testUUID(), testUUID()
	task := lifecycleEvidenceRef(workspaceID, skillID, "task")
	twin := task
	twin.Kind = EvidenceKindTwinRunFeedback
	twin.SourceID = uuidText(testUUID())
	manual := task
	manual.Kind = EvidenceKindManualRerun
	manual.SourceID = uuidText(testUUID())
	for _, ref := range []EvidenceRef{task, twin, manual} {
		key, ok := evidenceCaseLineageKey(ref)
		if !ok || key != "task\x00"+task.SourceRevisionID {
			t.Fatalf("task-family lineage for %s = %q/%v", ref.Kind, key, ok)
		}
	}
	wiki := task
	wiki.Kind = EvidenceKindWikiProposal
	wiki.SourceRevisionID = uuidText(testUUID())
	room := task
	room.Kind = EvidenceKindRoomOutcome
	if wikiKey, ok := evidenceCaseLineageKey(wiki); !ok || wikiKey != "wiki_proposal\x00"+wiki.SourceID {
		t.Fatalf("Wiki lineage = %q/%v", wikiKey, ok)
	}
	if roomKey, ok := evidenceCaseLineageKey(room); !ok || roomKey != "room_recommendation\x00"+room.SourceRevisionID+"\x00"+room.SourceID {
		t.Fatalf("Room lineage = %q/%v", roomKey, ok)
	}
}
