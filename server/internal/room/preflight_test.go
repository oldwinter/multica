package room

import (
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
)

func TestRoomCyclePlanPreservesV1AndPlansV2Synthesis(t *testing.T) {
	tests := []struct {
		name          string
		capability    int32
		source        string
		targets       int
		wantMax       int32
		wantSynthesis bool
	}{
		{name: "v1 multi agent schedule", capability: 1, source: "schedule", targets: 3, wantMax: 3},
		{name: "v2 single direct is optional", capability: 2, source: "message", targets: 1, wantMax: 2},
		{name: "v2 multi agent is required", capability: 2, source: "manual", targets: 2, wantMax: 3, wantSynthesis: true},
		{name: "v2 scheduled single agent is required", capability: 2, source: "schedule", targets: 1, wantMax: 2, wantSynthesis: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			gotMax, gotSynthesis := roomCyclePlan(test.capability, test.source, test.targets)
			if gotMax != test.wantMax || gotSynthesis != test.wantSynthesis {
				t.Fatalf("roomCyclePlan() = (%d, %t), want (%d, %t)", gotMax, gotSynthesis, test.wantMax, test.wantSynthesis)
			}
		})
	}
}

func TestRoomTurnCostLimitPartitionsCycleCeiling(t *testing.T) {
	total := pgtype.Int8{Int64: 10, Valid: true}
	want := []int64{4, 3, 3}
	var allocated int64
	for ordinal, expected := range want {
		limit := roomTurnCostLimit(total, 3, int32(ordinal))
		if limit == nil || *limit != expected {
			t.Fatalf("ordinal %d limit = %v, want %d", ordinal, limit, expected)
		}
		allocated += *limit
	}
	if allocated != total.Int64 {
		t.Fatalf("allocated cost = %d, want %d", allocated, total.Int64)
	}
	if got := roomTurnCostLimit(pgtype.Int8{Int64: 2, Valid: true}, 3, 0); got != nil {
		t.Fatalf("underfunded cycle allocated a turn limit: %v", *got)
	}
}

func TestRoomOutcomeCapabilityVersionRequiresExplicitWorkspaceFlag(t *testing.T) {
	for name, test := range map[string]struct {
		settings []byte
		want     int32
	}{
		"missing":   {settings: nil, want: 1},
		"malformed": {settings: []byte(`not json`), want: 1},
		"disabled":  {settings: []byte(`{"room_outcomes_v2":false}`), want: 1},
		"enabled":   {settings: []byte(`{"room_outcomes_v2":true}`), want: 2},
	} {
		t.Run(name, func(t *testing.T) {
			if got := roomOutcomeCapabilityVersion(test.settings); got != test.want {
				t.Fatalf("roomOutcomeCapabilityVersion() = %d, want %d", got, test.want)
			}
		})
	}
}
