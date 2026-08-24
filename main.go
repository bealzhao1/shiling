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
//	LLM 客户端 internal/llm ── OpenAI 兼容协议（流式 SSE 解析）
//
// 运行：
//
//	export HY_API_KEY=xxx        # 按默认模型设置对应 key
//	go run .                     # 默认启动 Web 网关 http://localhost:8080
//	go run . -addr :9090         # 指定监听地址
//	go run . -cli                # 命令行调试模式
//	go run . -config ./my.json   # 指定配置文件
package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strings"

	"shiling/internal/agent"
	"shiling/internal/config"
	"shiling/internal/gateway"
	"shiling/internal/skills"
)

const (
	skillPath        = "skills/shiling/SKILL.md"
	defaultConfigPath = "config.json"
	defaultAddr       = ":8082"
	defaultWebDir     = "web"
)

// CLI 命令常量。
const (
	cmdHelp   = "/help"
	cmdExit   = "/exit"
	cmdQuit   = "/quit"
	cmdClear  = "/clear"
	cmdModels = "/models"
	cmdModel  = "/model"
)

func main() {
	configPath := flag.String("config", defaultConfigPath, "模型配置文件路径（JSON）")
	addr := flag.String("addr", defaultAddr, "Web 网关监听地址")
	webDir := flag.String("web", defaultWebDir, "前端静态文件目录")
	cli := flag.Bool("cli", false, "以命令行模式运行（默认启动 Web 网关）")
	flag.Parse()

	// 1. 加载技能（缺失则降级）
	skill, err := skills.Load(skillPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "⚠️ 技能加载失败: %v（跳过技能注入）\n", err)
		skill = &skills.Skill{Name: "shiling-poetry-agent", Body: "你是诗词专家。"}
	}

	// 2. 加载模型配置（缺失则退出）
	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ 加载配置文件 %s 失败: %v\n", *configPath, err)
		os.Exit(1)
	}

	// 3. 启动
	if *cli {
		runCLI(cfg, skill)
		return
	}

	if err := gateway.EnsureWebDir(*webDir); err != nil {
		fmt.Fprintf(os.Stderr, "❌ %v\n", err)
		os.Exit(1)
	}
	srv := gateway.New(cfg, skill, *webDir)
	if err := srv.Run(*addr); err != nil {
		fmt.Fprintf(os.Stderr, "❌ 网关启动失败: %v\n", err)
		os.Exit(1)
	}
}

// runCLI 命令行调试模式，复用 agent 层。
func runCLI(cfg *config.Config, skill *skills.Skill) {
	ag := agent.New(cfg, skill)
	fmt.Println("🌸 诗词飞花令 Agent（CLI 调试模式）")
	fmt.Printf("   当前模型: %s\n", ag.CurrentModel())
	fmt.Println("   输入 /help 查看命令，/exit 退出")

	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print("你 > ")
		input, err := reader.ReadString('\n')
		if err != nil {
			fmt.Println("\n再见 🌸")
			return
		}
		input = strings.TrimSpace(input)

		switch {
		case input == cmdExit || input == cmdQuit:
			fmt.Println("再见 🌸")
			return
		case input == cmdHelp:
			printHelp()
			continue
		case input == cmdClear:
			ag.Reset()
			fmt.Println("🧹 对话历史已清空")
			continue
		case input == cmdModels:
			for _, name := range cfg.Names() {
				marker := "  "
				if name == ag.CurrentModel() {
					marker = "★ "
				}
				fmt.Printf("  %s%s\n", marker, cfg.Get(name).DisplayName(name))
			}
			continue
		case strings.HasPrefix(input, cmdModel):
			parts := strings.Fields(input)
			if len(parts) == 1 {
				fmt.Printf("当前模型: %s\n", ag.CurrentModel())
				continue
			}
			if err := ag.SwitchModel(parts[1]); err != nil {
				fmt.Println("❌", err)
				continue
			}
			fmt.Printf("✅ 已切换到模型: %s\n", ag.CurrentModel())
			continue
		case input == "":
			continue
		}

		fmt.Print("Agent > ")
		err = ag.StreamChat(input, func(ev agent.Event) error {
			switch ev.Type {
			case agent.EventDelta:
				fmt.Print(ev.Content)
			case agent.EventToolCall:
				fmt.Printf("\n🔧 调用工具: %s(%s)\nAgent > ", ev.Name, ev.Arguments)
			case agent.EventDone:
				fmt.Println()
			case agent.EventError:
				fmt.Println("\n❌", ev.Error)
			}
			return nil
		})
		if err != nil {
			fmt.Println("❌", err)
		}
		fmt.Println()
	}
}

func printHelp() {
	fmt.Println("📖 命令说明：")
	fmt.Println("  以\"X\"行令       飞花令：输出含 X 字的诗句")
	fmt.Println("  接龙\"诗句\"     接龙：以末字接下一句")
	fmt.Println("  写XX的诗/推荐   主题推荐")
	fmt.Println("  这句诗什么意思  诗句解读")
	fmt.Println("  /clear          清空对话历史")
	fmt.Println("  /models         列出所有可用模型")
	fmt.Println("  /model <name>   切换模型")
	fmt.Println("  /help           帮助")
	fmt.Println("  /exit           退出")
}
