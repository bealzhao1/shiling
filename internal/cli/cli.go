// Package cli 提供命令行调试模式，复用 agent 层，便于不启动 Web 网关时直接对话调试。
package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/bealzhao1/shiling/internal/agent"
	"github.com/bealzhao1/shiling/internal/config"
	"github.com/bealzhao1/shiling/internal/skill"
	"github.com/bealzhao1/shiling/internal/store"
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

// Run 命令行调试模式，复用 agent 层。
func Run(cfg *config.Config, sk *skill.Skill, st store.Store) {
	ag := agent.New(cfg, sk, st)
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
			if switchErr := ag.SwitchModel(parts[1]); switchErr != nil {
				fmt.Println("❌", switchErr)
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
