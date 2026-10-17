package core

import "time"

// AgentStatus 表示 Agent 节点的在线状态。
type AgentStatus string

const (
	AgentOnline  AgentStatus = "online"
	AgentOffline AgentStatus = "offline"
)

// Agent 是一个已注册的 AI 工作节点。
type Agent struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	Tools     []string          `json:"tools"`
	Meta      map[string]string `json:"meta,omitempty"`
	Status    AgentStatus       `json:"status"`
	LastSeen  time.Time         `json:"lastSeen"`
	CreatedAt time.Time         `json:"createdAt"`
	TasksDone int               `json:"tasksDone"`
}

// TaskStatus 表示任务生命周期状态。
type TaskStatus string

const (
	TaskPending   TaskStatus = "pending"
	TaskRunning   TaskStatus = "running"
	TaskSucceeded TaskStatus = "succeeded"
	TaskFailed    TaskStatus = "failed"
)

// Task 是下发给 Agent 执行的一次 LLM 会话任务。
type Task struct {
	ID         string     `json:"id"`
	AgentID    string     `json:"agentId"` // 为空表示任意在线 Agent 均可认领
	Prompt     string     `json:"prompt"`
	Status     TaskStatus `json:"status"`
	Result     string     `json:"result,omitempty"`
	Error      string     `json:"error,omitempty"`
	Steps      []string   `json:"steps,omitempty"` // Agent 执行过程步骤
	CreatedAt  time.Time  `json:"createdAt"`
	StartedAt  *time.Time `json:"startedAt,omitempty"`
	FinishedAt *time.Time `json:"finishedAt,omitempty"`
}

// LogEntry 是一条中心侧事件日志。
type LogEntry struct {
	Time    time.Time `json:"time"`
	Level   string    `json:"level"` // info / warn / error
	Source  string    `json:"source"`
	Message string    `json:"message"`
	TaskID  string    `json:"taskId,omitempty"`
}

// Stats 是 Dashboard 概览统计数据。
type Stats struct {
	AgentsTotal    int     `json:"agentsTotal"`
	AgentsOnline   int     `json:"agentsOnline"`
	TasksTotal     int     `json:"tasksTotal"`
	TasksPending   int     `json:"tasksPending"`
	TasksRunning   int     `json:"tasksRunning"`
	TasksSucceeded int     `json:"tasksSucceeded"`
	TasksFailed    int     `json:"tasksFailed"`
	TasksToday     int     `json:"tasksToday"`
	SuccessRate    float64 `json:"successRate"`
}
