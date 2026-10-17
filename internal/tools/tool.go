package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"mdagent/internal/llm"
)

// Tool 是 Agent 可调用工具的抽象。
type Tool interface {
	Name() string
	Description() string
	// Parameters 返回 JSON Schema 字符串，描述入参结构。
	Parameters() string
	Execute(ctx context.Context, args json.RawMessage) (string, error)
}

// Registry 是线程安全的工具注册表。
type Registry struct {
	mu    sync.RWMutex
	tools map[string]Tool
	order []string
}

func NewRegistry() *Registry {
	return &Registry{tools: make(map[string]Tool)}
}

func (r *Registry) Register(t Tool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.tools[t.Name()]; !exists {
		r.order = append(r.order, t.Name())
	}
	r.tools[t.Name()] = t
}

func (r *Registry) Get(name string) (Tool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.tools[name]
	return t, ok
}

// Names 返回注册顺序稳定的工具名列表。
func (r *Registry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return append([]string(nil), r.order...)
}

// Specs 返回提供给 LLM 的工具描述。
func (r *Registry) Specs() []llm.ToolSpec {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]llm.ToolSpec, 0, len(r.order))
	for _, name := range r.order {
		t := r.tools[name]
		out = append(out, llm.ToolSpec{
			Name:        t.Name(),
			Description: t.Description(),
			Parameters:  t.Parameters(),
		})
	}
	return out
}

// FuncTool 便于用函数快速定义一个工具。
type FuncTool struct {
	ToolName    string
	Desc        string
	Params      string
	Fn          func(ctx context.Context, args json.RawMessage) (string, error)
}

func (f *FuncTool) Name() string        { return f.ToolName }
func (f *FuncTool) Description() string { return f.Desc }
func (f *FuncTool) Parameters() string  { return f.Params }
func (f *FuncTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	if f.Fn == nil {
		return "", fmt.Errorf("tool %s 未实现", f.ToolName)
	}
	return f.Fn(ctx, args)
}
