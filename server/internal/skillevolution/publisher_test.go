package skillevolution

import (
	"context"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/skillbundle"
)

func TestWorkspaceSkillRepositoryOwnershipMatrix(t *testing.T) {
	pool := workspaceSkillPublisherTestPool(t)
	repo := NewWorkspaceSkillRepository(db.New(pool))
	ctx := context.Background()
	workspaceID, creatorID := publisherUUID(), publisherUUID()

	cases := []struct {
		name      string
		config    string
		pluginID  pgtype.UUID
		wantClass OwnershipClass
		wantFork  bool
	}{
		{name: "workspace empty config", config: `{}`, wantClass: OwnershipWorkspace},
		{name: "workspace unrelated config", config: `{"runtime":{"timeout":30}}`, wantClass: OwnershipWorkspace},
		{name: "plugin", config: `{}`, pluginID: publisherUUID(), wantClass: OwnershipPlugin, wantFork: true},
		{name: "github", config: `{"origin":{"type":"github"}}`, wantClass: OwnershipExternal, wantFork: true},
		{name: "skills sh", config: `{"origin":{"type":"skills_sh"}}`, wantClass: OwnershipExternal, wantFork: true},
		{name: "clawhub", config: `{"origin":{"type":"clawhub"}}`, wantClass: OwnershipExternal, wantFork: true},
		{name: "runtime local", config: `{"origin":{"type":"runtime_local"}}`, wantClass: OwnershipRuntimeLocal, wantFork: true},
		{name: "unknown origin", config: `{"origin":{"type":"future"}}`, wantClass: OwnershipUnknown, wantFork: true},
		{name: "non object config", config: `[]`, wantClass: OwnershipUnknown, wantFork: true},
		{name: "non object origin", config: `{"origin":true}`, wantClass: OwnershipUnknown, wantFork: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			skillID := publisherUUID()
			seedPublisherSkill(t, pool, publisherSkillSeed{
				ID: skillID, WorkspaceID: workspaceID, CreatorID: creatorID,
				Name: "skill-" + uuid.UUID(skillID.Bytes).String(), Config: tc.config,
				PluginInstallationID: tc.pluginID,
			})
			snapshot, err := repo.Load(ctx, workspaceID, skillID)
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if snapshot.Ownership.Class != tc.wantClass || snapshot.Ownership.ForkRequired != tc.wantFork ||
				snapshot.Ownership.DirectEvolution == tc.wantFork {
				t.Fatalf("ownership = %#v, want class=%q fork=%v", snapshot.Ownership, tc.wantClass, tc.wantFork)
			}
			if tc.wantFork {
				publisher := NewWorkspaceSkillPublisher(db.New(pool), pool)
				_, err := publisher.Publish(ctx, PublishSkillRequest{
					WorkspaceID: workspaceID, SkillID: skillID,
					ExpectedBaseHash: Digest(snapshot.Manifest.Hash), Bundle: snapshot.Bundle,
				})
				var forkErr *ForkRequiredError
				if !errors.Is(err, ErrForkRequired) || !errors.As(err, &forkErr) || forkErr.Ownership.Class != tc.wantClass {
					t.Fatalf("Publish error = %v, want fork required for %q", err, tc.wantClass)
				}
				after := mustLoadPublisherSkill(t, repo, workspaceID, skillID)
				if !reflect.DeepEqual(after.Skill, snapshot.Skill) || after.Manifest.Hash != snapshot.Manifest.Hash {
					t.Fatalf("blocked publication mutated Skill: before=%#v after=%#v", snapshot, after)
				}
			}
		})
	}

	crossWorkspaceSkillID := publisherUUID()
	seedPublisherSkill(t, pool, publisherSkillSeed{
		ID: crossWorkspaceSkillID, WorkspaceID: workspaceID, CreatorID: creatorID, Name: "cross-workspace", Config: `{}`,
	})
	if _, err := repo.Load(ctx, publisherUUID(), crossWorkspaceSkillID); !errors.Is(err, ErrWorkspaceSkillNotFound) {
		t.Fatalf("cross-workspace Load error = %v, want not found", err)
	}

	t.Run("external invalid live bundle still requires fork", func(t *testing.T) {
		skillID := publisherUUID()
		seedPublisherSkill(t, pool, publisherSkillSeed{
			ID: skillID, WorkspaceID: workspaceID, CreatorID: creatorID, Name: "invalid-external", Config: `{"origin":{"type":"github"}}`,
			Files: []skillbundle.File{{Path: "../legacy-invalid", Content: "content"}},
		})
		_, err := NewWorkspaceSkillPublisher(db.New(pool), pool).Publish(ctx, PublishSkillRequest{
			WorkspaceID: workspaceID, SkillID: skillID, ExpectedBaseHash: Digest("sha256:" + strings.Repeat("0", 64)),
			Bundle: publisherBundle(skillID, "invalid-external", "", "content", nil),
		})
		if !errors.Is(err, ErrForkRequired) {
			t.Fatalf("Publish error = %v, want fork required before live bundle validation", err)
		}
	})
}

