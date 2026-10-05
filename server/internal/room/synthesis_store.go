package room

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func validateRoomSynthesis(ctx context.Context, queries *db.Queries, workspaceID, roomID pgtype.UUID, raw []byte) (Synthesis, []byte, string, error) {
	citationIDs, err := synthesisCitationIDs(raw)
	if err != nil {
		return Synthesis{}, nil, "", err
	}
	parsed := make([]pgtype.UUID, 0, len(citationIDs))
	seen := make(map[pgtype.UUID]struct{}, len(citationIDs))
	for _, value := range citationIDs {
		id, parseErr := uuid.Parse(strings.TrimSpace(value))
		if parseErr != nil {
			return Synthesis{}, nil, "", fmt.Errorf("synthesis citation UUID: %w", ErrInvalidSynthesis)
		}
		candidate := pgtype.UUID{Bytes: id, Valid: true}
		if _, duplicate := seen[candidate]; duplicate {
			continue
		}
		seen[candidate] = struct{}{}
		parsed = append(parsed, candidate)
	}
	entries, err := queries.ListRoomEntriesByIDs(ctx, db.ListRoomEntriesByIDsParams{
		WorkspaceID: workspaceID, RoomID: roomID, EntryIds: parsed,
	})
	if err != nil {
		return Synthesis{}, nil, "", fmt.Errorf("load Room synthesis citations: %w", err)
	}
	return ValidateSynthesis(raw, roomEntryIDSet(entries))
}
