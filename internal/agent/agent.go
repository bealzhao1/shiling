// Package agent 是飞花令 Agent 的核心编排层：加载技能、维护对话历史、
// 驱动 function calling 多轮循环，并通过事件回调对外输出（流式文本增量 / 工具调用 / 完成 / 错误）。
//
// 网关层（gateway）与命令行模式（CLI）都复用本层的 StreamChat。
//
// 依赖关系：agent → llm（模型调用）、tools（工具注册表）、skill（技能提示）。
// 工具注册表内部依赖 store（诗词数据），由外部注入，实现数据源解耦。
package agent

import (
	"fmt"
	"sync"

	"github.com/bealzhao1/shiling/internal/config"
	"github.com/bealzhao1/shiling/internal/llm"
	"github.com/bealzhao1/shiling/internal/skill"
	"github.com/bealzhao1/shiling/internal/store"
	"github.com/bealzhao1/shiling/internal/tools"
)

// EventType 输出事件类型。
type EventType string

const (
	EventDelta    EventType = "delta"     // 文本增量，Content 字段承载
	EventToolCall EventType = "tool_call" // 工具调用，Name / Arguments 字段承载
	EventDone     EventType = "done"      // 本轮完成
	EventError    EventType = "error"     // 出错，Error 字段承载
)

// Event 一次对外输出事件。Type 不参与 JSON 序列化（由网关层的 SSE event 名承载）。
type Event struct {
	Type      EventType `json:"-"`
	Content   string    `json:"content,omitempty"`
	Name      string    `json:"name,omitempty"`
	Arguments string    `json:"arguments,omitempty"`
	Error     string    `json:"error,omitempty"`
}

// Emitter 事件回调。返回 error 可中断本轮（例如客户端断连）。
type Emitter func(Event) error

// Agent 单个会话的飞花令 Agent 实例。
type Agent struct {
	cfg    *config.Config
	skill  *skill.Skill
	client *llm.Client
	tools  *tools.Registry

	mu           sync.Mutex // 串行化单会话内的并发请求
	history      []llm.Message
	currentModel string
}

// New 创建 Agent 实例，使用配置中的默认模型。
func New(cfg *config.Config, sk *skill.Skill, st store.Store) *Agent {
	mc := cfg.Get(cfg.DefaultModel)
	return &Agent{
		cfg:          cfg,
		skill:        sk,
		client:       llm.NewClient(mc),
		tools:        tools.New(st),
		currentModel: cfg.DefaultModel,
		history: []llm.Message{
			{Role: "system", Content: sk.SystemPrompt()},
		},
	}
}

// CurrentModel 返回当前使用的模型 key。
func (a *Agent) CurrentModel() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.currentModel
}

// UpstreamModel 返回上游真实模型名（用于展示）。
func (a *Agent) UpstreamModel() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.client.Model
}

// SwitchModel 热切换到指定模型 key（不丢失历史）。
func (a *Agent) SwitchModel(name string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	mc := a.cfg.Get(name)
	if mc == nil {
		return fmt.Errorf("未知模型 %q", name)
	}
	a.client.Switch(mc)
	a.currentModel = name
	return nil
}

// Reset 清空对话历史（保留 system 提示）。
func (a *Agent) Reset() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.history = a.history[:1]
}

// StreamChat 处理一条用户输入，通过 emit 持续输出事件。
//
// 流程：
//  1. 用户消息入历史
//  2. 调用当前模型（流式），文本增量实时 emit EventDelta
//  3. 若模型请求工具调用：emit EventToolCall → 执行 → 结果回填历史 → 继续下一轮
//  4. 直到模型返回纯文本 → emit EventDone
//  5. 出错 → emit EventError 并回滚本条用户消息
func (a *Agent) StreamChat(userInput string, emit Emitter) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	// 1. 入历史
	a.history = append(a.history, llm.Message{Role: "user", Content: userInput})

	// 2~4. 工具调用多轮循环
	for {
		msg, err := a.client.ChatStream(a.history, a.tools.Defs(), 0.8, func(delta string) {
			_ = emit(Event{Type: EventDelta, Content: delta})
		})
		if err != nil {
			_ = emit(Event{Type: EventError, Error: err.Error()})
			a.history = a.history[:len(a.history)-1] // 回滚失败的 user 消息
			return err
		}

		// 模型请求调用工具
		if len(msg.ToolCalls) > 0 {
			a.history = append(a.history, *msg)
			for _, tc := range msg.ToolCalls {
				if err := emit(Event{Type: EventToolCall, Name: tc.Function.Name, Arguments: tc.Function.Arguments}); err != nil {
					return err
				}
				result := a.tools.Execute(tc)
				a.history = append(a.history, llm.Message{
					Role:       "tool",
					Content:    result,
					ToolCallID: tc.ID,
					Name:       tc.Function.Name,
				})
			}
			continue
		}

		// 正常回复
		a.history = append(a.history, *msg)
		return emit(Event{Type: EventDone})
	}
}