func TestWorkspaceSkillPublisherReplacesExactBundleAndPreservesMetadata(t *testing.T) {
	pool := workspaceSkillPublisherTestPool(t)
	ctx := context.Background()
	workspaceID, creatorID, skillID := publisherUUID(), publisherUUID(), publisherUUID()
	seedPublisherSkill(t, pool, publisherSkillSeed{
		ID: skillID, WorkspaceID: workspaceID, CreatorID: creatorID,
		Name: "deploy", Description: "old description", Content: "old content",
		Config: `{"runtime":{"timeout":30},"safe":true}`,
		Files:  []skillbundle.File{{Path: "old.md", Content: "old"}, {Path: "refs/keep.md", Content: "replace me"}},
	})
	agentA, agentB := publisherUUID(), publisherUUID()
	mustExecPublisher(t, pool, `INSERT INTO agent_skill (agent_id, skill_id, enabled) VALUES ($1, $3, TRUE), ($2, $3, FALSE)`, agentA, agentB, skillID)

	repo := NewWorkspaceSkillRepository(db.New(pool))
	before := mustLoadPublisherSkill(t, repo, workspaceID, skillID)
	wantAssignments := loadPublisherAssignments(t, pool, skillID)
	candidate := publisherBundle(skillID, "deploy-v2", "new description", "new content",
		[]skillbundle.File{{Path: "z-last.md", Content: "z"}, {Path: "refs/new.md", Content: "new"}})
	publisher := NewWorkspaceSkillPublisher(db.New(pool), pool)
	result, err := publisher.Publish(ctx, PublishSkillRequest{
		WorkspaceID: workspaceID, SkillID: skillID,
		ExpectedBaseHash: Digest(before.Manifest.Hash), Bundle: candidate,
	})
	if err != nil {
		t.Fatalf("Publish: %v", err)
	}
	if result.Idempotent || result.PreHash != Digest(before.Manifest.Hash) || result.PostHash == result.PreHash {
		t.Fatalf("unexpected result: %#v", result)
	}
	assertPublisherBundle(t, result.Snapshot.Bundle, candidate)
	wantSkill := before.Skill
	wantSkill.Name = candidate.Name
	wantSkill.Description = candidate.Description
	wantSkill.Content = candidate.Content
	if !reflect.DeepEqual(result.Snapshot.Skill, wantSkill) {
		t.Fatalf("persisted Skill = %#v, want only reviewed bundle fields changed from %#v", result.Snapshot.Skill, before.Skill)
	}
	if got := loadPublisherAssignments(t, pool, skillID); !reflect.DeepEqual(got, wantAssignments) {
		t.Fatalf("agent_skill assignments = %#v, want %#v", got, wantAssignments)
	}

	idempotent, err := publisher.Publish(ctx, PublishSkillRequest{
		WorkspaceID: workspaceID, SkillID: skillID,
		ExpectedBaseHash: Digest(before.Manifest.Hash), Bundle: candidate,
	})
	if err != nil || !idempotent.Idempotent || idempotent.PostHash != result.PostHash {
		t.Fatalf("duplicate Publish = (%#v, %v), want idempotent success", idempotent, err)
	}
}

