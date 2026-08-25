// 诗词飞花令 Agent —— 支持网页版对话框（SSE 流式）与命令行调试两种模式。
//
// 架构分层：
//
//	前端 web/index.html  ── POST /api/chat（SSE 流式）
//	      │
//	网关层 internal/gateway ── HTTP 路由 / 会话管理 / SSE 封装 / 静态文件托管
//	      │
//	Agent 层 internal/agent ── 加载 skill / 维护 history / function calling 编排 / 流式回调
//	      │
//	工具层 internal/tools ── 工具注册表（依赖注入 store）
//	      │
//	LLM 客户端 internal/llm ── OpenAI 兼容协议（流式 SSE 解析）
//	      │
//	存储层 internal/store ── 诗词数据访问抽象（当前 memory 实现）
//
// 运行：
//
//	export HY_API_KEY=xxx             # 按默认模型设置对应 key
//	go run ./cmd/server               # 默认启动 Web 网关 http://localhost:8082
//	go run ./cmd/server -addr :9090   # 指定监听地址
//	go run ./cmd/server -cli          # 命令行调试模式
//	go run ./cmd/server -config ./my.json
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/bealzhao1/shiling/internal/cli"
	"github.com/bealzhao1/shiling/internal/config"
	"github.com/bealzhao1/shiling/internal/gateway"
	"github.com/bealzhao1/shiling/internal/skill"
	"github.com/bealzhao1/shiling/internal/store/memory"
)

const (
	skillPath         = "skills/shiling/SKILL.md"
	defaultConfigPath = "config.json"
	defaultAddr       = ":8082"
	defaultWebDir     = "web"
)

func main() {
	configPath := flag.String("config", defaultConfigPath, "模型配置文件路径（JSON）")
	addr := flag.String("addr", defaultAddr, "Web 网关监听地址")
	webFlag := flag.String("web", defaultWebDir, "前端静态文件目录")
	cliMode := flag.Bool("cli", false, "以命令行模式运行（默认启动 Web 网关）")
	flag.Parse()

	// 0. 解析资源路径：支持从项目根或任意子目录运行
	cfgFile := resolvePath(*configPath)
	skillFile := resolvePath(skillPath)
	webDir := resolvePath(*webFlag)

	// 1. 加载技能（缺失则降级）
	sk, err := skill.Load(skillFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "⚠️ 技能加载失败: %v（跳过技能注入）\n", err)
		sk = &skill.Skill{Name: "shiling-poetry-agent", Body: "你是诗词专家。"}
	}

	// 2. 加载模型配置（缺失则退出）
	cfg, err := config.Load(cfgFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ 加载配置文件 %s 失败: %v\n", cfgFile, err)
		os.Exit(1)
	}

	// 3. 初始化诗词存储（默认内存实现；后续可替换为 SQLite/MySQL/ES/向量库）
	st := memory.New()

	// 4. 启动
	if *cliMode {
		cli.Run(cfg, sk, st)
		return
	}

	if err := gateway.EnsureWebDir(webDir); err != nil {
		fmt.Fprintf(os.Stderr, "❌ %v\n", err)
		os.Exit(1)
	}
	srv := gateway.New(cfg, sk, st, webDir)
	if err := srv.Run(*addr); err != nil {
		fmt.Fprintf(os.Stderr, "❌ 网关启动失败: %v\n", err)
		os.Exit(1)
	}
}

// resolvePath 解析资源/配置文件路径，使程序可从任意工作目录启动：
//  1. 绝对路径直接返回；
//  2. 相对路径先按当前工作目录解析，存在则返回；
//  3. 否则回退到项目根目录（含 go.mod 的目录）拼接。
func resolvePath(p string) string {
	if p == "" || filepath.IsAbs(p) {
		return p
	}
	if _, err := os.Stat(p); err == nil {
		return p
	}
	if root := projectRoot(); root != "" {
		candidate := filepath.Join(root, p)
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return p
}

// projectRoot 从当前工作目录向上查找包含 go.mod 的目录，作为项目根；
// 找不到则返回空串。
func projectRoot() string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}
