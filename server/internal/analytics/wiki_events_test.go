package analytics

import (
	"reflect"
	"strings"
	"testing"
)

func TestWikiEventsExposeOnlyBoundedProperties(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		event Event
		keys  []string
	}{
		{"search", WikiSearch("workspace", 2), []string{"result", "scope"}},
		{"proposal", WikiProposalReview("accepted", true), []string{"decision", "edited"}},
		{"lm wiki", LMWikiReview("rejected"), []string{"decision"}},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			keys := make([]string, 0, len(test.event.Properties))
			for key := range test.event.Properties {
				keys = append(keys, key)
				lower := strings.ToLower(key)
				for _, forbidden := range []string{"id", "query", "content", "path", "citation", "prompt"} {
					if strings.Contains(lower, forbidden) {
						t.Fatalf("sensitive property key %q in %#v", key, test.event.Properties)
					}
				}
			}
			if !sameStringSet(keys, test.keys) {
				t.Fatalf("property keys = %v, want %v", keys, test.keys)
			}
			if test.event.DistinctID != "server" || test.event.WorkspaceID != "" {
				t.Fatalf("event carries identity: %#v", test.event)
			}
		})
	}
}

func TestWikiSearchReducesCountsToHitOutcome(t *testing.T) {
	t.Parallel()
	if got := WikiSearch("workspace", 0).Properties["result"]; got != "empty" {
		t.Fatalf("empty search result = %v", got)
	}
	if got := WikiSearch("workspace", 99).Properties["result"]; got != "hit" {
		t.Fatalf("non-empty search result = %v", got)
	}
}

func sameStringSet(left, right []string) bool {
	leftSet := make(map[string]struct{}, len(left))
	for _, value := range left {
		leftSet[value] = struct{}{}
	}
	rightSet := make(map[string]struct{}, len(right))
	for _, value := range right {
		rightSet[value] = struct{}{}
	}
	return reflect.DeepEqual(leftSet, rightSet)
}
