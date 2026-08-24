// Package config 负责加载模型配置文件，支持多模型切换。
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// ModelConfig 单个模型提供商的配置。
type ModelConfig struct {
	Provider    string `json:"provider"`              // 厂商标识，如 deepseek / openai / anthropic / custom
	BaseURL     string `json:"base_url"`              // API 网关地址（含 /v1 等前缀）
	Model       string `json:"model"`                 // 上游真实模型名
	APIKey      string `json:"api_key,omitempty"`     // 可选：直接写死的 key（不推荐）
	APIKeyEnv   string `json:"api_key_env,omitempty"` // 优先从此环境变量读取 key
	NoAuth      bool   `json:"no_auth,omitempty"`     // 本地服务（如 Ollama）无需鉴权
	Description string `json:"description,omitempty"` // 人类可读的备注，会在 /models 中展示
}

// Config 整个配置文件结构。
type Config struct {
	DefaultModel string                  `json:"default_model"` // 启动时默认使用的模型 key
	Models       map[string]*ModelConfig `json:"models"`        // 模型清单：key → 配置
}

// Load 从 path 加载并解析配置文件。文件不存在时返回 error。
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取配置失败: %w", err)
	}
	var c Config
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("解析配置失败: %w", err)
	}
	if err := c.Validate(); err != nil {
		return nil, err
	}
	return &c, nil
}

// Validate 校验配置完整性。
func (c *Config) Validate() error {
	if len(c.Models) == 0 {
		return fmt.Errorf("配置中未定义任何模型（models 不能为空）")
	}
	if c.DefaultModel == "" {
		// 默认使用第一个 key
		for k := range c.Models {
			c.DefaultModel = k
			break
		}
	}
	if _, ok := c.Models[c.DefaultModel]; !ok {
		return fmt.Errorf("default_model %q 在 models 中不存在", c.DefaultModel)
	}
	for name, m := range c.Models {
		if m.BaseURL == "" {
			return fmt.Errorf("模型 %q 缺少 base_url", name)
		}
		if m.Model == "" {
			return fmt.Errorf("模型 %q 缺少 model", name)
		}
		if m.APIKey == "" && m.APIKeyEnv == "" && !m.NoAuth {
			return fmt.Errorf("模型 %q 缺少 api_key 或 api_key_env（本地服务可设置 no_auth: true）", name)
		}
	}
	return nil
}

// Get 取得指定 key 的模型配置，不存在返回 nil。
func (c *Config) Get(name string) *ModelConfig {
	return c.Models[name]
}

// Names 按 key 排序返回所有模型名，方便 /models 列表展示。
func (c *Config) Names() []string {
	names := make([]string, 0, len(c.Models))
	for k := range c.Models {
		names = append(names, k)
	}
	// 简单排序，避免 map 顺序抖动
	for i := 1; i < len(names); i++ {
		for j := i; j > 0 && names[j-1] > names[j]; j-- {
			names[j-1], names[j] = names[j], names[j-1]
		}
	}
	return names
}

// ResolveAPIKey 优先用显式 api_key，其次读环境变量 api_key_env。
func (m *ModelConfig) ResolveAPIKey() string {
	if m.APIKey != "" {
		return m.APIKey
	}
	if m.APIKeyEnv != "" {
		return os.Getenv(m.APIKeyEnv)
	}
	return ""
}

// DisplayName 返回模型在 /models 列表中的展示字符串。
func (m *ModelConfig) DisplayName(name string) string {
	desc := m.Description
	if desc == "" {
		desc = fmt.Sprintf("%s · %s", m.Provider, m.Model)
	}
	return fmt.Sprintf("%-16s → %s  (%s)", name, m.Model, strings.TrimSpace(desc))
}