func TestWorkspaceSkillPublisherStaleBaseCausesZeroMutation(t *testing.T) {
	pool := workspaceSkillPublisherTestPool(t)
	ctx := context.Background()
	workspaceID, skillID := publisherUUID(), publisherUUID()
	seedPublisherSkill(t, pool, publisherSkillSeed{
		ID: skillID, WorkspaceID: workspaceID, CreatorID: publisherUUID(), Name: "deploy",
		Content: "original", Config: `{}`, Files: []skillbundle.File{{Path: "notes.md", Content: "original"}},
	})
	repo := NewWorkspaceSkillRepository(db.New(pool))
	base := mustLoadPublisherSkill(t, repo, workspaceID, skillID)
	mustExecPublisher(t, pool, `UPDATE skill SET content = 'human edit' WHERE id = $1`, skillID)
	mustExecPublisher(t, pool, `UPDATE skill_file SET content = 'human file edit' WHERE skill_id = $1`, skillID)
	humanEdit := mustLoadPublisherSkill(t, repo, workspaceID, skillID)

	_, err := NewWorkspaceSkillPublisher(db.New(pool), pool).Publish(ctx, PublishSkillRequest{
		WorkspaceID: workspaceID, SkillID: skillID, ExpectedBaseHash: Digest(base.Manifest.Hash),
		Bundle: publisherBundle(skillID, "deploy", "", "candidate", []skillbundle.File{{Path: "candidate.md", Content: "candidate"}}),
	})
	var stale *StaleBaseError
	if !errors.Is(err, ErrStaleBase) || !errors.As(err, &stale) || stale.Current != Digest(humanEdit.Manifest.Hash) {
		t.Fatalf("Publish error = %v, want current stale hash", err)
	}
	after := mustLoadPublisherSkill(t, repo, workspaceID, skillID)
	assertPublisherBundle(t, after.Bundle, humanEdit.Bundle)
}

