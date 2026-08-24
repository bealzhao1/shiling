package skills

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadFromDisk(t *testing.T) {
	// 用项目真实的 SKILL.md 测试
	path := filepath.Join("..", "..", "skills", "shiling", "SKILL.md")
	s, err := Load(path)
	if err != nil {
		t.Fatalf("加载失败: %v", err)
	}
	if s.Name != "shiling-poetry-agent" {
		t.Errorf("name 解析错误: %q", s.Name)
	}
	if !strings.Contains(s.Description, "飞花令") {
		t.Errorf("description 应包含触发词: %q", s.Description[:50])
	}
	if !strings.Contains(s.Body, "When to use") {
		t.Errorf("正文应包含 Steps: %q", s.Body[:60])
	}
	sp := s.SystemPrompt()
	if !strings.Contains(sp, "SKILL.md 开始") || !strings.Contains(sp, "search_poems") {
		t.Error("SystemPrompt 应注入技能正文并提及工具")
	}
}

func TestLoadMissingFile(t *testing.T) {
	path := filepath.Join(os.TempDir(), "no_such_skill_"+t.Name(), "SKILL.md")
	if _, err := Load(path); err == nil {
		t.Error("文件不存在时应返回错误")
	}
}
