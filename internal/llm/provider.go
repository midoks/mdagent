package llm

import "context"

// Message 是一条对话消息。Role 取值：system / user / assistant / tool。
type Message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content,omitempty"`
	ToolCalls  []ToolCall `json:"toolCalls,omitempty"`  // assistant 发起的工具调用
	ToolCallID string     `json:"toolCallId,omitempty"` // tool 消息对应的调用 ID
	Name       string     `json:"name,omitempty"`       // tool 消息的工具名
}

// ToolCall 是模型请求的一次工具调用。
type ToolCall struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"` // JSON 字符串
}

// ToolSpec 是向模型描述工具的结构。
type ToolSpec struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Parameters  string `json:"parameters"` // JSON Schema 字符串
}

// Provider 抽象一个 LLM 提供方，便于在 Mock / OpenAI 兼容服务间切换。
type Provider interface {
	// Name 返回提供方标识，用于日志与展示。
	Name() string
	// Chat 发送对话历史与可用工具，返回模型的下一条消息（文本或工具调用）。
	Chat(ctx context.Context, messages []Message, tools []ToolSpec) (*Message, error)
}
