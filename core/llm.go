package core

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

type chatCompletionRequest struct {
	Model            string         `json:"model"`
	Messages         []message      `json:"messages"`
	Temperature      *float64       `json:"temperature,omitempty"`
	TopP             *float64       `json:"top_p,omitempty"`
	N                *int           `json:"n,omitempty"`
	Stop             interface{}    `json:"stop,omitempty"`
	MaxTokens        *int           `json:"max_tokens,omitempty"`
	PresencePenalty  *float64       `json:"presence_penalty,omitempty"`
	FrequencyPenalty *float64       `json:"frequency_penalty,omitempty"`
	LogitBias        map[string]int `json:"logit_bias,omitempty"`
	User             string         `json:"user,omitempty"`
	Stream           bool           `json:"stream,omitempty"`
	Tools            []tool         `json:"tools,omitempty"`
	ToolChoice       interface{}    `json:"tool_choice,omitempty"`
	Seed             *int           `json:"seed,omitempty"`
}

type message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content"`
	ToolCalls  []toolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
	Name       string     `json:"name,omitempty"`
	Refusal    string     `json:"refusal,omitempty"`
}

type toolCall struct {
	ID       string   `json:"id"`
	Type     string   `json:"type"`
	Function function `json:"function"`
}

type function struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type tool struct {
	Type     string       `json:"type"`
	Function toolFunction `json:"function"`
}

type toolFunction struct {
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Parameters  jsonSchema `json:"parameters"`
	Strict      *bool      `json:"strict,omitempty"`
}

type jsonSchema struct {
	Type        string                `json:"type"`
	Description string                `json:"description"`
	Required    []string              `json:"required,omitempty"`
	Properties  map[string]jsonSchema `json:"properties,omitempty"` // For object types
	Items       *jsonSchema           `json:"items,omitempty"`      // For array types
}

type chatResponse struct {
	ID                string   `json:"id"`
	Object            string   `json:"object"`                       // usually "chat.completion"
	Created           int64    `json:"created"`                      // timestamp
	Model             string   `json:"model"`                        // model used
	SystemFingerprint string   `json:"system_fingerprint,omitempty"` // optional
	Choices           []choice `json:"choices"`
	Usage             *usage   `json:"usage,omitempty"`
}

type choice struct {
	Index        int      `json:"index"`
	Message      message  `json:"message"`
	FinishReason string   `json:"finish_reason"`
	Delta        *message `json:"delta,omitempty"` // used in streaming mode
}

type usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type errorResponse struct {
	Error errorData `json:"error"`
}

type errorData struct {
	Message string `json:"message"`
}

var roleSystem = "system"
var roleUser = "user"
var roleAssistant = "assistant"
var roleTool = "tool"

func chat(cfg *LLMConfig, request chatCompletionRequest) (*chatResponse, error) {
	request.Model = cfg.Model
	jsonData, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	url := strings.TrimRight(cfg.BaseURL, "/") + "/chat/completions"
	req, err := http.NewRequest("POST", url, bytes.NewReader(jsonData))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)

	client := &http.Client{}
	response, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		var errResp errorResponse
		json.NewDecoder(response.Body).Decode(&errResp)
		return nil, fmt.Errorf("LLM error %d: %s", response.StatusCode, errResp.Error.Message)
	}

	var chatResp chatResponse
	if err := json.NewDecoder(response.Body).Decode(&chatResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &chatResp, nil
}