func TestWorkspaceSkillPublisherFailuresRollback(t *testing.T) {
	t.Run("name conflict", func(t *testing.T) {
		pool := workspaceSkillPublisherTestPool(t)
		workspaceID, skillID := publisherUUID(), publisherUUID()
		seedPublisherSkill(t, pool, publisherSkillSeed{ID: skillID, WorkspaceID: workspaceID, CreatorID: publisherUUID(), Name: "old", Content: "old", Config: `{}`})
		seedPublisherSkill(t, pool, publisherSkillSeed{ID: publisherUUID(), WorkspaceID: workspaceID, CreatorID: publisherUUID(), Name: "taken", Content: "other", Config: `{}`})
		repo := NewWorkspaceSkillRepository(db.New(pool))
		before := mustLoadPublisherSkill(t, repo, workspaceID, skillID)
		_, err := NewWorkspaceSkillPublisher(db.New(pool), pool).Publish(context.Background(), PublishSkillRequest{
			WorkspaceID: workspaceID, SkillID: skillID, ExpectedBaseHash: Digest(before.Manifest.Hash),
			Bundle: publisherBundle(skillID, "taken", "", "candidate", nil),
		})
		if !errors.Is(err, ErrSkillNameConflict) {
			t.Fatalf("Publish error = %v, want name conflict", err)
		}
		assertPublisherBundle(t, mustLoadPublisherSkill(t, repo, workspaceID, skillID).Bundle, before.Bundle)
	})

	t.Run("supporting file write", func(t *testing.T) {
		pool := workspaceSkillPublisherTestPool(t)
		workspaceID, skillID := publisherUUID(), publisherUUID()
		seedPublisherSkill(t, pool, publisherSkillSeed{
			ID: skillID, WorkspaceID: workspaceID, CreatorID: publisherUUID(), Name: "deploy", Content: "old", Config: `{}`,
			Files: []skillbundle.File{{Path: "old.md", Content: "old"}},
		})
		mustExecPublisher(t, pool, `
CREATE FUNCTION fail_publisher_file() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF NEW.path = 'broken.md' THEN RAISE EXCEPTION 'injected write failure'; END IF;
  RETURN NEW;
END $$;
CREATE TRIGGER fail_publisher_file BEFORE INSERT ON skill_file FOR EACH ROW EXECUTE FUNCTION fail_publisher_file()`)
		repo := NewWorkspaceSkillRepository(db.New(pool))
		before := mustLoadPublisherSkill(t, repo, workspaceID, skillID)
		_, err := NewWorkspaceSkillPublisher(db.New(pool), pool).Publish(context.Background(), PublishSkillRequest{
			WorkspaceID: workspaceID, SkillID: skillID, ExpectedBaseHash: Digest(before.Manifest.Hash),
			Bundle: publisherBundle(skillID, "deploy", "", "candidate", []skillbundle.File{{Path: "broken.md", Content: "candidate"}}),
		})
		if err == nil {
			t.Fatal("Publish succeeded, want injected file failure")
		}
		assertPublisherBundle(t, mustLoadPublisherSkill(t, repo, workspaceID, skillID).Bundle, before.Bundle)
	})

	t.Run("post write hash mismatch", func(t *testing.T) {
		pool := workspaceSkillPublisherTestPool(t)
		workspaceID, skillID := publisherUUID(), publisherUUID()
		seedPublisherSkill(t, pool, publisherSkillSeed{
			ID: skillID, WorkspaceID: workspaceID, CreatorID: publisherUUID(), Name: "deploy", Content: "old", Config: `{}`,
			Files: []skillbundle.File{{Path: "old.md", Content: "old"}},
		})
		mustExecPublisher(t, pool, `
CREATE FUNCTION drift_publisher_file() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN NEW.content := NEW.content || '-drift'; RETURN NEW; END $$;
CREATE TRIGGER drift_publisher_file BEFORE INSERT ON skill_file FOR EACH ROW EXECUTE FUNCTION drift_publisher_file()`)
		repo := NewWorkspaceSkillRepository(db.New(pool))
		before := mustLoadPublisherSkill(t, repo, workspaceID, skillID)
		_, err := NewWorkspaceSkillPublisher(db.New(pool), pool).Publish(context.Background(), PublishSkillRequest{
			WorkspaceID: workspaceID, SkillID: skillID, ExpectedBaseHash: Digest(before.Manifest.Hash),
			Bundle: publisherBundle(skillID, "deploy", "", "candidate", []skillbundle.File{{Path: "new.md", Content: "candidate"}}),
		})
		if !errors.Is(err, ErrPostWriteHashMismatch) {
			t.Fatalf("Publish error = %v, want post-write mismatch", err)
		}
		assertPublisherBundle(t, mustLoadPublisherSkill(t, repo, workspaceID, skillID).Bundle, before.Bundle)
	})
}

func TestWorkspaceSkillPublisherCommitAmbiguityIsNotRetried(t *testing.T) {
	pool := workspaceSkillPublisherTestPool(t)
	workspaceID, skillID := publisherUUID(), publisherUUID()
	seedPublisherSkill(t, pool, publisherSkillSeed{ID: skillID, WorkspaceID: workspaceID, CreatorID: publisherUUID(), Name: "deploy", Content: "old", Config: `{}`})
	repo := NewWorkspaceSkillRepository(db.New(pool))
	base := mustLoadPublisherSkill(t, repo, workspaceID, skillID)
	candidate := publisherBundle(skillID, "deploy", "", "candidate", []skillbundle.File{{Path: "new.md", Content: "new"}})
	starter := &publisherCommitStarter{pool: pool, commitErr: errors.New("injected connection loss after commit")}

	_, err := NewWorkspaceSkillPublisher(db.New(pool), starter).Publish(context.Background(), PublishSkillRequest{
		WorkspaceID: workspaceID, SkillID: skillID, ExpectedBaseHash: Digest(base.Manifest.Hash), Bundle: candidate,
	})
	if !errors.Is(err, ErrPublicationUnknown) || starter.beginCalls != 1 {
		t.Fatalf("Publish = %v after %d transactions, want one unknown attempt", err, starter.beginCalls)
	}
	assertPublisherBundle(t, mustLoadPublisherSkill(t, repo, workspaceID, skillID).Bundle, candidate)

	result, err := NewWorkspaceSkillPublisher(db.New(pool), pool).Publish(context.Background(), PublishSkillRequest{
		WorkspaceID: workspaceID, SkillID: skillID, ExpectedBaseHash: Digest(base.Manifest.Hash), Bundle: candidate,
	})
	if err != nil || !result.Idempotent {
		t.Fatalf("explicit resolution retry = (%#v, %v), want idempotent", result, err)
	}
}

