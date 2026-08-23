package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

const (
	twinModelGeneratorID         = "multica-llm-v1"
	twinModelMaxCompletionTokens = int64(16_384)
)

var ErrTwinGenerationUnavailable = errors.New("twin proposal model is not configured")

type twinStructuredLLM interface {
	Enabled() bool
	GenerateJSON(ctx context.Context, model, systemPrompt, userPrompt string, temperature float64, maxCompletionTokens int64) (string, error)
}

type llmTwinJSONModel struct{ client twinStructuredLLM }

// NewProductionTwinService wires the server's configured structured-LLM client
// into Twin generation. Both accepted schema-v1 and schema-v2 evidence use the
// model; the deterministic inventory adapter exists only for compatibility tests.
func NewProductionTwinService(queries *db.Queries, txStarter TwinTxStarter, client twinStructuredLLM) *TwinService {
	service := NewTwinService(queries, txStarter)
	service.ProposalGenerator = &ModelTwinProposalGenerator{
		model:       llmTwinJSONModel{client: client},
		generatorID: twinModelGeneratorID,
	}
	return service
}

func (m llmTwinJSONModel) GenerateJSON(ctx context.Context, request TwinModelRequest) ([]byte, error) {
	if m.client == nil || !m.client.Enabled() {
		return nil, ErrTwinGenerationUnavailable
	}
	payload, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("marshal Twin structured model request: %w", err)
	}
	response, err := m.client.GenerateJSON(
		ctx,
		"",
		"Return one JSON object containing only an assertions array. Derive working guidance only from the accepted canonical evidence and citation keys supplied by the user payload. Never infer identity or persona and never emit credentials, local paths, private profile names, or uncited claims.",
		string(payload),
		0,
		twinModelMaxCompletionTokens,
	)
	if err != nil {
		return nil, fmt.Errorf("call Twin structured model: %w", err)
	}
	return []byte(response), nil
}
