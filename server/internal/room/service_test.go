package room

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

type recordingNotifier struct {
	mu    sync.Mutex
	tasks []db.AgentTaskQueue
}

type testArtifactTargets struct{}

func (*testArtifactTargets) CreateRoomArtifactTarget(ctx context.Context, _ pgx.Tx, queries *db.Queries, artifact db.RoomArtifact) (pgtype.UUID, error) {
	switch artifact.Kind {
	case "issue":
		number, err := queries.IncrementIssueCounter(ctx, artifact.WorkspaceID)
		if err != nil {
			return pgtype.UUID{}, err
		}
		issue, err := queries.CreateIssue(ctx, db.CreateIssueParams{
			WorkspaceID: artifact.WorkspaceID, Title: artifact.Title,
			Description: pgtype.Text{String: artifact.Body, Valid: true}, Status: "todo", Priority: "none",
			CreatorType: "member", CreatorID: artifact.CreatedByUserID, Position: -1, Number: number,
		})
		if err != nil {
			return pgtype.UUID{}, err
		}
		metadata, err := json.Marshal(util.UUIDToString(artifact.ID))
		if err != nil {
			return pgtype.UUID{}, err
		}
		issue, err = queries.SetIssueMetadataKey(ctx, db.SetIssueMetadataKeyParams{
			Key: "room_artifact_id", Value: metadata, ID: issue.ID, WorkspaceID: artifact.WorkspaceID,
		})
		return issue.ID, err
	case "wiki":
		page, err := queries.CreateWikiPage(ctx, db.CreateWikiPageParams{
			WorkspaceID: artifact.WorkspaceID, Scope: "workspace",
			Path:  path.Join("rooms", util.UUIDToString(artifact.RoomID), util.UUIDToString(artifact.ID)+".md"),
			Title: artifact.Title, Content: artifact.Body, CreatedBy: artifact.CreatedByUserID,
		})
		return page.ID, err
	default:
		return pgtype.UUID{}, fmt.Errorf("unexpected artifact kind %q", artifact.Kind)
	}
}

func (*testArtifactTargets) RoomArtifactTargetCreated(context.Context, db.RoomArtifact) {}

func (n *recordingNotifier) NotifyTaskEnqueued(_ context.Context, task db.AgentTaskQueue) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.tasks = append(n.tasks, task)
}

func (n *recordingNotifier) EnqueueRoomTurn(ctx context.Context, queries *db.Queries, input RoomTaskEnqueueInput) (db.AgentTaskQueue, error) {
	return queries.CreateRoomTask(ctx, db.CreateRoomTaskParams{
		AgentID: input.Agent.ID, RuntimeID: input.Agent.RuntimeID, Priority: 0, Context: input.Context,
		SquadID: input.SquadID, OriginatorUserID: input.OriginatorUserID, AccountableUserID: input.AccountableUserID,
		OriginatorSource:     pgtype.Text{String: input.OriginatorSource, Valid: true},
		TriggerEvidenceKind:  pgtype.Text{String: input.TriggerEvidenceKind, Valid: true},
		TriggerEvidenceRefID: input.TriggerEvidenceID, RoomTurnID: input.RoomTurnID,
		SessionID: input.SessionID, WorkDir: input.WorkDir,
	})
}

func (n *recordingNotifier) count() int {
	n.mu.Lock()
	defer n.mu.Unlock()
	return len(n.tasks)
}

type recordingEvents struct {
	mu     sync.Mutex
	events []events.Event
}

func (r *recordingEvents) reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = nil
}

func (r *recordingEvents) snapshot() []events.Event {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]events.Event(nil), r.events...)
}

func (r *recordingEvents) Publish(event events.Event) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, event)
}

type serviceFixture struct {
	pool        *pgxpool.Pool
	service     *Service
	notifier    *recordingNotifier
	events      *recordingEvents
	workspaceID pgtype.UUID
	userID      pgtype.UUID
	leaderID    pgtype.UUID
	workerID    pgtype.UUID
	squadID     pgtype.UUID
}

func newServiceFixture(t *testing.T) serviceFixture {
	t.Helper()
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL is required")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}

	fixture := serviceFixture{pool: pool, notifier: &recordingNotifier{}}
	if err := pool.QueryRow(ctx, `
		INSERT INTO "user" (name, email)
		VALUES ('Room Test', 'room-test-' || gen_random_uuid()::text || '@example.com')
		RETURNING id
	`).Scan(&fixture.userID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `
		INSERT INTO workspace (name, slug, description, issue_prefix)
		VALUES ('Room Service', 'room-service-' || gen_random_uuid()::text, '', 'RMS')
		RETURNING id
	`).Scan(&fixture.workspaceID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO member (workspace_id, user_id, role)
		VALUES ($1, $2, 'owner')
	`, fixture.workspaceID, fixture.userID); err != nil {
		t.Fatal(err)
	}

	var runtimeID pgtype.UUID
	if err := pool.QueryRow(ctx, `
		INSERT INTO agent_runtime (
			workspace_id, owner_id, name, runtime_mode, provider, status, visibility
		) VALUES ($1, $2, 'Room runtime', 'cloud', 'codex', 'online', 'private')
		RETURNING id
	`, fixture.workspaceID, fixture.userID).Scan(&runtimeID); err != nil {
		t.Fatal(err)
	}
	for index, target := range []*pgtype.UUID{&fixture.leaderID, &fixture.workerID} {
		if err := pool.QueryRow(ctx, `
			INSERT INTO agent (
				workspace_id, name, description, runtime_mode, runtime_config,
				runtime_id, visibility, max_concurrent_tasks, owner_id, instructions
			) VALUES ($1, $2, '', 'cloud', '{}'::jsonb, $3, 'workspace', 1, $4, '')
			RETURNING id
		`, fixture.workspaceID, "Room agent "+string(rune('A'+index)), runtimeID, fixture.userID).Scan(target); err != nil {
			t.Fatal(err)
		}
	}
	if err := pool.QueryRow(ctx, `
		INSERT INTO squad (workspace_id, name, description, leader_id, creator_id)
		VALUES ($1, 'Room squad', '', $2, $3)
		RETURNING id
	`, fixture.workspaceID, fixture.leaderID, fixture.userID).Scan(&fixture.squadID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO squad_member (squad_id, member_type, member_id, role)
		VALUES ($1, 'agent', $2, 'leader'),
		       ($1, 'agent', $3, 'researcher'),
		       ($1, 'member', $4, 'sponsor')
	`, fixture.squadID, fixture.leaderID, fixture.workerID, fixture.userID); err != nil {
		t.Fatal(err)
	}

	fixture.events = &recordingEvents{}
	fixture.service = NewService(db.New(pool), pool, fixture.notifier, &testArtifactTargets{}, fixture.events)
	t.Cleanup(func() {
		pool.Exec(context.Background(), `DELETE FROM agent_task_queue WHERE room_turn_id IN (SELECT id FROM room_turn WHERE workspace_id = $1)`, fixture.workspaceID)
		pool.Exec(context.Background(), `DELETE FROM wiki_page WHERE workspace_id = $1`, fixture.workspaceID)
		pool.Exec(context.Background(), `DELETE FROM issue WHERE workspace_id = $1`, fixture.workspaceID)
		pool.Exec(context.Background(), `DELETE FROM room_artifact WHERE workspace_id = $1`, fixture.workspaceID)
		pool.Exec(context.Background(), `DELETE FROM room_turn WHERE workspace_id = $1`, fixture.workspaceID)
		pool.Exec(context.Background(), `DELETE FROM room_cycle WHERE workspace_id = $1`, fixture.workspaceID)
		pool.Exec(context.Background(), `DELETE FROM room_entry WHERE workspace_id = $1`, fixture.workspaceID)
		pool.Exec(context.Background(), `DELETE FROM room_participant WHERE workspace_id = $1`, fixture.workspaceID)
		pool.Exec(context.Background(), `DELETE FROM room WHERE workspace_id = $1`, fixture.workspaceID)
		pool.Exec(context.Background(), `DELETE FROM squad_member WHERE squad_id = $1`, fixture.squadID)
		pool.Exec(context.Background(), `DELETE FROM squad WHERE id = $1`, fixture.squadID)
		pool.Exec(context.Background(), `DELETE FROM agent WHERE workspace_id = $1`, fixture.workspaceID)
		pool.Exec(context.Background(), `DELETE FROM agent_runtime WHERE workspace_id = $1`, fixture.workspaceID)
		pool.Exec(context.Background(), `DELETE FROM member WHERE workspace_id = $1`, fixture.workspaceID)
		pool.Exec(context.Background(), `DELETE FROM workspace WHERE id = $1`, fixture.workspaceID)
		pool.Exec(context.Background(), `DELETE FROM "user" WHERE id = $1`, fixture.userID)
		pool.Close()
	})
	return fixture
}

