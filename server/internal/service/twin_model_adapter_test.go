package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestLLMTwinJSONModel_sendsOnlyStructuredAcceptedEvidence(t *testing.T) {
	client := &recordingTwinStructuredLLM{enabled: true, response: `{"assertions":[]}`}
	model := llmTwinJSONModel{client: client}
	request := TwinModelRequest{
		Instruction:           "Return JSON only.",
		Evidence:              json.RawMessage(`{"schema_version":2,"wiki_pages":[{"citation_key":"wiki_page:page-1@7","markdown":"Review changes."}]}`),
		CitationKeys:          []string{"wiki_page:page-1@7"},
		AllowedAssertionTypes: []TwinAssertionType{TwinAssertionProcedure},
		MaxAssertions:         12,
		MaxTextCodePoints:     800,
	}

	response, err := model.GenerateJSON(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if string(response) != client.response || client.calls != 1 || client.model != "" || client.temperature != 0 || client.maxCompletionTokens != twinModelMaxCompletionTokens {
		t.Fatalf("adapter call = %#v, response = %s", client, response)
	}
	if !strings.Contains(client.systemPrompt, "accepted canonical evidence") || strings.Contains(client.userPrompt, "/home/") {
		t.Fatalf("adapter prompts = system %q user %q", client.systemPrompt, client.userPrompt)
	}
	var sent TwinModelRequest
	if err := json.Unmarshal([]byte(client.userPrompt), &sent); err != nil {
		t.Fatalf("decode adapter user payload: %v", err)
	}
	if string(sent.Evidence) != string(request.Evidence) || len(sent.CitationKeys) != 1 || sent.CitationKeys[0] != request.CitationKeys[0] {
		t.Fatalf("adapter request = %#v", sent)
	}
}

func TestLLMTwinJSONModel_disabledClientIsExplainableAndInert(t *testing.T) {
	client := &recordingTwinStructuredLLM{}
	_, err := (llmTwinJSONModel{client: client}).GenerateJSON(context.Background(), TwinModelRequest{})
	if !errors.Is(err, ErrTwinGenerationUnavailable) || client.calls != 0 {
		t.Fatalf("GenerateJSON() error = %v, calls = %d", err, client.calls)
	}
}

type recordingTwinStructuredLLM struct {
	enabled             bool
	response            string
	err                 error
	calls               int
	model               string
	systemPrompt        string
	userPrompt          string
	temperature         float64
	maxCompletionTokens int64
}

func (c *recordingTwinStructuredLLM) Enabled() bool { return c.enabled }

func (c *recordingTwinStructuredLLM) GenerateJSON(_ context.Context, model, systemPrompt, userPrompt string, temperature float64, maxCompletionTokens int64) (string, error) {
	c.calls++
	c.model = model
	c.systemPrompt = systemPrompt
	c.userPrompt = userPrompt
	c.temperature = temperature
	c.maxCompletionTokens = maxCompletionTokens
	return c.response, c.err
}
