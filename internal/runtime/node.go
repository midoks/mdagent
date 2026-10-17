// Package runtime 提供 Agent 节点运行时：注册、心跳拉取任务、
// 执行 "LLM ↔ 工具" 循环并上报结果。
package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"mdagent/internal/core"
	"mdagent/internal/llm"
	"mdagent/internal/tools"
)

// NodeConfig 是 Agent 节点的配置。
type NodeConfig struct {
	Name      string
	ServerURL string
	Provider  llm.Provider
	Registry  *tools.Registry
	Heartbeat time.Duration // 心跳间隔，建议 5s
	MaxSteps  int           // 单任务最大 LLM↔工具 循环轮数
}

// Node 是一个 Agent 工作节点。
type Node struct {
	cfg NodeConfig
	id  string
	hc  *http.Client
}

func NewNode(cfg NodeConfig) *Node {
	if cfg.Heartbeat <= 0 {
		cfg.Heartbeat = 5 * time.Second
	}
	if cfg.MaxSteps <= 0 {
		cfg.MaxSteps = 8
	}
	return &Node{cfg: cfg, hc: &http.Client{Timeout: 30 * time.Second}}
}

// Run 注册到中心端并进入心跳循环，直到 ctx 取消。
func (n *Node) Run(ctx context.Context) error {
	if err := n.register(ctx); err != nil {
		return err
	}
	log.Printf("[agent] 已注册: id=%s name=%s provider=%s tools=%v",
		n.id, n.cfg.Name, n.cfg.Provider.Name(), n.cfg.Registry.Names())

	ticker := time.NewTicker(n.cfg.Heartbeat)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			log.Printf("[agent] 退出")
			return nil
		case <-ticker.C:
			tasks, err := n.heartbeat()
			if err != nil {
				log.Printf("[agent] 心跳失败: %v", err)
				continue
			}
			for _, t := range tasks {
				t := t
				go n.executeTask(t)
			}
		}
	}
}

// ---- 与中心端的协议 ----

type registerReq struct {
	Name  string            `json:"name"`
	Tools []string          `json:"tools"`
	Meta  map[string]string `json:"meta,omitempty"`
}

type heartbeatReq struct {
	ID string `json:"id"`
}

type heartbeatResp struct {
	Tasks []*core.Task `json:"tasks"`
}

type reportReq struct {
	TaskID  string   `json:"taskId"`
	AgentID string   `json:"agentId"`
	Result  string   `json:"result,omitempty"`
	Error   string   `json:"error,omitempty"`
	Steps   []string `json:"steps,omitempty"`
}

func (n *Node) register(ctx context.Context) error {
	body := registerReq{
		Name:  n.cfg.Name,
		Tools: n.cfg.Registry.Names(),
		Meta: map[string]string{
			"provider": n.cfg.Provider.Name(),
		},
	}
	var resp core.Agent
	// 中心端可能尚未就绪，做有限重试。
	var lastErr error
	for i := 0; i < 10; i++ {
		lastErr = n.post(ctx, "/api/agents/register", body, &resp)
		if lastErr == nil {
			n.id = resp.ID
			return nil
		}
		log.Printf("[agent] 注册失败(%d/10): %v，1s 后重试", i+1, lastErr)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second):
		}
	}
	return fmt.Errorf("注册中心端失败: %w", lastErr)
}

func (n *Node) heartbeat() ([]*core.Task, error) {
	var resp heartbeatResp
	if err := n.post(context.Background(), "/api/agents/heartbeat", heartbeatReq{ID: n.id}, &resp); err != nil {
		return nil, err
	}
	return resp.Tasks, nil
}

func (n *Node) report(req reportReq) {
	var out map[string]any
	if err := n.post(context.Background(), "/api/tasks/report", req, &out); err != nil {
		log.Printf("[agent] 上报任务 %s 结果失败: %v", req.TaskID, err)
	}
}

func (n *Node) post(ctx context.Context, path string, in, out any) error {
	raw, err := json.Marshal(in)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, n.cfg.ServerURL+path, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := n.hc.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		var e struct {
			Error string `json:"error"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&e)
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, e.Error)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// ---- 任务执行：Agent Loop ----

func (n *Node) executeTask(t *core.Task) {
	log.Printf("[agent] 开始任务 %s: %s", t.ID, truncate(t.Prompt, 60))
	steps := []string{}
	specs := n.cfg.Registry.Specs()

	messages := []llm.Message{
		{Role: "system", Content: "你是一个可调用工具的 AI Agent。需要时使用工具获取信息，然后给出简洁的最终回答。"},
		{Role: "user", Content: t.Prompt},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	for i := 0; i < n.cfg.MaxSteps; i++ {
		resp, err := n.cfg.Provider.Chat(ctx, messages, specs)
		if err != nil {
			n.report(reportReq{TaskID: t.ID, AgentID: n.id, Error: err.Error(), Steps: steps})
			return
		}
		if len(resp.ToolCalls) == 0 {
			steps = append(steps, fmt.Sprintf("第 %d 轮：生成最终回答", i+1))
			n.report(reportReq{TaskID: t.ID, AgentID: n.id, Result: resp.Content, Steps: steps})
			log.Printf("[agent] 任务 %s 完成", t.ID)
			return
		}

		messages = append(messages, *resp)
		for _, tc := range resp.ToolCalls {
			step := fmt.Sprintf("第 %d 轮：调用工具 %s(%s)", i+1, tc.Name, tc.Arguments)
			steps = append(steps, step)
			log.Printf("[agent] 任务 %s %s", t.ID, step)

			tool, ok := n.cfg.Registry.Get(tc.Name)
			var out string
			if !ok {
				out = fmt.Sprintf("错误：工具 %s 不存在", tc.Name)
			} else {
				result, err := tool.Execute(ctx, json.RawMessage(tc.Arguments))
				if err != nil {
					out = "错误：" + err.Error()
				} else {
					out = result
				}
			}
			messages = append(messages, llm.Message{
				Role:       "tool",
				Name:       tc.Name,
				ToolCallID: tc.ID,
				Content:    out,
			})
		}
	}
	n.report(reportReq{TaskID: t.ID, AgentID: n.id, Error: "超过最大执行轮数", Steps: steps})
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
