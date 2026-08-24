// Package skills 解析 SKILL.md 技能文件，供 Agent 注入系统提示。
package skills

import (
	"fmt"
	"os"
	"strings"
)

// Skill 一个技能的内容。
type Skill struct {
	Name        string // 技能名
	Description string // 触发条件描述
	Body        string // 正文（行为手册）
}

// Load 从磁盘加载并解析一个 SKILL.md 文件。
// 格式：
//
//	---
//	name: xxx
//	description: xxx
//	---
//	# 正文
func Load(path string) (*Skill, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取技能文件失败: %w", err)
	}
	content := string(data)

	// 解析 frontmatter（--- 包裹的 YAML 头）
	parts := strings.SplitN(content, "---", 3)
	skill := &Skill{Name: "unknown"}
	if len(parts) == 3 {
		// 逐行解析 name 与 description（支持多行 description）
		lines := strings.Split(parts[1], "\n")
		var descLines []string
		inDesc := false
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			switch {
			case strings.HasPrefix(trimmed, "name:"):
				skill.Name = strings.TrimSpace(strings.TrimPrefix(trimmed, "name:"))
			case strings.HasPrefix(trimmed, "description:"):
				inDesc = true
				rest := strings.TrimSpace(strings.TrimPrefix(trimmed, "description:"))
				rest = strings.TrimPrefix(rest, "|")
				rest = strings.TrimSpace(rest)
				if rest != "" {
					descLines = append(descLines, rest)
				}
			case inDesc:
				// description 块内的行（不以新 key 开头且不是空行时继续收集）
				if strings.Contains(trimmed, ":") && isYAMLKey(trimmed) {
					inDesc = false
				} else if trimmed == "" {
					// 空行：若下一行仍是缩进内容则继续，否则结束
					continue
				} else {
					descLines = append(descLines, trimmed)
				}
			}
		}
		skill.Description = strings.Join(descLines, "\n")
		skill.Body = strings.TrimSpace(parts[2])
	} else {
		skill.Body = strings.TrimSpace(content)
	}
	return skill, nil
}

// isYAMLKey 判断一行是否为新的 YAML 顶层键（如 "⚠️ 不适用:" 或 "触发词:"）。
func isYAMLKey(line string) bool {
	// 简单启发式：形如 "key:" 或 "key: value" 且 key 不含空格或为中文词
	idx := strings.Index(line, ":")
	if idx <= 0 {
		return false
	}
	key := strings.TrimSpace(line[:idx])
	// 排除含空格的短语键（YAML 中一般键不含空格）
	return !strings.Contains(key, " ")
}

// SystemPrompt 将技能转换为 Agent 的系统提示。
func (s *Skill) SystemPrompt() string {
	var b strings.Builder
	b.WriteString("你是一个诗词领域的专家 Agent。以下是你的行为手册（SKILL.md），必须严格遵守：\n\n")
	b.WriteString("===== SKILL.md 开始 =====\n")
	b.WriteString(s.Body)
	b.WriteString("\n===== SKILL.md 结束 =====\n\n")
	b.WriteString("你可以调用工具来查询本地诗词库。当需要查诗时，调用 search_poems 工具。")
	return b.String()
}
