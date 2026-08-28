package skillevolution

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/skillbundle"
)

func TestWorkspaceSkillForkerCopiesExactBundleAndClearsExternalOwnership(t *testing.T) {
	pool := workspaceSkillForkerTestPool(t)
	workspaceID, sourceID, actorID := publisherUUID(), publisherUUID(), publisherUUID()
	seedPublisherSkill(t, pool, publisherSkillSeed{
		ID: sourceID, WorkspaceID: workspaceID, CreatorID: publisherUUID(), Name: "external",
		Description: "External source", Content: "source body",
		Config: `{"origin":{"type":"github","source_url":"https://example.invalid/repo"},"runtime":{"timeout":30},"keep":true}`,
		Files:  []skillbundle.File{{Path: "references/a.md", Content: "A"}, {Path: "scripts/run.sh", Content: "echo ok"}},
	})
	repository := NewWorkspaceSkillRepository(db.New(pool))
	source := mustLoadPublisherSkill(t, repository, workspaceID, sourceID)
	forker := NewWorkspaceSkillForker(db.New(pool), pool)
	input := ForkSkillInput{
		WorkspaceID: workspaceID, SourceSkillID: sourceID,
		ExpectedSourceHash: Digest(source.Manifest.Hash), NewName: "workspace-copy",
		ActorID: actorID, IdempotencyKey: "fork:external:v1",
	}

	created, err := forker.ForkSkill(context.Background(), input)
	if err != nil {
		t.Fatalf("ForkSkill: %v", err)
	}
	if !created.SkillID.Valid || created.SkillID == sourceID || !created.SourceAuditDigest.Valid() {
		t.Fatalf("fork result = %#v", created)
	}
	forked := mustLoadPublisherSkill(t, repository, workspaceID, created.SkillID)
	if forked.Ownership.Class != OwnershipWorkspace || !forked.Ownership.DirectEvolution || forked.Skill.PluginInstallationID.Valid {
		t.Fatalf("fork ownership = %#v / plugin=%#v", forked.Ownership, forked.Skill.PluginInstallationID)
	}
	expectedBundle := source.Bundle
	expectedBundle.ID = created.SkillID.String()
	expectedBundle.Name = input.NewName
	assertPublisherBundle(t, forked.Bundle, expectedBundle)
	if forked.Skill.CreatedBy != actorID || forked.Skill.Name != input.NewName ||
		forked.Skill.Description != source.Skill.Description || forked.Skill.Content != source.Skill.Content ||
		!sameForkFiles(forked.Files, source.Files) {
		t.Fatalf("forked Skill did not preserve bundle/creator: %#v", forked)
	}
	var config map[string]json.RawMessage
	if err := json.Unmarshal(forked.Skill.Config, &config); err != nil {
		t.Fatal(err)
	}
	var runtimeConfig struct {
		Timeout int `json:"timeout"`
	}
	var keep bool
	if _, present := config["origin"]; present || json.Unmarshal(config["runtime"], &runtimeConfig) != nil ||
		json.Unmarshal(config["keep"], &keep) != nil || runtimeConfig.Timeout != 30 || !keep {
		t.Fatalf("fork config = %s", forked.Skill.Config)
	}
	var audit skillForkAudit
	if err := json.Unmarshal(config[forkAuditConfigKey], &audit); err != nil {
		t.Fatal(err)
	}
	if audit.SourceSkillID != sourceID.String() || audit.SourceOwnership != OwnershipExternal ||
		audit.SourceHash != Digest(source.Manifest.Hash) || audit.Digest != created.SourceAuditDigest ||
		audit.ActorID != actorID.String() || audit.IdempotencyKey != input.IdempotencyKey {
		t.Fatalf("fork audit = %#v", audit)
	}
	if assignments := loadPublisherAssignments(t, pool, created.SkillID); len(assignments) != 0 {
		t.Fatalf("fork assignments = %#v, want none", assignments)
	}

	replayed, err := forker.ForkSkill(context.Background(), input)
	if err != nil || replayed != created {
		t.Fatalf("ForkSkill replay = (%#v, %v), want %#v", replayed, err, created)
	}
}