func TestWorkspaceSkillPublisherTimeoutBeforeCommitRequiresInspection(t *testing.T) {
	pool := workspaceSkillPublisherTestPool(t)
	workspaceID, skillID := publisherUUID(), publisherUUID()
	seedPublisherSkill(t, pool, publisherSkillSeed{ID: skillID, WorkspaceID: workspaceID, CreatorID: publisherUUID(), Name: "deploy", Content: "old", Config: `{}`})
	repo := NewWorkspaceSkillRepository(db.New(pool))
	base := mustLoadPublisherSkill(t, repo, workspaceID, skillID)
	candidate := publisherBundle(skillID, "deploy", "", "candidate", nil)
	starter := &publisherCommitStarter{pool: pool, beforeCommitErr: context.DeadlineExceeded}

	_, err := NewWorkspaceSkillPublisher(db.New(pool), starter).Publish(context.Background(), PublishSkillRequest{
		WorkspaceID: workspaceID, SkillID: skillID, ExpectedBaseHash: Digest(base.Manifest.Hash), Bundle: candidate,
	})
	if !errors.Is(err, ErrPublicationUnknown) || !errors.Is(err, context.DeadlineExceeded) || starter.beginCalls != 1 {
		t.Fatalf("Publish = %v after %d transactions, want one unknown attempt", err, starter.beginCalls)
	}
	assertPublisherBundle(t, mustLoadPublisherSkill(t, repo, workspaceID, skillID).Bundle, base.Bundle)
}

func TestWorkspaceSkillPublisherDetectsDriftBeforeReturning(t *testing.T) {
	pool := workspaceSkillPublisherTestPool(t)
	workspaceID, skillID := publisherUUID(), publisherUUID()
	seedPublisherSkill(t, pool, publisherSkillSeed{ID: skillID, WorkspaceID: workspaceID, CreatorID: publisherUUID(), Name: "deploy", Content: "old", Config: `{}`})
	repo := NewWorkspaceSkillRepository(db.New(pool))
	base := mustLoadPublisherSkill(t, repo, workspaceID, skillID)
	candidate := publisherBundle(skillID, "deploy", "", "candidate", []skillbundle.File{{Path: "new.md", Content: "new"}})
	starter := &publisherCommitStarter{pool: pool, afterCommit: func(ctx context.Context) error {
		_, err := pool.Exec(ctx, `UPDATE skill_file SET content = 'concurrent edit' WHERE skill_id = $1 AND path = 'new.md'`, skillID)
		return err
	}}

	_, err := NewWorkspaceSkillPublisher(db.New(pool), starter).Publish(context.Background(), PublishSkillRequest{
		WorkspaceID: workspaceID, SkillID: skillID, ExpectedBaseHash: Digest(base.Manifest.Hash), Bundle: candidate,
	})
	var unknown *PublicationUnknownError
	if !errors.Is(err, ErrPublicationUnknown) || !errors.Is(err, ErrConcurrentBundleDrift) || !errors.As(err, &unknown) ||
		unknown.ExpectedPostHash == unknown.ObservedPostHash {
		t.Fatalf("Publish error = %#v, want publication unknown concurrent drift", err)
	}
}

