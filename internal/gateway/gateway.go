// Package gateway 是 Web 网关层：承接 HTTP 请求，负责路由、会话管理、
// SSE 流式输出封装与前端静态文件托管。
//
// 职责边界：
//   - 只做「输入/输出」：解析用户请求 → 交给 agent → 把事件转成 SSE 推给前端
//   - 不关心业务逻辑（诗词、工具调用等由 agent 层负责）
package gateway

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"shiling/internal/agent"
	"shiling/internal/config"
	"shiling/internal/skills"
)

// session 一个会话：绑定的 Agent 实例 + 最近活跃时间（用于 TTL 清理）。
type session struct {
	agent      *agent.Agent
	lastActive time.Time
}

// Server 网关服务。
type Server struct {
	cfg   *config.Config
	skill *skills.Skill
	web   string // 前端静态文件目录

	mu       sync.RWMutex
	sessions map[string]*session

	sessionTTL time.Duration
}

// New 创建网关服务。
func New(cfg *config.Config, skill *skills.Skill, webDir string) *Server {
	s := &Server{
		cfg:        cfg,
		skill:      skill,
		web:        webDir,
		sessions:   make(map[string]*session),
		sessionTTL: 30 * time.Minute,
	}
	go s.cleanupLoop()
	return s
}

// Run 启动 HTTP 服务并阻塞。返回服务退出错误。
func (s *Server) Run(addr string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", s.handleHealth)
	mux.HandleFunc("/api/chat", s.handleChat)
	mux.HandleFunc("/api/models", s.handleModels)
	mux.HandleFunc("/api/reset", s.handleReset)
	mux.Handle("/", http.FileServer(http.Dir(s.web)))

	handler := s.withCORS(mux)

	fmt.Printf("🚀 飞花令 Agent Web 网关已启动: http://localhost%s\n", addr)
	fmt.Printf("   默认模型: %s\n", s.cfg.DefaultModel)
	fmt.Println("   前端页面: 浏览器打开 http://localhost" + addr)

	srv := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}
	return srv.ListenAndServe()
}

// ---- HTTP handlers ----

// chatRequest 前端发来的对话请求体。
type chatRequest struct {
	Message   string `json:"message"`
	SessionID string `json:"session_id,omitempty"`
	Model     string `json:"model,omitempty"` // 可选：指定本轮使用的模型 key
}

// handleChat POST /api/chat —— SSE 流式对话。
func (s *Server) handleChat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req chatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(req.Message) == "" {
		http.Error(w, "message is required", http.StatusBadRequest)
		return
	}

	// 获取或新建会话
	sess, sid := s.getOrCreate(req.SessionID)

	// 可选：切换模型
	if req.Model != "" && req.Model != sess.agent.CurrentModel() {
		if err := sess.agent.SwitchModel(req.Model); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}

	// 建立 SSE
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	// 首帧：会话元信息（前端据此记住 session_id）
	_ = writeSSE(w, flusher, "meta", map[string]string{
		"session_id": sid,
		"model":      sess.agent.CurrentModel(),
		"upstream":   sess.agent.UpstreamModel(),
	})

	// 交给 agent 编排，把事件转成 SSE
	err := sess.agent.StreamChat(req.Message, func(ev agent.Event) error {
		s.touch(sid)
		switch ev.Type {
		case agent.EventDelta:
			return writeSSE(w, flusher, "delta", map[string]string{"content": ev.Content})
		case agent.EventToolCall:
			return writeSSE(w, flusher, "tool_call", map[string]string{
				"name":      ev.Name,
				"arguments": ev.Arguments,
			})
		case agent.EventDone:
			return writeSSE(w, flusher, "done", map[string]string{})
		case agent.EventError:
			return writeSSE(w, flusher, "error", map[string]string{"message": ev.Error})
		}
		return nil
	})
	// 若 emit 因客户端断连返回错误，这里静默结束；否则补一帧结束标记
	if err != nil {
		_ = writeSSE(w, flusher, "error", map[string]string{"message": err.Error()})
	}
	_ = writeSSE(w, flusher, "close", map[string]string{})
}

// handleHealth GET /healthz —— 健康检查（供 K8s liveness/readiness 探针使用）。
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// handleModels GET /api/models —— 返回模型清单与默认模型。
func (s *Server) handleModels(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	resp := map[string]any{
		"default": s.cfg.DefaultModel,
		"models":  s.cfg.Models,
	}
	_ = json.NewEncoder(w).Encode(resp)
}

// handleReset POST /api/reset —— 重置指定会话的对话历史。
func (s *Server) handleReset(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SessionID string `json:"session_id"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	if req.SessionID != "" {
		s.mu.RLock()
		sess, ok := s.sessions[req.SessionID]
		s.mu.RUnlock()
		if ok {
			sess.agent.Reset()
		}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]bool{"ok": true})
}

// ---- 会话管理 ----

// getOrCreate 按 session_id 取会话，不存在则新建。
func (s *Server) getOrCreate(sessionID string) (*session, string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if sessionID != "" {
		if sess, ok := s.sessions[sessionID]; ok {
			sess.lastActive = time.Now()
			return sess, sessionID
		}
	}
	id := newSessionID()
	sess := &session{agent: agent.New(s.cfg, s.skill), lastActive: time.Now()}
	s.sessions[id] = sess
	return sess, id
}

// touch 更新会话活跃时间。
func (s *Server) touch(sessionID string) {
	s.mu.Lock()
	if sess, ok := s.sessions[sessionID]; ok {
		sess.lastActive = time.Now()
	}
	s.mu.Unlock()
}

// cleanupLoop 定期清理过期会话。
func (s *Server) cleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		now := time.Now()
		s.mu.Lock()
		for id, sess := range s.sessions {
			if now.Sub(sess.lastActive) > s.sessionTTL {
				delete(s.sessions, id)
			}
		}
		s.mu.Unlock()
	}
}

// ---- 工具函数 ----

// writeSSE 写一帧 SSE 事件并 flush。
func writeSSE(w http.ResponseWriter, f http.Flusher, event string, data any) error {
	b, err := json.Marshal(data)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "event: %s\n", event); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "data: %s\n\n", b); err != nil {
		return err
	}
	f.Flush()
	return nil
}

// withCORS 包裹 mux，附加跨域头（便于前端独立部署调试）。
func (s *Server) withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// newSessionID 生成随机会话 ID。
func newSessionID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

// EnsureWebDir 确保前端目录存在且包含 index.html（用于启动时校验）。
func EnsureWebDir(dir string) error {
	info, err := os.Stat(dir)
	if err != nil {
		return fmt.Errorf("前端目录不存在: %s", dir)
	}
	if !info.IsDir() {
		return fmt.Errorf("前端路径不是目录: %s", dir)
	}
	// 校验 index.html 存在
	if _, err := os.Stat(filepath.Join(dir, "index.html")); err != nil {
		return fmt.Errorf("前端目录缺少 index.html: %s", dir)
	}
	return nil
}