func TestWorkspaceSkillForkerRejectsStaleUnknownAndConflictingSources(t *testing.T) {
	pool := workspaceSkillForkerTestPool(t)
	workspaceID, actorID := publisherUUID(), publisherUUID()
	for _, tc := range []struct {
		name      string
		config    string
		pluginID  pgtype.UUID
		wantClass OwnershipClass
	}{
		{name: "workspace", config: `{}`, wantClass: OwnershipWorkspace},
		{name: "unknown", config: `{"origin":{"type":"future"}}`, wantClass: OwnershipUnknown},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sourceID := publisherUUID()
			seedPublisherSkill(t, pool, publisherSkillSeed{ID: sourceID, WorkspaceID: workspaceID, CreatorID: actorID, Name: "source-" + tc.name, Config: tc.config, PluginInstallationID: tc.pluginID})
			source := mustLoadPublisherSkill(t, NewWorkspaceSkillRepository(db.New(pool)), workspaceID, sourceID)
			_, err := NewWorkspaceSkillForker(db.New(pool), pool).ForkSkill(context.Background(), ForkSkillInput{
				WorkspaceID: workspaceID, SourceSkillID: sourceID, ExpectedSourceHash: Digest(source.Manifest.Hash),
				NewName: "copy-" + tc.name, ActorID: actorID, IdempotencyKey: "fork:" + tc.name,
			})
			var ownershipErr *ForkRequiredError
			if !errors.As(err, &ownershipErr) || ownershipErr.Ownership.Class != tc.wantClass {
				t.Fatalf("ForkSkill error = %v, want ownership %q", err, tc.wantClass)
			}
		})
	}

	sourceID := publisherUUID()
	seedPublisherSkill(t, pool, publisherSkillSeed{ID: sourceID, WorkspaceID: workspaceID, CreatorID: actorID, Name: "stale-source", Config: `{"origin":{"type":"runtime_local"}}`})
	source := mustLoadPublisherSkill(t, NewWorkspaceSkillRepository(db.New(pool)), workspaceID, sourceID)
	input := ForkSkillInput{WorkspaceID: workspaceID, SourceSkillID: sourceID, ExpectedSourceHash: Digest(source.Manifest.Hash), NewName: "stale-copy", ActorID: actorID, IdempotencyKey: "fork:stale"}
	mustExecPublisher(t, pool, `UPDATE skill SET content = 'changed' WHERE id = $1`, sourceID)
	if _, err := NewWorkspaceSkillForker(db.New(pool), pool).ForkSkill(context.Background(), input); !errors.Is(err, ErrStaleBase) {
		t.Fatalf("stale ForkSkill error = %v", err)
	}

	conflictID := publisherUUID()
	seedPublisherSkill(t, pool, publisherSkillSeed{ID: conflictID, WorkspaceID: workspaceID, CreatorID: actorID, Name: "taken", Config: `{}`})
	current := mustLoadPublisherSkill(t, NewWorkspaceSkillRepository(db.New(pool)), workspaceID, sourceID)
	input.ExpectedSourceHash = Digest(current.Manifest.Hash)
	input.NewName = "taken"
	if _, err := NewWorkspaceSkillForker(db.New(pool), pool).ForkSkill(context.Background(), input); !errors.Is(err, ErrSkillForkIdempotency) {
		t.Fatalf("conflicting ForkSkill error = %v", err)
	}
}

func workspaceSkillForkerTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	pool := workspaceSkillPublisherTestPool(t)
	mustExecPublisher(t, pool, `ALTER TABLE skill ALTER COLUMN id SET DEFAULT gen_random_uuid()`)
	return pool
}

func sameForkFiles(left, right []db.SkillFile) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index].Path != right[index].Path || left[index].Content != right[index].Content {
			return false
		}
	}
	return true
}
