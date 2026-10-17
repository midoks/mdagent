package core

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

// Manager 承载中心侧业务逻辑：注册、心跳分发、结果回收、统计。
type Manager struct {
	store *Store
}

func NewManager(s *Store) *Manager {
	return &Manager{store: s}
}

func newID(prefix string) string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return prefix + "-" + hex.EncodeToString(b)
}

// ---- Agent ----

func (m *Manager) RegisterAgent(name string, tools []string, meta map[string]string) *Agent {
	now := time.Now()
	a := &Agent{
		ID:        newID("a"),
		Name:      name,
		Tools:     tools,
		Meta:      meta,
		Status:    AgentOnline,
		LastSeen:  now,
		CreatedAt: now,
	}
	m.store.AddAgent(a)
	m.AddLog("info", "server", fmt.Sprintf("Agent [%s] %s 注册上线，工具数 %d", a.ID, a.Name, len(tools)), "")
	return a
}

// Heartbeat 刷新在线状态并为该 Agent 认领待执行任务。
func (m *Manager) Heartbeat(agentID string) ([]*Task, bool) {
	now := time.Now()
	if !m.store.TouchAgent(agentID, now) {
		return nil, false
	}
	claimed := m.store.ClaimTasks(agentID, now, 5)
	for _, t := range claimed {
		m.AddLog("info", "server", fmt.Sprintf("任务 %s 分配给 Agent %s", t.ID, agentID), t.ID)
	}
	return claimed, true
}

// SweepOffline 标记超时离线的 Agent 并记录日志。
func (m *Manager) SweepOffline(threshold time.Duration) {
	for _, name := range m.store.MarkOffline(threshold, time.Now()) {
		m.AddLog("warn", "server", fmt.Sprintf("Agent [%s] 心跳超时，标记离线", name), "")
	}
}

func (m *Manager) ListAgents() []*Agent {
	return m.store.ListAgents()
}

// ---- Task ----

func (m *Manager) CreateTask(agentID, prompt string) (*Task, error) {
	if agentID != "" {
		if _, ok := m.store.GetAgent(agentID); !ok {
			return nil, fmt.Errorf("agent %s 不存在", agentID)
		}
	}
	t := &Task{
		ID:        newID("t"),
		AgentID:   agentID,
		Prompt:    prompt,
		Status:    TaskPending,
		CreatedAt: time.Now(),
	}
	m.store.AddTask(t)
	target := agentID
	if target == "" {
		target = "任意在线 Agent"
	}
	m.AddLog("info", "server", fmt.Sprintf("创建任务 %s，目标：%s", t.ID, target), t.ID)
	return t, nil
}

// ReportResult 回收 Agent 上报的执行结果。
func (m *Manager) ReportResult(agentID, taskID, result, errStr string, steps []string) bool {
	t, ok := m.store.FinishTask(taskID, result, errStr, steps, time.Now())
	if !ok {
		return false
	}
	if errStr != "" {
		m.AddLog("error", agentID, fmt.Sprintf("任务 %s 执行失败：%s", taskID, errStr), taskID)
	} else {
		m.AddLog("info", agentID, fmt.Sprintf("任务 %s 执行成功，步骤数 %d", taskID, len(steps)), taskID)
	}
	_ = t
	return true
}

func (m *Manager) GetTask(id string) (*Task, bool) {
	return m.store.GetTask(id)
}

func (m *Manager) ListTasks() []*Task {
	return m.store.ListTasks()
}

// ---- Log / Stats ----

func (m *Manager) AddLog(level, source, message, taskID string) {
	m.store.AddLog(LogEntry{
		Time:    time.Now(),
		Level:   level,
		Source:  source,
		Message: message,
		TaskID:  taskID,
	})
}

func (m *Manager) ListLogs(limit int) []LogEntry {
	return m.store.ListLogs(limit)
}

func (m *Manager) Stats() Stats {
	var st Stats
	agents := m.store.ListAgents()
	st.AgentsTotal = len(agents)
	for _, a := range agents {
		if a.Status == AgentOnline {
			st.AgentsOnline++
		}
	}
	tasks := m.store.ListTasks()
	st.TasksTotal = len(tasks)
	now := time.Now()
	finished := 0
	for _, t := range tasks {
		switch t.Status {
		case TaskPending:
			st.TasksPending++
		case TaskRunning:
			st.TasksRunning++
		case TaskSucceeded:
			st.TasksSucceeded++
			finished++
		case TaskFailed:
			st.TasksFailed++
			finished++
		}
		y, mo, d := t.CreatedAt.Date()
		ny, nmo, nd := now.Date()
		if y == ny && mo == nmo && d == nd {
			st.TasksToday++
		}
	}
	if finished > 0 {
		st.SuccessRate = float64(st.TasksSucceeded) / float64(finished) * 100
	}
	return st
}
