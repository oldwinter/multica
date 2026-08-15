package protocol

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	RoomTaskContextType         = "room"
	RoomTaskContextSchemaV1     = 1
	DaemonCapabilityRoomTasksV1 = "room-tasks-v1"
)

var ErrInvalidRoomTaskContext = errors.New("invalid Room task context")

// RoomTaskTranscriptEntryV1 is the storage-independent subset of a Room Entry
// needed by a daemon to participate in a Room turn.
type RoomTaskTranscriptEntryV1 struct {
	ID         string   `json:"id"`
	Ordinal    int64    `json:"ordinal"`
	EntryType  string   `json:"entry_type"`
	AuthorType string   `json:"author_type"`
	AuthorID   string   `json:"author_id,omitempty"`
	Body       string   `json:"body"`
	Mentions   []string `json:"mentions,omitempty"`
	CreatedAt  string   `json:"created_at"`
}

// RoomTaskContextV1 is the durable queue payload shared by the Room producer
// and daemon claim consumer. It intentionally has no dependency on Room or
// sqlc storage types.
type RoomTaskContextV1 struct {
	Type          string                      `json:"type"`
	SchemaVersion int                         `json:"schema_version"`
	WorkspaceID   string                      `json:"workspace_id"`
	RoomID        string                      `json:"room_id"`
	CycleID       string                      `json:"cycle_id"`
	TurnID        string                      `json:"turn_id"`
	Title         string                      `json:"title"`
	Instructions  string                      `json:"instructions"`
	Memory        json.RawMessage             `json:"memory"`
	Transcript    []RoomTaskTranscriptEntryV1 `json:"transcript"`
}

// EncodeRoomTaskContextV1 stamps and validates the v1 discriminator before a
// Room producer persists the payload in the task queue.
func EncodeRoomTaskContextV1(context RoomTaskContextV1) ([]byte, error) {
	context.Type = RoomTaskContextType
	context.SchemaVersion = RoomTaskContextSchemaV1
	if err := validateRoomTaskContextV1(context); err != nil {
		return nil, err
	}
	payload, err := json.Marshal(context)
	if err != nil {
		return nil, fmt.Errorf("encode Room task context v1: %w", err)
	}
	return payload, nil
}

// ParseRoomTaskContextV1 parses and validates the versioned durable queue
// payload. Unsupported or legacy shapes fail closed instead of being silently
// interpreted as another task kind.
func ParseRoomTaskContextV1(payload []byte) (RoomTaskContextV1, error) {
	var context RoomTaskContextV1
	if err := json.Unmarshal(payload, &context); err != nil {
		return RoomTaskContextV1{}, fmt.Errorf("%w: decode Room task context v1: %v", ErrInvalidRoomTaskContext, err)
	}
	if err := validateRoomTaskContextV1(context); err != nil {
		return RoomTaskContextV1{}, err
	}
	return context, nil
}

func validateRoomTaskContextV1(context RoomTaskContextV1) error {
	if context.Type != RoomTaskContextType {
		return fmt.Errorf("Room task context type %q: %w", context.Type, ErrInvalidRoomTaskContext)
	}
	if context.SchemaVersion != RoomTaskContextSchemaV1 {
		return fmt.Errorf("Room task context schema version %d: %w", context.SchemaVersion, ErrInvalidRoomTaskContext)
	}
	if strings.TrimSpace(context.Title) == "" {
		return fmt.Errorf("Room task context title is empty: %w", ErrInvalidRoomTaskContext)
	}
	for name, value := range map[string]string{
		"workspace_id": context.WorkspaceID,
		"room_id":      context.RoomID,
		"cycle_id":     context.CycleID,
		"turn_id":      context.TurnID,
	} {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("Room task context %s is empty: %w", name, ErrInvalidRoomTaskContext)
		}
		if _, err := uuid.Parse(value); err != nil {
			return fmt.Errorf("Room task context %s is not a UUID: %w", name, ErrInvalidRoomTaskContext)
		}
	}
	if len(context.Memory) == 0 || !json.Valid(context.Memory) {
		return fmt.Errorf("Room task context memory is not valid JSON: %w", ErrInvalidRoomTaskContext)
	}
	for index, entry := range context.Transcript {
		if strings.TrimSpace(entry.ID) == "" || strings.TrimSpace(entry.EntryType) == "" || strings.TrimSpace(entry.AuthorType) == "" {
			return fmt.Errorf("Room task context transcript entry %d is incomplete: %w", index, ErrInvalidRoomTaskContext)
		}
		if _, err := uuid.Parse(entry.ID); err != nil {
			return fmt.Errorf("Room task context transcript entry %d id is not a UUID: %w", index, ErrInvalidRoomTaskContext)
		}
		if entry.AuthorID != "" {
			if _, err := uuid.Parse(entry.AuthorID); err != nil {
				return fmt.Errorf("Room task context transcript entry %d author id is not a UUID: %w", index, ErrInvalidRoomTaskContext)
			}
		}
		if _, err := time.Parse(time.RFC3339Nano, entry.CreatedAt); err != nil {
			return fmt.Errorf("Room task context transcript entry %d created_at is not RFC3339: %w", index, ErrInvalidRoomTaskContext)
		}
		for mentionIndex, mentionID := range entry.Mentions {
			if _, err := uuid.Parse(mentionID); err != nil {
				return fmt.Errorf("Room task context transcript entry %d mention %d is not a UUID: %w", index, mentionIndex, ErrInvalidRoomTaskContext)
			}
		}
	}
	return nil
}
