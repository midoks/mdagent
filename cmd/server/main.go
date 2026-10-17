package main

import (
	"flag"
	"log"
	"time"

	"github.com/gin-gonic/gin"

	"mdagent/internal/core"
	"mdagent/internal/server"
)

func main() {
	addr := flag.String("addr", ":8080", "HTTP 监听地址")
	flag.Parse()

	gin.SetMode(gin.ReleaseMode)

	store := core.NewStore()
	mgr := core.NewManager(store)

	// 心跳超时巡检：超过 15s 未心跳的 Agent 标记离线。
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			mgr.SweepOffline(15 * time.Second)
		}
	}()

	srv, err := server.New(mgr)
	if err != nil {
		log.Fatalf("初始化服务失败: %v", err)
	}

	log.Printf("[server] Dashboard: http://localhost%s", *addr)
	if err := srv.Run(*addr); err != nil {
		log.Fatalf("服务退出: %v", err)
	}
}