func TestWorkspaceSkillPublisherRollbackUsesSameSeam(t *testing.T) {
	pool := workspaceSkillPublisherTestPool(t)
	workspaceID, skillID := publisherUUID(), publisherUUID()
	seedPublisherSkill(t, pool, publisherSkillSeed{
		ID: skillID, WorkspaceID: workspaceID, CreatorID: publisherUUID(), Name: "deploy-a", Description: "a", Content: "a", Config: `{}`,
		Files: []skillbundle.File{{Path: "a.md", Content: "a"}},
	})
	repo := NewWorkspaceSkillRepository(db.New(pool))
	publisher := NewWorkspaceSkillPublisher(db.New(pool), pool)
	a := mustLoadPublisherSkill(t, repo, workspaceID, skillID)
	bBundle := publisherBundle(skillID, "deploy-b", "b", "b", []skillbundle.File{{Path: "b.md", Content: "b"}})
	b := publishAndLoad(t, publisher, repo, workspaceID, skillID, Digest(a.Manifest.Hash), bBundle)
	cBundle := publisherBundle(skillID, "deploy-c", "c", "c", []skillbundle.File{{Path: "c.md", Content: "c"}})
	c := publishAndLoad(t, publisher, repo, workspaceID, skillID, Digest(b.Manifest.Hash), cBundle)

	rolledBackToB := publishAndLoad(t, publisher, repo, workspaceID, skillID, Digest(c.Manifest.Hash), b.Bundle)
	assertPublisherBundle(t, rolledBackToB.Bundle, b.Bundle)
	rolledBackToA := publishAndLoad(t, publisher, repo, workspaceID, skillID, Digest(rolledBackToB.Manifest.Hash), a.Bundle)
	assertPublisherBundle(t, rolledBackToA.Bundle, a.Bundle)
}

type publisherSkillSeed struct {
	ID                   pgtype.UUID
	WorkspaceID          pgtype.UUID
	CreatorID            pgtype.UUID
	Name                 string
	Description          string
	Content              string
	Config               string
	PluginInstallationID pgtype.UUID
	Files                []skillbundle.File
}

func seedPublisherSkill(t *testing.T, pool *pgxpool.Pool, seed publisherSkillSeed) {
	t.Helper()
	if seed.Config == "" {
		seed.Config = `{}`
	}
	mustExecPublisher(t, pool, `
INSERT INTO skill (id, workspace_id, name, description, content, config, created_by, plugin_installation_id)
VALUES ($1, $2, $3, $4, $5, $6::jsonb, $7, $8)`,
		seed.ID, seed.WorkspaceID, seed.Name, seed.Description, seed.Content, seed.Config, seed.CreatorID, seed.PluginInstallationID)
	for _, file := range seed.Files {
		mustExecPublisher(t, pool, `INSERT INTO skill_file (skill_id, path, content) VALUES ($1, $2, $3)`, seed.ID, file.Path, file.Content)
	}
}

func workspaceSkillPublisherTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://multica:multica@localhost:5432/multica?sslmode=disable"
	}
	admin, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Skipf("connect to Postgres: %v", err)
	}
	if err := admin.Ping(ctx); err != nil {
		admin.Close()
		t.Skipf("Postgres unavailable: %v", err)
	}
	schema := "skill_publisher_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+pgx.Identifier{schema}.Sanitize()); err != nil {
		admin.Close()
		t.Fatalf("create test schema: %v", err)
	}
	t.Cleanup(func() {
		_, _ = admin.Exec(context.Background(), "DROP SCHEMA "+pgx.Identifier{schema}.Sanitize()+" CASCADE")
		admin.Close()
	})
	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	config.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatalf("open schema pool: %v", err)
	}
	t.Cleanup(pool.Close)
	mustExecPublisher(t, pool, `
CREATE TABLE skill (
  id UUID PRIMARY KEY,
  workspace_id UUID NOT NULL,
  name TEXT NOT NULL,
  description TEXT NOT NULL DEFAULT '',
  content TEXT NOT NULL DEFAULT '',
  config JSONB NOT NULL DEFAULT '{}',
  created_by UUID NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  plugin_installation_id UUID NULL,
  UNIQUE (workspace_id, name)
);
CREATE TABLE skill_file (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  skill_id UUID NOT NULL,
  path TEXT NOT NULL,
  content TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (skill_id, path)
);
CREATE TABLE agent_skill (
  agent_id UUID NOT NULL,
  skill_id UUID NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  enabled BOOLEAN NOT NULL DEFAULT TRUE,
  PRIMARY KEY (agent_id, skill_id)
)`)
	return pool
}

