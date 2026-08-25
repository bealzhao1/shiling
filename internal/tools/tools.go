// Package tools 定义 Agent 可调用的工具，通过依赖注入的 store.Store 访问诗词数据。
//
// 当前为「裸写工具」实现；后续演进为 MCP Server 时，本层会退化为 MCP 客户端，
// 工具定义与执行逻辑迁移到独立的 MCP Server 中。
package tools

import (
	"encoding/json"
	"fmt"

	"github.com/bealzhao1/shiling/internal/llm"
	"github.com/bealzhao1/shiling/internal/store"
)

// Registry 工具注册表：持有执行工具所需的依赖（诗词存储）。
type Registry struct {
	store store.Store
}

// New 创建工具注册表。
func New(s store.Store) *Registry {
	return &Registry{store: s}
}

// searchPoemsArgs 工具参数。
type searchPoemsArgs struct {
	Keyword string `json:"keyword"` // 令字或主题词，如 "月"、"花"、"思念"
}

// poemResult 返回给模型的结果结构。
type poemResult struct {
	Keyword string       `json:"keyword"`
	Count   int          `json:"count"`
	Poems   []store.Poem `json:"poems"`
}

// 工具参数 JSON Schema。
var searchPoemsSchema = `{
	"type": "object",
	"properties": {
		"keyword": {
			"type": "string",
			"description": "令字或主题词，如：月、花、风、春、山、水、云、雪、日、人、夜"
		}
	},
	"required": ["keyword"],
	"additionalProperties": false
}`

// Defs 返回注册给模型的所有工具定义。
func (r *Registry) Defs() []llm.Tool {
	return []llm.Tool{
		{
			Type: "function",
			Function: llm.Function{
				Name:        "search_poems",
				Description: "按关键字（令字或主题词）从本地诗词库检索诗句，返回匹配的诗句、作者与诗名。用于飞花令、接龙、主题推荐。",
				Parameters:  json.RawMessage(searchPoemsSchema),
			},
		},
	}
}

// Execute 执行模型发起的工具调用，返回结果字符串。
func (r *Registry) Execute(tc llm.ToolCall) string {
	var args searchPoemsArgs
	if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
		return fmt.Sprintf(`{"error":"参数解析失败: %v"}`, err)
	}
	switch tc.Function.Name {
	case "search_poems":
		return r.searchPoems(args.Keyword)
	default:
		return fmt.Sprintf(`{"error":"未知工具: %s"}`, tc.Function.Name)
	}
}

// searchPoems 检索诗词库并返回 JSON 结果。
func (r *Registry) searchPoems(keyword string) string {
	if keyword == "" {
		return `{"error":"keyword 不能为空"}`
	}
	results := r.store.Search(keyword)
	// 去重（诗词库可能因多关键字重复收录同一首）
	seen := map[string]bool{}
	var uniq []store.Poem
	for _, p := range results {
		if !seen[p.Text] {
			seen[p.Text] = true
			uniq = append(uniq, p)
		}
	}
	out, _ := json.Marshal(poemResult{
		Keyword: keyword,
		Count:   len(uniq),
		Poems:   uniq,
	})
	return string(out)
}
