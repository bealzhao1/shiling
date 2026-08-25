// Package llm 封装 OpenAI 兼容协议的 Chat Completions API（DeepSeek / OpenAI / Moonshot / 阿里云 / 混元 等），
// 支持 function calling 与流式（SSE）输出。
//
// 配置由 internal/config 提供，本包不再持有任何 base_url / model 常量。
package llm

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/bealzhao1/shiling/internal/config"
)

// Message 对话消息。
type Message struct {
	Role       string     `json:"role"`                   // system / user / assistant / tool
	Content    string     `json:"content"`                // 文本内容
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`   // assistant 请求的调用
	ToolCallID string     `json:"tool_call_id,omitempty"` // tool 角色的回执
	Name       string     `json:"name,omitempty"`         // 工具名（可选）
}

// ToolCall 模型发起的工具调用请求。
type ToolCall struct {
	Index    int      `json:"index,omitempty"` // 流式返回时的工具序号
	ID       string   `json:"id"`
	Type     string   `json:"type"`
	Function ToolFunc `json:"function"`
}

// ToolFunc 工具调用的函数名与参数（参数为 JSON 字符串）。
type ToolFunc struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// Tool 注册给模型使用的工具定义（OpenAI tools 格式）。
type Tool struct {
	Type     string   `json:"type"`
	Function Function `json:"function"`
}

// Function 工具函数元信息。
type Function struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"` // JSON Schema
}

// Client 通用 OpenAI 兼容 API 客户端。由 config.ModelConfig 驱动。
type Client struct {
	BaseURL  string       // API 网关地址
	APIKey   string       // 已解析好的 key
	Model    string       // 上游真实模型名
	Provider string       // 厂商标识（仅用于日志）
	NoAuth   bool         // 本地服务无需鉴权（不发送 Authorization 头）
	HTTP     *http.Client // 复用 HTTP 客户端
}

// NewClient 根据配置项构造客户端，并解析 API Key。
func NewClient(mc *config.ModelConfig) *Client {
	return &Client{
		BaseURL:  mc.BaseURL,
		APIKey:   mc.ResolveAPIKey(),
		Model:    mc.Model,
		Provider: mc.Provider,
		NoAuth:   mc.NoAuth,
		HTTP:     &http.Client{Timeout: 0}, // 流式请求不设整体超时，由调用方控制生命周期
	}
}

// Switch 热切换到另一个模型配置（不创建新的 Client，复用 http 连接池）。
func (c *Client) Switch(mc *config.ModelConfig) {
	c.BaseURL = mc.BaseURL
	c.APIKey = mc.ResolveAPIKey()
	c.Model = mc.Model
	c.Provider = mc.Provider
	c.NoAuth = mc.NoAuth
}

// ChatRequest 一次对话请求。
type ChatRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Tools       []Tool    `json:"tools,omitempty"`
	Temperature float64   `json:"temperature,omitempty"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
	Stream      bool      `json:"stream,omitempty"`
}

// ChatResponse API 返回（非流式）。
type ChatResponse struct {
	ID      string `json:"id"`
	Choices []struct {
		Index   int     `json:"index"`
		Message Message `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
}

// streamChunk 流式返回的单条增量。
type streamChunk struct {
	Choices []struct {
		Index int `json:"index"`
		Delta struct {
			Role      string     `json:"role"`
			Content   string     `json:"content"`
			ToolCalls []ToolCall `json:"tool_calls"`
		} `json:"delta"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
}

// Chat 发送一轮对话（非流式），返回 assistant 回复。支持工具定义。
func (c *Client) Chat(messages []Message, tools []Tool, temperature float64) (*Message, error) {
	body, err := json.Marshal(ChatRequest{
		Model:       c.Model,
		Messages:    messages,
		Tools:       tools,
		Temperature: temperature,
	})
	if err != nil {
		return nil, fmt.Errorf("序列化请求失败: %w", err)
	}

	httpReq, err := c.buildRequest(body)
	if err != nil {
		return nil, err
	}
	resp, err := c.HTTP.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("请求 %s API 失败: %w", c.Provider, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s API 返回 %d: %s", c.Provider, resp.StatusCode, string(respBody))
	}

	var parsed ChatResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}
	if parsed.Error != nil {
		return nil, fmt.Errorf("%s 错误: %s", c.Provider, parsed.Error.Message)
	}
	if len(parsed.Choices) == 0 {
		return nil, fmt.Errorf("%s 返回空选择", c.Provider)
	}
	return &parsed.Choices[0].Message, nil
}

// ChatStream 发送一轮流式对话。onDelta 会在每个文本增量到达时被调用；
// 返回累积完成的 assistant Message（含完整 content 与 tool_calls）。
func (c *Client) ChatStream(messages []Message, tools []Tool, temperature float64, onDelta func(string)) (*Message, error) {
	body, err := json.Marshal(ChatRequest{
		Model:       c.Model,
		Messages:    messages,
		Tools:       tools,
		Temperature: temperature,
		Stream:      true,
	})
	if err != nil {
		return nil, fmt.Errorf("序列化请求失败: %w", err)
	}

	httpReq, err := c.buildRequest(body)
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Accept", "text/event-stream")

	resp, err := c.HTTP.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("请求 %s API 失败: %w", c.Provider, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("%s API 返回 %d: %s", c.Provider, resp.StatusCode, string(respBody))
	}

	var (
		content strings.Builder
		tcMap   = map[int]*ToolCall{}
		tcOrder []int
	)

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024) // 增大 buffer 以容纳长 arguments
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "[DONE]" {
			break
		}

		var chunk streamChunk
		if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
			continue // 忽略无法解析的行（如空 data、心跳注释）
		}
		if len(chunk.Choices) == 0 {
			continue
		}
		delta := chunk.Choices[0].Delta

		if delta.Content != "" {
			content.WriteString(delta.Content)
			if onDelta != nil {
				onDelta(delta.Content)
			}
		}

		for _, tc := range delta.ToolCalls {
			if cur, ok := tcMap[tc.Index]; ok {
				// 增量拼接：name/id 只在首个 chunk 出现，arguments 逐段累积
				if tc.ID != "" {
					cur.ID = tc.ID
				}
				if tc.Type != "" {
					cur.Type = tc.Type
				}
				if tc.Function.Name != "" {
					cur.Function.Name = tc.Function.Name
				}
				cur.Function.Arguments += tc.Function.Arguments
			} else {
				cp := tc
				tcMap[tc.Index] = &cp
				tcOrder = append(tcOrder, tc.Index)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("读取 %s 流式响应失败: %w", c.Provider, err)
	}

	msg := &Message{Role: "assistant", Content: content.String()}
	for _, idx := range tcOrder {
		msg.ToolCalls = append(msg.ToolCalls, *tcMap[idx])
	}
	return msg, nil
}

// buildRequest 构造带鉴权头的 POST /chat/completions 请求。
func (c *Client) buildRequest(body []byte) (*http.Request, error) {
	if c.APIKey == "" && !c.NoAuth {
		return nil, fmt.Errorf("未设置 API Key（请检查对应模型配置的 api_key / api_key_env，或本地服务设置 no_auth: true）")
	}
	httpReq, err := http.NewRequest("POST", c.BaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("构造请求失败: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if c.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.APIKey)
	}
	return httpReq, nil
}