func mustExecPublisher(t *testing.T, pool *pgxpool.Pool, sql string, args ...any) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), sql, args...); err != nil {
		t.Fatalf("execute publisher fixture SQL: %v", err)
	}
}

func mustLoadPublisherSkill(t *testing.T, repo *WorkspaceSkillRepository, workspaceID, skillID pgtype.UUID) WorkspaceSkillSnapshot {
	t.Helper()
	snapshot, err := repo.Load(context.Background(), workspaceID, skillID)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return snapshot
}

func publisherBundle(skillID pgtype.UUID, name, description, content string, files []skillbundle.File) skillbundle.Skill {
	return skillbundle.Skill{
		ID: uuid.UUID(skillID.Bytes).String(), Source: skillbundle.SourceWorkspace,
		Name: name, Description: description, Content: content, Files: files,
	}
}

func publisherUUID() pgtype.UUID {
	return pgtype.UUID{Bytes: uuid.New(), Valid: true}
}

func assertPublisherBundle(t *testing.T, got, want skillbundle.Skill) {
	t.Helper()
	gotManifest, err := skillbundle.BuildValidatedManifest(got)
	if err != nil {
		t.Fatalf("validate got bundle: %v", err)
	}
	wantManifest, err := skillbundle.BuildValidatedManifest(want)
	if err != nil {
		t.Fatalf("validate want bundle: %v", err)
	}
	if gotManifest.Hash != wantManifest.Hash {
		t.Fatalf("bundle hash = %s, want %s\ngot: %#v\nwant: %#v", gotManifest.Hash, wantManifest.Hash, got, want)
	}
}

type publisherAssignment struct {
	AgentID pgtype.UUID
	Enabled bool
}

func loadPublisherAssignments(t *testing.T, pool *pgxpool.Pool, skillID pgtype.UUID) []publisherAssignment {
	t.Helper()
	rows, err := pool.Query(context.Background(), `SELECT agent_id, enabled FROM agent_skill WHERE skill_id = $1 ORDER BY agent_id`, skillID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var result []publisherAssignment
	for rows.Next() {
		var assignment publisherAssignment
		if err := rows.Scan(&assignment.AgentID, &assignment.Enabled); err != nil {
			t.Fatal(err)
		}
		result = append(result, assignment)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return result
}

func publishAndLoad(
	t *testing.T,
	publisher SkillPublisher,
	repo *WorkspaceSkillRepository,
	workspaceID, skillID pgtype.UUID,
	expected Digest,
	bundle skillbundle.Skill,
) WorkspaceSkillSnapshot {
	t.Helper()
	if _, err := publisher.Publish(context.Background(), PublishSkillRequest{
		WorkspaceID: workspaceID, SkillID: skillID, ExpectedBaseHash: expected, Bundle: bundle,
	}); err != nil {
		t.Fatalf("Publish: %v", err)
	}
	return mustLoadPublisherSkill(t, repo, workspaceID, skillID)
}

type publisherCommitStarter struct {
	pool            *pgxpool.Pool
	afterCommit     func(context.Context) error
	beforeCommitErr error
	commitErr       error
	beginCalls      int
}

func (s *publisherCommitStarter) BeginTx(ctx context.Context, options pgx.TxOptions) (pgx.Tx, error) {
	s.beginCalls++
	tx, err := s.pool.BeginTx(ctx, options)
	if err != nil {
		return nil, err
	}
	return &publisherCommitTx{
		Tx: tx, afterCommit: s.afterCommit, beforeCommitErr: s.beforeCommitErr, commitErr: s.commitErr,
	}, nil
}

type publisherCommitTx struct {
	pgx.Tx
	afterCommit     func(context.Context) error
	beforeCommitErr error
	commitErr       error
}

func (tx *publisherCommitTx) Commit(ctx context.Context) error {
	if tx.beforeCommitErr != nil {
		return tx.beforeCommitErr
	}
	if err := tx.Tx.Commit(ctx); err != nil {
		return err
	}
	if tx.afterCommit != nil {
		if err := tx.afterCommit(ctx); err != nil {
			return fmt.Errorf("after commit: %w", err)
		}
	}
	return tx.commitErr
}
