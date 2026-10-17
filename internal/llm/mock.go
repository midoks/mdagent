package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// Mock 是一个规则驱动的本地 Provider，用于无 API Key 时演示完整的
// "模型请求工具 → 执行 → 汇总" 的 Agent Loop。
type Mock struct{}

func NewMock() *Mock { return &Mock{} }

func (m *Mock) Name() string { return "mock" }

func (m *Mock) Chat(_ context.Context, messages []Message, toolSpecs []ToolSpec) (*Message, error) {
	if len(messages) == 0 {
		return nil, fmt.Errorf("empty messages")
	}
	last := messages[len(messages)-1]

	// 工具结果回灌后，输出汇总文本，结束本轮对话。
	if last.Role == "tool" {
		return &Message{
			Role:    "assistant",
			Content: fmt.Sprintf("工具 `%s` 执行完成，结果如下：\n%s", last.Name, last.Content),
		}, nil
	}

	text := lastUserText(messages)
	lower := strings.ToLower(text)
	hasTool := func(name string) bool {
		for _, t := range toolSpecs {
			if t.Name == name {
				return true
			}
		}
		return false
	}

	call := func(name, args string) *Message {
		return &Message{
			Role: "assistant",
			ToolCalls: []ToolCall{{
				ID:        fmt.Sprintf("call_%d", time.Now().UnixNano()),
				Name:      name,
				Arguments: args,
			}},
		}
	}

	switch {
	case strings.Contains(text, "时间") || strings.Contains(text, "几点") || strings.Contains(text, "日期") ||
		strings.Contains(lower, "time") || strings.Contains(lower, "date") || strings.Contains(lower, "clock"):
		if hasTool("get_time") {
			return call("get_time", `{}`), nil
		}
	case strings.Contains(text, "计算") || strings.Contains(lower, "calc"):
		if expr := extractExpr(text); expr != "" && hasTool("calculator") {
			args, _ := json.Marshal(map[string]string{"expression": expr})
			return call("calculator", string(args)), nil
		}
	case strings.HasPrefix(lower, "echo"):
		if hasTool("echo") {
			content := strings.TrimSpace(text[len("echo"):])
			args, _ := json.Marshal(map[string]string{"text": content})
			return call("echo", string(args)), nil
		}
	}

	return &Message{
		Role: "assistant",
		Content: fmt.Sprintf("Mock 回复：收到输入「%s」。\n\n可触发工具调用的示例：\n- 现在几点了 / current time\n- 计算 1+2*3\n- echo hello", text),
	}, nil
}

func lastUserText(messages []Message) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "user" {
			return messages[i].Content
		}
	}
	return ""
}

// extractExpr 提取“计算/calc”之后的算式，并做合法字符白名单校验。
func extractExpr(text string) string {
	var expr string
	if idx := strings.Index(text, "计算"); idx >= 0 {
		expr = text[idx+len("计算"):]
	} else if idx := strings.Index(strings.ToLower(text), "calc"); idx >= 0 {
		expr = text[idx+len("calc"):]
	} else {
		return ""
	}
	expr = strings.Trim(expr, " \t\r\n=？?。.")
	if expr == "" {
		return ""
	}
	for _, r := range expr {
		ok := (r >= '0' && r <= '9') || r == '.' ||
			r == '+' || r == '-' || r == '*' || r == '/' || r == '%' ||
			r == '(' || r == ')' || r == ' '
		if !ok {
			return ""
		}
	}
	return expr
}
