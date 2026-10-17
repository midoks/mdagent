package server

import (
	"html/template"
	"io/fs"
	"net/http"

	"github.com/gin-gonic/gin"

	"mdagent/internal/core"
	"mdagent/web"
)

// Server 是中心端 HTTP 服务：Dashboard 页面 + Agent/任务/日志 API。
type Server struct {
	manager *core.Manager
	engine  *gin.Engine
}

func New(m *core.Manager) (*Server, error) {
	r := gin.New()
	r.Use(gin.Recovery())

	tpl, err := template.New("").ParseFS(web.Assets, "templates/*.html")
	if err != nil {
		return nil, err
	}
	r.SetHTMLTemplate(tpl)

	staticSub, err := fs.Sub(web.Assets, "static")
	if err != nil {
		return nil, err
	}
	r.StaticFS("/static", http.FS(staticSub))

	s := &Server{manager: m, engine: r}
	h := &apiHandler{m: m}

	// 页面
	r.GET("/", s.page("index.html", "overview", "概览"))
	r.GET("/agents", s.page("agents.html", "agents", "Agent"))
	r.GET("/tasks", s.page("tasks.html", "tasks", "任务"))
	r.GET("/logs", s.page("logs.html", "logs", "日志"))

	// Agent 侧协议
	r.POST("/api/agents/register", h.registerAgent)
	r.POST("/api/agents/heartbeat", h.heartbeat)
	r.POST("/api/tasks/report", h.reportTask)

	// Dashboard API
	r.GET("/api/stats", h.stats)
	r.GET("/api/agents", h.listAgents)
	r.GET("/api/tasks", h.listTasks)
	r.GET("/api/tasks/:id", h.getTask)
	r.POST("/api/tasks", h.createTask)
	r.GET("/api/logs", h.listLogs)

	return s, nil
}

func (s *Server) page(tplName, page, title string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.HTML(http.StatusOK, tplName, gin.H{"Page": page, "Title": title})
	}
}

func (s *Server) Run(addr string) error {
	return s.engine.Run(addr)
}
