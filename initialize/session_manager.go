package initialize

import (
	"aurora/internal/chatgpt"
	"log"
	"sync"
	"time"
)

// SessionManager 按 conversationID 缓存 ChatClientState，
// 使得同一对话的多轮请求复用相同的 DeviceID / SessionID，
// 与 chatgpttoapi 的 session 行为对齐。
type SessionManager struct {
	mu       sync.RWMutex
	sessions map[string]*sessionEntry
	ttl      time.Duration
}

type sessionEntry struct {
	state    *chatgpt.ChatClientState
	lastUsed time.Time
}

const defaultSessionTTL = 30 * time.Minute

func NewSessionManager() *SessionManager {
	sm := &SessionManager{
		sessions: make(map[string]*sessionEntry),
		ttl:      defaultSessionTTL,
	}
	go sm.cleanupLoop()
	return sm
}

// GetOrCreate 根据 conversationID 获取已有状态，不存在则返回 nil。
// 调用方在拿到 conversationID 后调用 Register 注册。
func (sm *SessionManager) Get(conversationID string) *chatgpt.ChatClientState {
	if conversationID == "" {
		return nil
	}
	sm.mu.RLock()
	entry, ok := sm.sessions[conversationID]
	sm.mu.RUnlock()
	if !ok {
		return nil
	}
	sm.mu.Lock()
	entry.lastUsed = time.Now()
	sm.mu.Unlock()
	return entry.state
}

// Register 将 ChatClientState 注册到 conversationID 下。
func (sm *SessionManager) Register(conversationID string, state *chatgpt.ChatClientState) {
	if conversationID == "" || state == nil {
		return
	}
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.sessions[conversationID] = &sessionEntry{
		state:    state,
		lastUsed: time.Now(),
	}
}

func (sm *SessionManager) cleanupLoop() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		sm.mu.Lock()
		now := time.Now()
		removed := 0
		for convID, entry := range sm.sessions {
			if now.Sub(entry.lastUsed) > sm.ttl {
				delete(sm.sessions, convID)
				removed++
			}
		}
		if removed > 0 {
			log.Printf("[session] 清理过期 session %d 个，当前活跃 %d 个", removed, len(sm.sessions))
		}
		sm.mu.Unlock()
	}
}
