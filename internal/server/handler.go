package server

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"mdagent/internal/core"
)

type apiHandler struct {
	m *core.Manager
}

func writeErr(c *gin.Context, code int, msg string) {
	c.JSON(code, gin.H{"error": msg})
}

// ---- Agent 侧协议 ----

type registerReq struct {
	Name  string            `json:"name"`
	Tools []string          `json:"tools"`
	Meta  map[string]string `json:"meta"`
}

func (h *apiHandler) registerAgent(c *gin.Context) {
	var req registerReq
	if err := c.ShouldBindJSON(&req); err != nil {
		writeErr(c, http.StatusBadRequest, "请求格式错误: "+err.Error())
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		writeErr(c, http.StatusBadRequest, "name 不能为空")
		return
	}
	a := h.m.RegisterAgent(req.Name, req.Tools, req.Meta)
	c.JSON(http.StatusOK, a)
}

func (h *apiHandler) heartbeat(c *gin.Context) {
	var req struct {
		ID string `json:"id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.ID == "" {
		writeErr(c, http.StatusBadRequest, "id 不能为空")
		return
	}
	tasks, ok := h.m.Heartbeat(req.ID)
	if !ok {
		writeErr(c, http.StatusNotFound, "agent 未注册")
		return
	}
	c.JSON(http.StatusOK, gin.H{"tasks": tasks})
}

func (h *apiHandler) reportTask(c *gin.Context) {
	var req struct {
		TaskID  string   `json:"taskId"`
		AgentID string   `json:"agentId"`
		Result  string   `json:"result"`
		Error   string   `json:"error"`
		Steps   []string `json:"steps"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.TaskID == "" {
		writeErr(c, http.StatusBadRequest, "taskId 不能为空")
		return
	}
	if !h.m.ReportResult(req.AgentID, req.TaskID, req.Result, req.Error, req.Steps) {
		writeErr(c, http.StatusNotFound, "任务不存在或已结束")
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// ---- Dashboard API ----

func (h *apiHandler) stats(c *gin.Context) {
	c.JSON(http.StatusOK, h.m.Stats())
}

func (h *apiHandler) listAgents(c *gin.Context) {
	c.JSON(http.StatusOK, h.m.ListAgents())
}

func (h *apiHandler) listTasks(c *gin.Context) {
	c.JSON(http.StatusOK, h.m.ListTasks())
}

func (h *apiHandler) getTask(c *gin.Context) {
	t, ok := h.m.GetTask(c.Param("id"))
	if !ok {
		writeErr(c, http.StatusNotFound, "任务不存在")
		return
	}
	c.JSON(http.StatusOK, t)
}

func (h *apiHandler) createTask(c *gin.Context) {
	var req struct {
		AgentID string `json:"agentId"`
		Prompt  string `json:"prompt"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		writeErr(c, http.StatusBadRequest, "请求格式错误: "+err.Error())
		return
	}
	req.Prompt = strings.TrimSpace(req.Prompt)
	if req.Prompt == "" {
		writeErr(c, http.StatusBadRequest, "prompt 不能为空")
		return
	}
	if len([]rune(req.Prompt)) > 2000 {
		writeErr(c, http.StatusBadRequest, "prompt 过长（上限 2000 字）")
		return
	}
	t, err := h.m.CreateTask(req.AgentID, req.Prompt)
	if err != nil {
		writeErr(c, http.StatusBadRequest, err.Error())
		return
	}
	c.JSON(http.StatusOK, t)
}

func (h *apiHandler) listLogs(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "200"))
	logs := h.m.ListLogs(limit)
	level := c.Query("level")
	if level != "" && level != "all" {
		filtered := logs[:0]
		for _, l := range logs {
			if l.Level == level {
				filtered = append(filtered, l)
			}
		}
		logs = filtered
	}
	c.JSON(http.StatusOK, logs)
}
