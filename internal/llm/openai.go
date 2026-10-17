package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// OpenAI 是 OpenAI 兼容协议（/chat/completions）的 Provider，
// 同样适用于 DeepSeek、通义千问等兼容服务，通过环境变量配置：
//
//	OPENAI_API_KEY  必填
//	OPENAI_BASE_URL 可选，默认 https://api.openai.com/v1
//	MODEL           可选，默认 gpt-4o-mini
type OpenAI struct {
	apiKey  string
	baseURL string
	model   string
	client  *http.Client
}

func NewOpenAIFromEnv() *OpenAI {
	base := os.Getenv("OPENAI_BASE_URL")
	if base == "" {
		base = "https://api.openai.com/v1"
	}
	model := os.Getenv("MODEL")
	if model == "" {
		model = "gpt-4o-mini"
	}
	return &OpenAI{
		apiKey:  os.Getenv("OPENAI_API_KEY"),
		baseURL: base,
		model:   model,
		client:  &http.Client{Timeout: 60 * time.Second},
	}
}

// Available 报告是否具备调用条件。
func (o *OpenAI) Available() bool { return o.apiKey != "" }

func (o *OpenAI) Name() string { return "openai:" + o.model }

// ---- 线上协议结构（与内部 Message 解耦，字段名遵循 OpenAI 规范） ----

type oaiMessage struct {
	Role       string        `json:"role"`
	Content    string        `json:"content,omitempty"`
	ToolCalls  []oaiToolCall `json:"tool_calls,omitempty"`
	ToolCallID string        `json:"tool_call_id,omitempty"`
	Name       string        `json:"name,omitempty"`
}

type oaiToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type oaiTool struct {
	Type     string `json:"type"`
	Function struct {
		Name        string          `json:"name"`
		Description string          `json:"description"`
		Parameters  json.RawMessage `json:"parameters"`
	} `json:"function"`
}

type oaiRequest struct {
	Model    string       `json:"model"`
	Messages []oaiMessage `json:"messages"`
	Tools    []oaiTool    `json:"tools,omitempty"`
}

type oaiResponse struct {
	Choices []struct {
		Message oaiMessage `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func (o *OpenAI) Chat(ctx context.Context, messages []Message, tools []ToolSpec) (*Message, error) {
	reqBody := oaiRequest{Model: o.model}
	for _, m := range messages {
		om := oaiMessage{
			Role:       m.Role,
			Content:    m.Content,
			ToolCallID: m.ToolCallID,
			Name:       m.Name,
		}
		for _, tc := range m.ToolCalls {
			var oc oaiToolCall
			oc.ID = tc.ID
			oc.Type = "function"
			oc.Function.Name = tc.Name
			oc.Function.Arguments = tc.Arguments
			om.ToolCalls = append(om.ToolCalls, oc)
		}
		reqBody.Messages = append(reqBody.Messages, om)
	}
	for _, t := range tools {
		var ot oaiTool
		ot.Type = "function"
		ot.Function.Name = t.Name
		ot.Function.Description = t.Description
		params := t.Parameters
		if params == "" {
			params = `{"type":"object","properties":{}}`
		}
		ot.Function.Parameters = json.RawMessage(params)
		reqBody.Tools = append(reqBody.Tools, ot)
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, o.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+o.apiKey)

	resp, err := o.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	var or oaiResponse
	if err := json.Unmarshal(raw, &or); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}
	if or.Error != nil {
		return nil, fmt.Errorf("LLM 接口错误: %s", or.Error.Message)
	}
	if len(or.Choices) == 0 {
		return nil, fmt.Errorf("LLM 返回为空 (HTTP %d)", resp.StatusCode)
	}

	om := or.Choices[0].Message
	out := &Message{Role: "assistant", Content: om.Content}
	for _, oc := range om.ToolCalls {
		out.ToolCalls = append(out.ToolCalls, ToolCall{
			ID:        oc.ID,
			Name:      oc.Function.Name,
			Arguments: oc.Function.Arguments,
		})
	}
	return out, nil
}
