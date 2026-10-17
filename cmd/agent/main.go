package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"mdagent/internal/llm"
	"mdagent/internal/runtime"
	"mdagent/internal/tools"
)

func main() {
	serverURL := flag.String("server", "http://127.0.0.1:8080", "中心端地址")
	name := flag.String("name", "", "Agent 名称（默认自动生成）")
	providerName := flag.String("provider", "auto", "LLM Provider: auto | mock | openai")
	flag.Parse()

	if *name == "" {
		host, _ := os.Hostname()
		if host == "" {
			host = "node"
		}
		*name = fmt.Sprintf("%s-%d", host, os.Getpid())
	}

	provider := pickProvider(*providerName)
	log.Printf("[agent] 使用 LLM Provider: %s", provider.Name())

	registry := tools.NewRegistry()
	for _, t := range tools.Builtin() {
		registry.Register(t)
	}

	node := runtime.NewNode(runtime.NodeConfig{
		Name:      *name,
		ServerURL: *serverURL,
		Provider:  provider,
		Registry:  registry,
	})

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := node.Run(ctx); err != nil {
		log.Fatalf("[agent] 运行失败: %v", err)
	}
}

// pickProvider 按配置选择 LLM Provider：
//   - openai: 强制使用 OpenAI 兼容协议（需要 OPENAI_API_KEY）
//   - mock:   强制使用本地 Mock
//   - auto:   有 OPENAI_API_KEY 则用 OpenAI，否则 Mock
func pickProvider(name string) llm.Provider {
	switch name {
	case "mock":
		return llm.NewMock()
	case "openai":
		p := llm.NewOpenAIFromEnv()
		if !p.Available() {
			log.Fatal("[agent] provider=openai 但未设置 OPENAI_API_KEY")
		}
		return p
	default:
		if p := llm.NewOpenAIFromEnv(); p.Available() {
			return p
		}
		return llm.NewMock()
	}
}
