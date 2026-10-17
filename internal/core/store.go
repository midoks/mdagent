package core

import (
	"sort"
	"sync"
	"time"
)

const defaultMaxLogs = 1000

// Store 是线程安全的内存存储，承担框架默认的轻量持久化。
// 生产环境可替换为数据库实现。
type Store struct {
	mu        sync.RWMutex
	agents    map[string]*Agent
	tasks     map[string]*Task
	taskOrder []string
	logs      []LogEntry
	maxLogs   int
}

func NewStore() *Store {
	return &Store{
		agents:  make(map[string]*Agent),
		tasks:   make(map[string]*Task),
		maxLogs: defaultMaxLogs,
	}
}

// ---- Agent ----

func (s *Store) AddAgent(a *Agent) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.agents[a.ID] = a
}

func (s *Store) GetAgent(id string) (*Agent, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	a, ok := s.agents[id]
	if !ok {
		return nil, false
	}
	cp := *a
	return &cp, true
}

// TouchAgent 刷新 Agent 心跳时间与在线状态。
func (s *Store) TouchAgent(id string, now time.Time) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, ok := s.agents[id]
	if !ok {
		return false
	}
	a.LastSeen = now
	a.Status = AgentOnline
	return true
}

// MarkOffline 将超时未心跳的在线 Agent 标记为离线，返回新离线的 Agent 名称。
func (s *Store) MarkOffline(threshold time.Duration, now time.Time) []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	var names []string
	for _, a := range s.agents {
		if a.Status == AgentOnline && now.Sub(a.LastSeen) > threshold {
			a.Status = AgentOffline
			names = append(names, a.Name)
		}
	}
	return names
}

func (s *Store) ListAgents() []*Agent {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*Agent, 0, len(s.agents))
	for _, a := range s.agents {
		cp := *a
		out = append(out, &cp)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out
}

// ---- Task ----

func (s *Store) AddTask(t *Task) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tasks[t.ID] = t
	s.taskOrder = append(s.taskOrder, t.ID)
}

func (s *Store) GetTask(id string) (*Task, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	t, ok := s.tasks[id]
	if !ok {
		return nil, false
	}
	cp := copyTask(t)
	return &cp, true
}

// ClaimTasks 为 Agent 认领待执行任务（定向任务优先匹配），并置为运行中。
func (s *Store) ClaimTasks(agentID string, now time.Time, limit int) []*Task {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]*Task, 0, limit)
	for _, id := range s.taskOrder {
		t := s.tasks[id]
		if t.Status != TaskPending {
			continue
		}
		if t.AgentID != "" && t.AgentID != agentID {
			continue
		}
		t.AgentID = agentID
		t.Status = TaskRunning
		ts := now
		t.StartedAt = &ts
		cp := copyTask(t)
		out = append(out, &cp)
		if len(out) >= limit {
			break
		}
	}
	return out
}

// FinishTask 写入任务执行结果并累计 Agent 完成数。
func (s *Store) FinishTask(taskID, result, errStr string, steps []string, now time.Time) (*Task, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.tasks[taskID]
	if !ok {
		return nil, false
	}
	if t.Status != TaskRunning {
		return nil, false
	}
	t.Result = result
	t.Steps = steps
	if errStr != "" {
		t.Status = TaskFailed
		t.Error = errStr
	} else {
		t.Status = TaskSucceeded
	}
	t.FinishedAt = &now
	if a, ok := s.agents[t.AgentID]; ok {
		a.TasksDone++
	}
	cp := copyTask(t)
	return &cp, true
}

// ListTasks 按创建时间倒序返回任务副本。
func (s *Store) ListTasks() []*Task {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*Task, 0, len(s.tasks))
	for i := len(s.taskOrder) - 1; i >= 0; i-- {
		t := s.tasks[s.taskOrder[i]]
		cp := copyTask(t)
		out = append(out, &cp)
	}
	return out
}

func copyTask(t *Task) Task {
	cp := *t
	if t.Steps != nil {
		cp.Steps = append([]string(nil), t.Steps...)
	}
	return cp
}

// ---- Log ----

func (s *Store) AddLog(l LogEntry) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.logs = append(s.logs, l)
	if len(s.logs) > s.maxLogs {
		s.logs = append([]LogEntry(nil), s.logs[len(s.logs)-s.maxLogs:]...)
	}
}

// ListLogs 按时间倒序返回日志。
func (s *Store) ListLogs(limit int) []LogEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if limit <= 0 || limit > len(s.logs) {
		limit = len(s.logs)
	}
	out := make([]LogEntry, 0, limit)
	for i := len(s.logs) - 1; i >= 0 && len(out) < limit; i-- {
		out = append(out, s.logs[i])
	}
	return out
}
