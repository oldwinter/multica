package service

import (
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/skillbundle"
)

func TestAgentSkillSource(t *testing.T) {
	tests := []struct {
		name  string
		skill db.Skill
		want  string
	}{
		{name: "manual workspace skill", skill: db.Skill{}, want: skillbundle.SourceWorkspace},
		{
			name: "plugin skill",
			skill: db.Skill{PluginInstallationID: pgtype.UUID{
				Bytes: [16]byte{1}, Valid: true,
			}},
			want: skillbundle.SourcePlugin,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := agentSkillSource(tt.skill); got != tt.want {
				t.Fatalf("agentSkillSource() = %q, want %q", got, tt.want)
			}
		})
	}
}
