// Package tools 定义 Agent 可调用的工具。
package tools

import (
	"encoding/json"
	"fmt"

	"shiling/internal/llm"
	"shiling/internal/poems"
)

// searchPoemsArgs 工具参数。
type searchPoemsArgs struct {
	Keyword string `json:"keyword"` // 令字或主题词，如 "月"、"花"、"思念"
}

// poemResult 返回给模型的结果结构。
type poemResult struct {
	Keyword string       `json:"keyword"`
	Count   int          `json:"count"`
	Poems   []poems.Poem `json:"poems"`
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
func Defs() []llm.Tool {
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
func Execute(tc llm.ToolCall) string {
	var args searchPoemsArgs
	if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
		return fmt.Sprintf(`{"error":"参数解析失败: %v"}`, err)
	}
	switch tc.Function.Name {
	case "search_poems":
		return searchPoems(args.Keyword)
	default:
		return fmt.Sprintf(`{"error":"未知工具: %s"}`, tc.Function.Name)
	}
}

// searchPoems 检索诗词库并返回 JSON 结果。
func searchPoems(keyword string) string {
	if keyword == "" {
		return `{"error":"keyword 不能为空"}`
	}
	results := poems.Search(keyword)
	// 去重（诗词库可能因多关键字重复收录同一首）
	seen := map[string]bool{}
	var uniq []poems.Poem
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