func TestCreateExpandsSquadParticipants(t *testing.T) {
	fixture := newServiceFixture(t)
	created, err := fixture.service.Create(context.Background(), CreateInput{
		WorkspaceID:        fixture.workspaceID,
		ActorUserID:        fixture.userID,
		Title:              "Architecture council",
		Instructions:       "Debate trade-offs and preserve decisions.",
		FacilitatorSquadID: fixture.squadID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.Room.FacilitatorAgentID != fixture.leaderID {
		t.Fatalf("facilitator = %v, want squad leader %v", created.Room.FacilitatorAgentID, fixture.leaderID)
	}
	if len(created.Participants) != 3 {
		t.Fatalf("participants = %d, want 3", len(created.Participants))
	}
	for _, participant := range created.Participants {
		if participant.ParticipantType == "agent" && !participant.SourceSquadID.Valid {
			t.Fatalf("squad agent participant missing source squad: %+v", participant)
		}
	}
}

func TestPostMessageMentionQueuesOnlyMentionedAgent(t *testing.T) {
	fixture := newServiceFixture(t)
	created, err := fixture.service.Create(context.Background(), CreateInput{
		WorkspaceID:        fixture.workspaceID,
		ActorUserID:        fixture.userID,
		Title:              "Research room",
		FacilitatorSquadID: fixture.squadID,
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := fixture.service.PostMessage(context.Background(), MessageInput{
		WorkspaceID:    fixture.workspaceID,
		RoomID:         created.Room.ID,
		ActorUserID:    fixture.userID,
		Body:           "Investigate the retry semantics, @Room agent B.",
		MentionAgents:  []pgtype.UUID{fixture.workerID},
		IdempotencyKey: "message:test-mention",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Cycle.Status != "queued" || len(result.Turns) != 1 {
		t.Fatalf("message wake = status %q turns %d", result.Cycle.Status, len(result.Turns))
	}
	if result.Turns[0].AgentID != fixture.workerID {
		t.Fatalf("turn agent = %v, want %v", result.Turns[0].AgentID, fixture.workerID)
	}
	if fixture.notifier.count() != 1 {
		t.Fatalf("notified tasks = %d, want 1", fixture.notifier.count())
	}
}

func TestPrivateAgentInvocationIsDeniedForAnotherWorkspaceMember(t *testing.T) {
	fixture := newServiceFixture(t)
	ctx := context.Background()
	var otherUserID pgtype.UUID
	if err := fixture.pool.QueryRow(ctx, `
		INSERT INTO "user" (name, email)
		VALUES ('Room Intruder', 'room-intruder-' || gen_random_uuid()::text || '@example.com')
		RETURNING id
	`).Scan(&otherUserID); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.pool.Exec(ctx, `
		INSERT INTO member (workspace_id, user_id, role) VALUES ($1, $2, 'admin')
	`, fixture.workspaceID, otherUserID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		fixture.pool.Exec(context.Background(), `DELETE FROM member WHERE workspace_id = $1 AND user_id = $2`, fixture.workspaceID, otherUserID)
		fixture.pool.Exec(context.Background(), `DELETE FROM "user" WHERE id = $1`, otherUserID)
	})

	if _, err := fixture.service.Create(ctx, CreateInput{
		WorkspaceID: fixture.workspaceID, ActorUserID: otherUserID,
		Title: "Unauthorized room", FacilitatorAgentID: fixture.leaderID,
	}); !errors.Is(err, ErrInvocationNotAllowed) {
		t.Fatalf("private agent create error = %v, want %v", err, ErrInvocationNotAllowed)
	}
	var rooms int
	if err := fixture.pool.QueryRow(ctx, `SELECT count(*) FROM room WHERE workspace_id = $1 AND created_by_user_id = $2`, fixture.workspaceID, otherUserID).Scan(&rooms); err != nil {
		t.Fatal(err)
	}
	if rooms != 0 {
		t.Fatalf("unauthorized create persisted %d rooms", rooms)
	}

	created, err := fixture.service.Create(ctx, CreateInput{
		WorkspaceID: fixture.workspaceID, ActorUserID: fixture.userID,
		Title: "Owner room", FacilitatorAgentID: fixture.leaderID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.service.Wake(ctx, WakeInput{
		WorkspaceID: fixture.workspaceID, RoomID: created.Room.ID, ActorUserID: otherUserID,
		Source: "manual", WakeKey: "manual:unauthorized",
	}); !errors.Is(err, ErrInvocationNotAllowed) {
		t.Fatalf("private agent wake error = %v, want %v", err, ErrInvocationNotAllowed)
	}
	var cycles, tasks int
	if err := fixture.pool.QueryRow(ctx, `
		SELECT (SELECT count(*) FROM room_cycle WHERE room_id = $1),
		       (SELECT count(*) FROM agent_task_queue WHERE room_turn_id IN (SELECT id FROM room_turn WHERE room_id = $1))
	`, created.Room.ID).Scan(&cycles, &tasks); err != nil {
		t.Fatal(err)
	}
	if cycles != 0 || tasks != 0 {
		t.Fatalf("unauthorized wake persisted cycles=%d tasks=%d", cycles, tasks)
	}
}

func TestMultiTargetWakeReservesWholeDailyBudget(t *testing.T) {
	fixture := newServiceFixture(t)
	limit := int32(1)
	created, err := fixture.service.Create(context.Background(), CreateInput{
		WorkspaceID: fixture.workspaceID, ActorUserID: fixture.userID,
		Title: "Budget room", FacilitatorSquadID: fixture.squadID, DailyTurnLimit: &limit,
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := fixture.service.Wake(context.Background(), WakeInput{
		WorkspaceID: fixture.workspaceID, RoomID: created.Room.ID, ActorUserID: fixture.userID,
		Source: "manual", WakeKey: "manual:whole-budget",
		TargetAgentIDs: []pgtype.UUID{fixture.leaderID, fixture.workerID, fixture.leaderID},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Cycle.Status != "refused" || result.Cycle.RefusalReason.String != "budget_exhausted" || len(result.Tasks) != 0 {
		t.Fatalf("multi-target budget result = %+v", result)
	}
}

func TestArchivedFacilitatorPersistsMessageWithoutTask(t *testing.T) {
	fixture := newServiceFixture(t)
	created, err := fixture.service.Create(context.Background(), CreateInput{
		WorkspaceID: fixture.workspaceID, ActorUserID: fixture.userID,
		Title: "Archive room", FacilitatorAgentID: fixture.leaderID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.pool.Exec(context.Background(), `UPDATE agent SET archived_at = now(), archived_by = $2 WHERE id = $1`, fixture.leaderID, fixture.userID); err != nil {
		t.Fatal(err)
	}
	result, err := fixture.service.PostMessage(context.Background(), MessageInput{
		WorkspaceID: fixture.workspaceID, RoomID: created.Room.ID, ActorUserID: fixture.userID,
		Body: "Keep this note after archival.", IdempotencyKey: "message:archived",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Cycle.Status != "refused" || result.Cycle.RefusalReason.String != "no_targets" || len(result.Tasks) != 0 {
		t.Fatalf("archived facilitator result = %+v", result)
	}
	var entries, tasks int
	if err := fixture.pool.QueryRow(context.Background(), `
		SELECT (SELECT count(*) FROM room_entry WHERE room_id = $1),
		       (SELECT count(*) FROM agent_task_queue WHERE room_turn_id IN (SELECT id FROM room_turn WHERE room_id = $1))
	`, created.Room.ID).Scan(&entries, &tasks); err != nil {
		t.Fatal(err)
	}
	if entries != 1 || tasks != 0 {
		t.Fatalf("archived facilitator persisted entries=%d tasks=%d", entries, tasks)
	}
}

func TestMessageReplayReturnsOriginalTaskAndRejectsDifferentRequest(t *testing.T) {
	fixture := newServiceFixture(t)
	created, err := fixture.service.Create(context.Background(), CreateInput{
		WorkspaceID: fixture.workspaceID, ActorUserID: fixture.userID,
		Title: "Replay room", FacilitatorAgentID: fixture.leaderID,
	})
	if err != nil {
		t.Fatal(err)
	}
	input := MessageInput{
		WorkspaceID: fixture.workspaceID, RoomID: created.Room.ID, ActorUserID: fixture.userID,
		Body: "One durable message.", IdempotencyKey: "message:replay",
	}
	first, err := fixture.service.PostMessage(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	replay, err := fixture.service.PostMessage(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Tasks) != 1 || len(replay.Tasks) != 1 || replay.Tasks[0].ID != first.Tasks[0].ID {
		t.Fatalf("replayed task IDs first=%v replay=%v", first.Tasks, replay.Tasks)
	}
	if fixture.notifier.count() != 1 {
		t.Fatalf("message replay notified %d tasks, want 1", fixture.notifier.count())
	}
	input.Body = "A different request with the same key."
	if _, err := fixture.service.PostMessage(context.Background(), input); !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("changed replay error = %v, want %v", err, ErrIdempotencyConflict)
	}
}

func TestWakeReplayDoesNotNotifyOrBroadcastRuntimeState(t *testing.T) {
	fixture := newServiceFixture(t)
	created, err := fixture.service.Create(context.Background(), CreateInput{
		WorkspaceID: fixture.workspaceID, ActorUserID: fixture.userID,
		Title: "Private replay", FacilitatorAgentID: fixture.leaderID,
	})
	if err != nil {
		t.Fatal(err)
	}
	input := WakeInput{
		WorkspaceID: fixture.workspaceID, RoomID: created.Room.ID,
		ActorUserID: fixture.userID, Source: "manual", WakeKey: "manual:private-replay",
	}
	first, err := fixture.service.Wake(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.pool.Exec(context.Background(), `
		UPDATE room_turn SET status = 'completed', result = '{"output":"secret"}'::jsonb,
			session_id = 'secret-session', work_dir = '/secret/workdir', completed_at = now()
		WHERE id = $1
	`, first.Turns[0].ID); err != nil {
		t.Fatal(err)
	}
	fixture.events.reset()
	replay, err := fixture.service.Wake(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if len(replay.Tasks) != 1 || replay.Tasks[0].ID != first.Tasks[0].ID || fixture.notifier.count() != 1 {
		t.Fatalf("wake replay tasks=%v notifications=%d", replay.Tasks, fixture.notifier.count())
	}
	if events := fixture.events.snapshot(); len(events) != 0 {
		t.Fatalf("wake replay published events: %+v", events)
	}

	fixture.events.reset()
	fixture.service.afterWake(context.Background(), created.Room, fixture.userID, replay)
	events := fixture.events.snapshot()
	if len(events) != 1 {
		t.Fatalf("public wake events = %d, want 1", len(events))
	}
	encoded, err := json.Marshal(events[0].Payload)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"agent_id", "task_id", "result", "session_id", "work_dir", "secret"} {
		if strings.Contains(string(encoded), forbidden) {
			t.Fatalf("public Room cycle payload leaked %q: %s", forbidden, encoded)
		}
	}
}

func TestWakeReplayRechecksInvocationPermission(t *testing.T) {
	fixture := newServiceFixture(t)
	created, err := fixture.service.Create(context.Background(), CreateInput{
		WorkspaceID: fixture.workspaceID, ActorUserID: fixture.userID,
		Title: "Revoked replay", FacilitatorAgentID: fixture.leaderID,
	})
	if err != nil {
		t.Fatal(err)
	}
	input := WakeInput{
		WorkspaceID: fixture.workspaceID, RoomID: created.Room.ID,
		ActorUserID: fixture.userID, Source: "manual", WakeKey: "manual:revoked-replay",
	}
	if _, err := fixture.service.Wake(context.Background(), input); err != nil {
		t.Fatal(err)
	}
	var otherUserID pgtype.UUID
	if err := fixture.pool.QueryRow(context.Background(), `
		INSERT INTO "user" (name, email)
		VALUES ('Room Replay Member', 'room-replay-' || gen_random_uuid()::text || '@example.com') RETURNING id
	`).Scan(&otherUserID); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.pool.Exec(context.Background(), `
		INSERT INTO member (workspace_id, user_id, role) VALUES ($1, $2, 'member')
	`, fixture.workspaceID, otherUserID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		fixture.pool.Exec(context.Background(), `DELETE FROM member WHERE workspace_id = $1 AND user_id = $2`, fixture.workspaceID, otherUserID)
		fixture.pool.Exec(context.Background(), `DELETE FROM "user" WHERE id = $1`, otherUserID)
	})
	input.ActorUserID = otherUserID
	if _, err := fixture.service.Wake(context.Background(), input); !errors.Is(err, ErrInvocationNotAllowed) {
		t.Fatalf("revoked replay error = %v, want %v", err, ErrInvocationNotAllowed)
	}
}

func TestRefusedWakeReplayRejectsDifferentTargets(t *testing.T) {
	fixture := newServiceFixture(t)
	created, err := fixture.service.Create(context.Background(), CreateInput{
		WorkspaceID: fixture.workspaceID, ActorUserID: fixture.userID,
		Title: "Refused identity", FacilitatorSquadID: fixture.squadID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.service.SetStatus(context.Background(), fixture.workspaceID, created.Room.ID, "paused"); err != nil {
		t.Fatal(err)
	}
	input := WakeInput{
		WorkspaceID: fixture.workspaceID, RoomID: created.Room.ID,
		ActorUserID: fixture.userID, Source: "manual", WakeKey: "manual:refused-identity",
		TargetAgentIDs: []pgtype.UUID{fixture.leaderID},
	}
	first, err := fixture.service.Wake(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if first.Cycle.Status != "refused" || len(first.Turns) != 1 || first.Turns[0].AgentID != fixture.leaderID || first.Turns[0].Status != "refused" {
		t.Fatalf("refused wake did not persist its target: %+v", first)
	}
	replay, err := fixture.service.Wake(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if len(replay.Turns) != 1 || replay.Turns[0].ID != first.Turns[0].ID || fixture.notifier.count() != 0 {
		t.Fatalf("same-target replay = turns %+v notifications %d", replay.Turns, fixture.notifier.count())
	}
	input.TargetAgentIDs = []pgtype.UUID{fixture.workerID}
	if _, err := fixture.service.Wake(context.Background(), input); !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("different-target refused replay error = %v, want %v", err, ErrIdempotencyConflict)
	}
	used, err := db.New(fixture.pool).CountRoomTurnsSince(context.Background(), db.CountRoomTurnsSinceParams{
		WorkspaceID: fixture.workspaceID, RoomID: created.Room.ID,
		SinceAt: pgtype.Timestamptz{Time: time.Now().Add(-time.Hour), Valid: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if used != 0 {
		t.Fatalf("refused targets consumed %d turns of execution budget", used)
	}
}

func TestTranscriptWindowsContainNewestEntriesInChronologicalOrder(t *testing.T) {
	fixture := newServiceFixture(t)
	ctx := context.Background()
	created, err := fixture.service.Create(ctx, CreateInput{
		WorkspaceID: fixture.workspaceID, ActorUserID: fixture.userID,
		Title: "Long transcript", FacilitatorAgentID: fixture.leaderID,
	})
	if err != nil {
		t.Fatal(err)
	}
	queries := db.New(fixture.pool)
	for ordinal := 1; ordinal <= 205; ordinal++ {
		if _, err := queries.AddRoomEntry(ctx, db.AddRoomEntryParams{
			WorkspaceID: fixture.workspaceID, RoomID: created.Room.ID,
			EntryType: "message", AuthorType: "member", AuthorID: fixture.userID,
			Body: fmt.Sprintf("entry-%03d", ordinal), Mentions: []byte(`[]`),
		}); err != nil {
			t.Fatal(err)
		}
	}
	detail, err := fixture.service.Get(ctx, fixture.workspaceID, created.Room.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(detail.Entries) != 200 || detail.Entries[0].Body != "entry-006" || detail.Entries[199].Body != "entry-205" {
		t.Fatalf("detail transcript window len=%d first=%q last=%q", len(detail.Entries), detail.Entries[0].Body, detail.Entries[len(detail.Entries)-1].Body)
	}
	wake, err := fixture.service.Wake(ctx, WakeInput{
		WorkspaceID: fixture.workspaceID, RoomID: created.Room.ID, ActorUserID: fixture.userID,
		Source: "manual", WakeKey: "manual:latest-transcript",
	})
	if err != nil {
		t.Fatal(err)
	}
	contextV1, err := protocol.ParseRoomTaskContextV1(wake.Tasks[0].Context)
	if err != nil {
		t.Fatal(err)
	}
	if len(contextV1.Transcript) != 100 || contextV1.Transcript[0].Body != "entry-106" || contextV1.Transcript[99].Body != "entry-205" {
		t.Fatalf("task transcript window len=%d first=%q last=%q", len(contextV1.Transcript), contextV1.Transcript[0].Body, contextV1.Transcript[len(contextV1.Transcript)-1].Body)
	}
}

func TestCreateWaitsForWorkspaceDeleteLock(t *testing.T) {
	fixture := newServiceFixture(t)
	ctx := context.Background()
	holder, err := fixture.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer holder.Rollback(ctx)
	if _, err := holder.Exec(ctx, `SELECT id FROM workspace WHERE id = $1 FOR UPDATE`, fixture.workspaceID); err != nil {
		t.Fatal(err)
	}

	done := make(chan error, 1)
	go func() {
		_, createErr := fixture.service.Create(context.Background(), CreateInput{
			WorkspaceID: fixture.workspaceID, ActorUserID: fixture.userID,
			Title: "Deletion race", FacilitatorAgentID: fixture.leaderID,
		})
		done <- createErr
	}()
	select {
	case createErr := <-done:
		t.Fatalf("room create returned while workspace delete lock was held: %v", createErr)
	case <-time.After(150 * time.Millisecond):
	}
	if err := holder.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	select {
	case createErr := <-done:
		if createErr != nil {
			t.Fatal(createErr)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("room create stayed blocked after workspace delete lock released")
	}
}

func TestMessageWriteCannotCommitAfterWorkspaceDelete(t *testing.T) {
	fixture := newServiceFixture(t)
	ctx := context.Background()
	created, err := fixture.service.Create(ctx, CreateInput{
		WorkspaceID: fixture.workspaceID, ActorUserID: fixture.userID,
		Title: "Delete race", FacilitatorAgentID: fixture.leaderID,
	})
	if err != nil {
		t.Fatal(err)
	}
	holder, err := fixture.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer holder.Rollback(ctx)
	if _, err := holder.Exec(ctx, `SELECT id FROM workspace WHERE id = $1 FOR UPDATE`, fixture.workspaceID); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		_, messageErr := fixture.service.PostMessage(context.Background(), MessageInput{
			WorkspaceID: fixture.workspaceID, RoomID: created.Room.ID, ActorUserID: fixture.userID,
			Body: "Do not outlive the workspace.", IdempotencyKey: "delete-race",
		})
		done <- messageErr
	}()
	select {
	case messageErr := <-done:
		t.Fatalf("Room message returned while workspace delete lock was held: %v", messageErr)
	case <-time.After(150 * time.Millisecond):
	}
	qtx := db.New(holder)
	if err := qtx.DeleteWorkspaceTasks(ctx, fixture.workspaceID); err != nil {
		t.Fatal(err)
	}
	if err := qtx.DeleteWorkspaceRoomData(ctx, fixture.workspaceID); err != nil {
		t.Fatal(err)
	}
	for _, statement := range []struct {
		query string
		args  []any
	}{
		{`DELETE FROM squad_member WHERE squad_id = $2 OR (member_type = 'member' AND member_id IN (SELECT user_id FROM member WHERE workspace_id = $1))`, []any{fixture.workspaceID, fixture.squadID}},
		{`DELETE FROM squad WHERE workspace_id = $1`, []any{fixture.workspaceID}},
		{`DELETE FROM agent WHERE workspace_id = $1`, []any{fixture.workspaceID}},
		{`DELETE FROM agent_runtime WHERE workspace_id = $1`, []any{fixture.workspaceID}},
		{`DELETE FROM member WHERE workspace_id = $1`, []any{fixture.workspaceID}},
		{`DELETE FROM workspace WHERE id = $1`, []any{fixture.workspaceID}},
	} {
		if _, err := holder.Exec(ctx, statement.query, statement.args...); err != nil {
			t.Fatal(err)
		}
	}
	if err := holder.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	select {
	case messageErr := <-done:
		if !errors.Is(messageErr, ErrNotFound) {
			t.Fatalf("Room message after workspace deletion = %v, want not found", messageErr)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Room message stayed blocked after workspace deletion committed")
	}
	var entries, cycles, turns int
	if err := fixture.pool.QueryRow(ctx, `
		SELECT (SELECT count(*) FROM room_entry WHERE workspace_id = $1),
		       (SELECT count(*) FROM room_cycle WHERE workspace_id = $1),
		       (SELECT count(*) FROM room_turn WHERE workspace_id = $1)
	`, fixture.workspaceID).Scan(&entries, &cycles, &turns); err != nil {
		t.Fatal(err)
	}
	if entries != 0 || cycles != 0 || turns != 0 {
		t.Fatalf("workspace deletion left Room rows entries=%d cycles=%d turns=%d", entries, cycles, turns)
	}
}

func TestPausedAndBudgetExhaustedWakesPersistRefusal(t *testing.T) {
	fixture := newServiceFixture(t)
	limit := int32(1)
	created, err := fixture.service.Create(context.Background(), CreateInput{
		WorkspaceID:        fixture.workspaceID,
		ActorUserID:        fixture.userID,
		Title:              "Guarded room",
		FacilitatorAgentID: fixture.leaderID,
		DailyTurnLimit:     &limit,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.service.SetStatus(context.Background(), fixture.workspaceID, created.Room.ID, "paused"); err != nil {
		t.Fatal(err)
	}
	paused, err := fixture.service.Wake(context.Background(), WakeInput{
		WorkspaceID: fixture.workspaceID,
		RoomID:      created.Room.ID,
		ActorUserID: fixture.userID,
		Source:      "manual",
		WakeKey:     "manual:paused",
	})
	if err != nil {
		t.Fatal(err)
	}
	if paused.Cycle.Status != "refused" || paused.Cycle.RefusalReason.String != "room_paused" {
		t.Fatalf("paused wake = %+v", paused.Cycle)
	}
	if fixture.notifier.count() != 0 {
		t.Fatalf("paused wake notified %d tasks", fixture.notifier.count())
	}

	if _, err := fixture.service.SetStatus(context.Background(), fixture.workspaceID, created.Room.ID, "active"); err != nil {
		t.Fatal(err)
	}
	accepted, err := fixture.service.Wake(context.Background(), WakeInput{
		WorkspaceID: fixture.workspaceID,
		RoomID:      created.Room.ID,
		ActorUserID: fixture.userID,
		Source:      "manual",
		WakeKey:     "manual:accepted",
	})
	if err != nil || accepted.Cycle.Status != "queued" {
		t.Fatalf("accepted wake = %+v, %v", accepted.Cycle, err)
	}
	if _, err := fixture.pool.Exec(context.Background(), `
		UPDATE room_cycle SET status = 'completed', completed_at = now() WHERE id = $1
	`, accepted.Cycle.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.pool.Exec(context.Background(), `
		UPDATE room SET active_cycle_id = NULL WHERE id = $1
	`, created.Room.ID); err != nil {
		t.Fatal(err)
	}
	exhausted, err := fixture.service.Wake(context.Background(), WakeInput{
		WorkspaceID: fixture.workspaceID,
		RoomID:      created.Room.ID,
		ActorUserID: fixture.userID,
		Source:      "manual",
		WakeKey:     "manual:exhausted",
	})
	if err != nil {
		t.Fatal(err)
	}
	if exhausted.Cycle.Status != "refused" || exhausted.Cycle.RefusalReason.String != "budget_exhausted" {
		t.Fatalf("budget wake = %+v", exhausted.Cycle)
	}
	if fixture.notifier.count() != 1 {
		t.Fatalf("budget wake changed notification count to %d", fixture.notifier.count())
	}
}

func TestDispatchDueAdvancesOnceAndPersistsPausedRefusal(t *testing.T) {
	fixture := newServiceFixture(t)
	interval := int32(5)
	now := time.Date(2026, 8, 13, 4, 40, 0, 0, time.UTC)

	active, err := fixture.service.Create(context.Background(), CreateInput{
		WorkspaceID: fixture.workspaceID, ActorUserID: fixture.userID,
		Title: "Scheduled active room", FacilitatorAgentID: fixture.leaderID,
		ScheduleIntervalMinutes: &interval,
	})
	if err != nil {
		t.Fatal(err)
	}
	activePlan := now.Add(-17 * time.Minute)
	if _, err := fixture.pool.Exec(context.Background(), `UPDATE room SET next_wake_at = $2 WHERE id = $1`, active.Room.ID, activePlan); err != nil {
		t.Fatal(err)
	}

	first, err := fixture.service.DispatchDue(context.Background(), now, 100)
	if err != nil {
		t.Fatal(err)
	}
	if first.RoomsAdvanced != 1 || first.CyclesQueued != 1 || first.TasksQueued != 1 || first.CyclesRefused != 0 {
		t.Fatalf("active due result = %+v", first)
	}
	second, err := fixture.service.DispatchDue(context.Background(), now, 100)
	if err != nil {
		t.Fatal(err)
	}
	if second != (DueResult{}) || fixture.notifier.count() != 1 {
		t.Fatalf("replayed due result = %+v, notifications = %d", second, fixture.notifier.count())
	}
	var activeCycles, activeTasks int
	var activeNext time.Time
	if err := fixture.pool.QueryRow(context.Background(), `
		SELECT (SELECT count(*) FROM room_cycle WHERE room_id = $1),
		       (SELECT count(*) FROM agent_task_queue WHERE room_turn_id IN (SELECT id FROM room_turn WHERE room_id = $1)),
		       (SELECT next_wake_at FROM room WHERE id = $1)
	`, active.Room.ID).Scan(&activeCycles, &activeTasks, &activeNext); err != nil {
		t.Fatal(err)
	}
	if activeCycles != 1 || activeTasks != 1 || !activeNext.After(now) {
		t.Fatalf("active schedule state = cycles %d tasks %d next %s", activeCycles, activeTasks, activeNext)
	}

	paused, err := fixture.service.Create(context.Background(), CreateInput{
		WorkspaceID: fixture.workspaceID, ActorUserID: fixture.userID,
		Title: "Scheduled paused room", FacilitatorAgentID: fixture.leaderID,
		ScheduleIntervalMinutes: &interval,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.service.SetStatus(context.Background(), fixture.workspaceID, paused.Room.ID, "paused"); err != nil {
		t.Fatal(err)
	}
	pausedPlan := now.Add(-time.Minute)
	if _, err := fixture.pool.Exec(context.Background(), `UPDATE room SET next_wake_at = $2 WHERE id = $1`, paused.Room.ID, pausedPlan); err != nil {
		t.Fatal(err)
	}
	pausedResult, err := fixture.service.DispatchDue(context.Background(), now, 100)
	if err != nil {
		t.Fatal(err)
	}
	if pausedResult.RoomsAdvanced != 1 || pausedResult.CyclesRefused != 1 || pausedResult.TasksQueued != 0 {
		t.Fatalf("paused due result = %+v", pausedResult)
	}
	var refusal string
	var pausedNext time.Time
	if err := fixture.pool.QueryRow(context.Background(), `
		SELECT cycle.refusal_reason, room.next_wake_at
		FROM room_cycle cycle JOIN room ON room.id = cycle.room_id
		WHERE cycle.room_id = $1
	`, paused.Room.ID).Scan(&refusal, &pausedNext); err != nil {
		t.Fatal(err)
	}
	if refusal != "room_paused" || !pausedNext.After(now) || fixture.notifier.count() != 1 {
		t.Fatalf("paused schedule state = refusal %q next %s notifications %d", refusal, pausedNext, fixture.notifier.count())
	}
}

func TestDispatchDueIsolatesRevokedAgentAccess(t *testing.T) {
	fixture := newServiceFixture(t)
	ctx := context.Background()
	interval := int32(5)
	now := time.Date(2026, 8, 13, 5, 0, 0, 0, time.UTC)

	denied, err := fixture.service.Create(ctx, CreateInput{
		WorkspaceID: fixture.workspaceID, ActorUserID: fixture.userID,
		Title: "Revoked schedule", FacilitatorAgentID: fixture.leaderID,
		ScheduleIntervalMinutes: &interval,
	})
	if err != nil {
		t.Fatal(err)
	}
	healthy, err := fixture.service.Create(ctx, CreateInput{
		WorkspaceID: fixture.workspaceID, ActorUserID: fixture.userID,
		Title: "Healthy schedule", FacilitatorAgentID: fixture.workerID,
		ScheduleIntervalMinutes: &interval,
	})
	if err != nil {
		t.Fatal(err)
	}

	var otherUserID pgtype.UUID
	if err := fixture.pool.QueryRow(ctx, `
		INSERT INTO "user" (name, email)
		VALUES ('Room Agent Owner', 'room-owner-' || gen_random_uuid()::text || '@example.com')
		RETURNING id
	`).Scan(&otherUserID); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.pool.Exec(ctx, `UPDATE agent SET owner_id = $2 WHERE id = $1`, fixture.leaderID, otherUserID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		fixture.pool.Exec(context.Background(), `UPDATE agent SET owner_id = $2 WHERE id = $1`, fixture.leaderID, fixture.userID)
		fixture.pool.Exec(context.Background(), `DELETE FROM "user" WHERE id = $1`, otherUserID)
	})

	deniedPlan := now.Add(-2 * time.Minute)
	healthyPlan := now.Add(-time.Minute)
	if _, err := fixture.pool.Exec(ctx, `
		UPDATE room
		SET next_wake_at = CASE id WHEN $1 THEN $2::timestamptz ELSE $3::timestamptz END
		WHERE id IN ($1, $4)
	`, denied.Room.ID, deniedPlan, healthyPlan, healthy.Room.ID); err != nil {
		t.Fatal(err)
	}

	result, err := fixture.service.DispatchDue(ctx, now, 100)
	if err != nil {
		t.Fatal(err)
	}
	if result.RoomsAdvanced != 2 || result.CyclesRefused != 1 || result.CyclesQueued != 1 || result.TasksQueued != 1 {
		t.Fatalf("isolated due result = %+v", result)
	}

	var refusal string
	var deniedNext, healthyNext time.Time
	if err := fixture.pool.QueryRow(ctx, `
		SELECT cycle.refusal_reason,
		       (SELECT next_wake_at FROM room WHERE id = $1),
		       (SELECT next_wake_at FROM room WHERE id = $2)
		FROM room_cycle cycle
		WHERE cycle.room_id = $1
	`, denied.Room.ID, healthy.Room.ID).Scan(&refusal, &deniedNext, &healthyNext); err != nil {
		t.Fatal(err)
	}
	if refusal != "invocation_not_allowed" || !deniedNext.After(now) || !healthyNext.After(now) {
		t.Fatalf("isolated schedule state = refusal %q denied %s healthy %s", refusal, deniedNext, healthyNext)
	}
	if fixture.notifier.count() != 1 {
		t.Fatalf("isolated schedule notifications = %d, want 1", fixture.notifier.count())
	}
}

func TestSyncTaskCompletionPersistsOneEntryAndAdvancesMemoryOnce(t *testing.T) {
	fixture := newServiceFixture(t)
	created, err := fixture.service.Create(context.Background(), CreateInput{
		WorkspaceID:        fixture.workspaceID,
		ActorUserID:        fixture.userID,
		Title:              "Continuity room",
		FacilitatorAgentID: fixture.leaderID,
	})
	if err != nil {
		t.Fatal(err)
	}
	wake, err := fixture.service.Wake(context.Background(), WakeInput{
		WorkspaceID: fixture.workspaceID,
		RoomID:      created.Room.ID,
		ActorUserID: fixture.userID,
		Source:      "manual",
		WakeKey:     "manual:completion",
	})
	if err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(map[string]string{"output": "Prefer a lease-based reconciliation loop."})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.pool.Exec(context.Background(), `
		UPDATE agent_task_queue
		SET status = 'completed', result = $2::jsonb, session_id = 'room-session-1',
		    work_dir = '/tmp/room-work', started_at = now(), completed_at = now()
		WHERE id = $1
	`, wake.Tasks[0].ID, payload); err != nil {
		t.Fatal(err)
	}

	changed, err := fixture.service.SyncTask(context.Background(), wake.Tasks[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("first terminal synchronization did not change the Room")
	}
	changed, err = fixture.service.SyncTask(context.Background(), wake.Tasks[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if changed {
		t.Fatal("replayed terminal synchronization changed the Room")
	}

	detail, err := fixture.service.Get(context.Background(), fixture.workspaceID, created.Room.ID)
	if err != nil {
		t.Fatal(err)
	}
	if detail.Room.MemoryVersion != 1 || detail.Room.ActiveCycleID.Valid {
		t.Fatalf("room memory version/active cycle = %d/%v, want 1/empty", detail.Room.MemoryVersion, detail.Room.ActiveCycleID)
	}
	if len(detail.Entries) != 1 || detail.Entries[0].EntryType != "result" || detail.Entries[0].AuthorID != fixture.leaderID {
		t.Fatalf("result entries = %+v", detail.Entries)
	}
	if !strings.Contains(string(detail.Room.Memory), "lease-based reconciliation") {
		t.Fatalf("memory did not include completed contribution: %s", detail.Room.Memory)
	}
	if len(detail.Turns) != 1 || detail.Turns[0].Status != "completed" || detail.Turns[0].SessionID.String != "room-session-1" {
		t.Fatalf("completed turn = %+v", detail.Turns)
	}
	if len(detail.Cycles) != 1 || detail.Cycles[0].Status != "completed" {
		t.Fatalf("completed cycle = %+v", detail.Cycles)
	}
}

func TestSyncTaskWaitsForRetryAndReconcilesLatestAttempt(t *testing.T) {
	fixture := newServiceFixture(t)
	created, err := fixture.service.Create(context.Background(), CreateInput{
		WorkspaceID:        fixture.workspaceID,
		ActorUserID:        fixture.userID,
		Title:              "Retry room",
		FacilitatorAgentID: fixture.leaderID,
	})
	if err != nil {
		t.Fatal(err)
	}
	wake, err := fixture.service.Wake(context.Background(), WakeInput{
		WorkspaceID: fixture.workspaceID,
		RoomID:      created.Room.ID,
		ActorUserID: fixture.userID,
		Source:      "manual",
		WakeKey:     "manual:retry",
	})
	if err != nil {
		t.Fatal(err)
	}
	var retryID pgtype.UUID
	if err := fixture.pool.QueryRow(context.Background(), `
		WITH failed AS (
			UPDATE agent_task_queue
			SET status = 'failed', error = 'temporary network failure',
			    failure_reason = 'provider_network', completed_at = now()
			WHERE id = $1
			RETURNING *
		)
		INSERT INTO agent_task_queue (
			agent_id, runtime_id, status, context, attempt, max_attempts,
			parent_task_id, retry_of_task_id, room_turn_id
		)
		SELECT agent_id, runtime_id, 'queued', context, 2, max_attempts,
		       id, id, room_turn_id
		FROM failed
		RETURNING id
	`, wake.Tasks[0].ID).Scan(&retryID); err != nil {
		t.Fatal(err)
	}
	changed, err := fixture.service.SyncTask(context.Background(), wake.Tasks[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if changed {
		t.Fatal("failed parent finalized the Room while a retry was active")
	}
	payload := []byte(`{"output":"The retry completed cleanly."}`)
	if _, err := fixture.pool.Exec(context.Background(), `
		UPDATE agent_task_queue
		SET status = 'completed', result = $2::jsonb, started_at = now(), completed_at = now()
		WHERE id = $1
	`, retryID, payload); err != nil {
		t.Fatal(err)
	}

	count, err := fixture.service.Reconcile(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("reconciled tasks = %d, want 1", count)
	}
	detail, err := fixture.service.Get(context.Background(), fixture.workspaceID, created.Room.ID)
	if err != nil {
		t.Fatal(err)
	}
	if detail.Turns[0].Status != "completed" || len(detail.Entries) != 1 || detail.Room.MemoryVersion != 1 {
		t.Fatalf("reconciled Room = turn %q, entries %d, memory %d", detail.Turns[0].Status, len(detail.Entries), detail.Room.MemoryVersion)
	}
}

func TestPromoteCompletedEntryIsIdempotentForEveryTarget(t *testing.T) {
	fixture := newServiceFixture(t)
	created, err := fixture.service.Create(context.Background(), CreateInput{
		WorkspaceID:        fixture.workspaceID,
		ActorUserID:        fixture.userID,
		Title:              "Promotion room",
		FacilitatorAgentID: fixture.leaderID,
	})
	if err != nil {
		t.Fatal(err)
	}
	wake, err := fixture.service.Wake(context.Background(), WakeInput{
		WorkspaceID: fixture.workspaceID,
		RoomID:      created.Room.ID,
		ActorUserID: fixture.userID,
		Source:      "manual",
		WakeKey:     "manual:promotion",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.pool.Exec(context.Background(), `
		UPDATE agent_task_queue
		SET status = 'completed', result = '{"output":"Adopt lease-based reconciliation."}'::jsonb,
		    started_at = now(), completed_at = now()
		WHERE id = $1
	`, wake.Tasks[0].ID); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.service.SyncTask(context.Background(), wake.Tasks[0].ID); err != nil {
		t.Fatal(err)
	}
	detail, err := fixture.service.Get(context.Background(), fixture.workspaceID, created.Room.ID)
	if err != nil {
		t.Fatal(err)
	}
	entryID := detail.Entries[0].ID

	for _, kind := range []string{"issue", "wiki", "decision"} {
		t.Run(kind, func(t *testing.T) {
			input := PromotionInput{
				WorkspaceID: fixture.workspaceID, RoomID: created.Room.ID,
				ActorUserID: fixture.userID, EntryID: entryID, Kind: kind,
				IdempotencyKey: "promotion:" + kind, Title: "Lease reconciliation",
				Rationale: "Make recovery deterministic.",
			}
			first, err := fixture.service.Promote(context.Background(), input)
			if err != nil {
				t.Fatal(err)
			}
			second, err := fixture.service.Promote(context.Background(), input)
			if err != nil {
				t.Fatal(err)
			}
			if first.Artifact.ID != second.Artifact.ID || !first.Artifact.TargetID.Valid || first.Artifact.TargetID != second.Artifact.TargetID {
				t.Fatalf("promotion replay changed identity: first=%+v second=%+v", first.Artifact, second.Artifact)
			}

			var targetCount int
			switch kind {
			case "issue":
				err = fixture.pool.QueryRow(context.Background(), `SELECT count(*) FROM issue WHERE id = $1 AND workspace_id = $2`, first.Artifact.TargetID, fixture.workspaceID).Scan(&targetCount)
			case "wiki":
				err = fixture.pool.QueryRow(context.Background(), `SELECT count(*) FROM wiki_page WHERE id = $1 AND workspace_id = $2`, first.Artifact.TargetID, fixture.workspaceID).Scan(&targetCount)
			case "decision":
				if first.Artifact.TargetID != first.Artifact.ID {
					t.Fatalf("decision target = %v, want immutable artifact %v", first.Artifact.TargetID, first.Artifact.ID)
				}
				targetCount = 1
			}
			if err != nil || targetCount != 1 {
				t.Fatalf("target count = %d, err = %v", targetCount, err)
			}
		})
	}

	conflict := PromotionInput{
		WorkspaceID: fixture.workspaceID, RoomID: created.Room.ID,
		ActorUserID: fixture.userID, EntryID: entryID, Kind: "decision",
		IdempotencyKey: "promotion:decision", Title: "Changed title",
	}
	if _, err := fixture.service.Promote(context.Background(), conflict); !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("changed replay error = %v, want idempotency conflict", err)
	}
}

func TestConcurrentPromotionCreatesOneArtifactAndTarget(t *testing.T) {
	fixture := newServiceFixture(t)
	created, err := fixture.service.Create(context.Background(), CreateInput{
		WorkspaceID: fixture.workspaceID, ActorUserID: fixture.userID,
		Title: "Concurrent promotion", FacilitatorAgentID: fixture.leaderID,
	})
	if err != nil {
		t.Fatal(err)
	}
	wake, err := fixture.service.Wake(context.Background(), WakeInput{
		WorkspaceID: fixture.workspaceID, RoomID: created.Room.ID,
		ActorUserID: fixture.userID, Source: "manual", WakeKey: "manual:concurrent-promotion",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.pool.Exec(context.Background(), `
		UPDATE agent_task_queue SET status = 'completed',
		result = '{"output":"Use one atomic promotion transaction."}'::jsonb,
		started_at = now(), completed_at = now() WHERE id = $1
	`, wake.Tasks[0].ID); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.service.SyncTask(context.Background(), wake.Tasks[0].ID); err != nil {
		t.Fatal(err)
	}
	detail, err := fixture.service.Get(context.Background(), fixture.workspaceID, created.Room.ID)
	if err != nil {
		t.Fatal(err)
	}
	input := PromotionInput{
		WorkspaceID: fixture.workspaceID, RoomID: created.Room.ID,
		ActorUserID: fixture.userID, EntryID: detail.Entries[0].ID,
		Kind: "issue", IdempotencyKey: "promotion:concurrent", Title: "Atomic promotion",
	}

	results := make(chan PromotionResult, 8)
	errors := make(chan error, 8)
	var wg sync.WaitGroup
	for index := 0; index < 8; index++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result, promoteErr := fixture.service.Promote(context.Background(), input)
			if promoteErr != nil {
				errors <- promoteErr
				return
			}
			results <- result
		}()
	}
	wg.Wait()
	close(results)
	close(errors)
	for promoteErr := range errors {
		t.Errorf("concurrent promotion: %v", promoteErr)
	}
	var artifactID pgtype.UUID
	for result := range results {
		if !artifactID.Valid {
			artifactID = result.Artifact.ID
		} else if result.Artifact.ID != artifactID {
			t.Errorf("promotion returned artifact %v, want %v", result.Artifact.ID, artifactID)
		}
	}
	var artifactCount, issueCount int
	if err := fixture.pool.QueryRow(context.Background(), `SELECT count(*) FROM room_artifact WHERE room_id = $1 AND idempotency_key = 'promotion:concurrent'`, created.Room.ID).Scan(&artifactCount); err != nil {
		t.Fatal(err)
	}
	if err := fixture.pool.QueryRow(context.Background(), `SELECT count(*) FROM issue WHERE metadata @> jsonb_build_object('room_artifact_id', $1::text)`, artifactID).Scan(&issueCount); err != nil {
		t.Fatal(err)
	}
	if artifactCount != 1 || issueCount != 1 {
		t.Fatalf("artifacts/issues = %d/%d, want 1/1", artifactCount, issueCount)
	}
}
