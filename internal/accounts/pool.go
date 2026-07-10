package accounts

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
)

var ErrNoAvailable = errors.New("no available account of the requested type")

// Pool 账号池管理，按类型分三个数组，Acquire 直接取无需遍历
// 临时账号存放在单独的 map[string]*Account 中,以 token hash 为 key
type Pool struct {
	mu       sync.Mutex
	noauth   []*Account
	free     []*Account
	puid     []*Account
	cursors  [3]int // 0=noauth,1=free,2=puid

	// 临时账号 (外部传入的 accessToken 创建的)
	tempMu    sync.RWMutex
	temporary map[string]*Account // key = tokenHash
}

// typeIndex 返回 AccountType 在 cursors 中的索引
func typeIndex(t AccountType) int {
	switch t {
	case TypeNoAuth:
		return 0
	case TypeFree:
		return 1
	case TypePUID:
		return 2
	}
	return -1
}

func (p *Pool) sliceFor(t AccountType) *[]*Account {
	switch t {
	case TypeNoAuth:
		return &p.noauth
	case TypeFree:
		return &p.free
	case TypePUID:
		return &p.puid
	}
	return nil
}

func NewPool(initial []*Account) *Pool {
	pool := &Pool{
		temporary: make(map[string]*Account),
	}
	for _, acct := range initial {
		pool.AddAccount(acct)
	}
	return pool
}

// AddAccount 按类型添加到对应数组
func (p *Pool) AddAccount(acct *Account) {
	if acct == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	switch acct.Type {
	case TypeNoAuth:
		p.noauth = append(p.noauth, acct)
	case TypeFree:
		p.free = append(p.free, acct)
	case TypePUID:
		p.puid = append(p.puid, acct)
	}
}

// Acquire 从对应类型数组中轮询获取一个可用账号
func (p *Pool) Acquire(acctType AccountType) (*Account, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	slice := p.sliceFor(acctType)
	if slice == nil || len(*slice) == 0 {
		return nil, ErrNoAvailable
	}

	idx := typeIndex(acctType)
	if idx < 0 {
		return nil, ErrNoAvailable
	}

	entries := *slice
	for i := 0; i < len(entries); i++ {
		cur := (p.cursors[idx] + i) % len(entries)
		if entries[cur].Status == StatusActive {
			p.cursors[idx] = (cur + 1) % len(entries)
			entries[cur].TotalCalls++
			return entries[cur], nil
		}
	}

	return nil, ErrNoAvailable
}

// Release 保留接口但不再需要主动调用（统计在 Acquire 时已更新）。
// 保留以防将来改为独占模式需要。
func (p *Pool) Release(acct *Account, result error) {
}

// ReportFailure 标记账号为过期，Acquire 时自动跳过，后续健康检查会尝试续期
func (p *Pool) ReportFailure(acct *Account) bool {
	if acct == nil {
		return false
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	acct.Status = StatusExpired
	acct.FailedCalls++
	return true
}

// ExpiredAccounts 返回所有过期账号（用于健康检查）
func (p *Pool) ExpiredAccounts() []*Account {
	p.mu.Lock()
	defer p.mu.Unlock()
	var out []*Account
	for _, list := range [][]*Account{p.noauth, p.free, p.puid} {
		for _, a := range list {
			if a.Status == StatusExpired {
				out = append(out, a)
			}
		}
	}
	return out
}

// TokenRenewer 续期回调，由 bootstrap 提供
type TokenRenewer func(acct *Account) bool

// StartHealthCheck 启动健康检查 goroutine
func (p *Pool) StartHealthCheck(interval time.Duration, renew TokenRenewer) func() {
	if interval <= 0 {
		interval = 5 * time.Minute
	}
	stop := make(chan struct{})
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				p.runHealthCheck(renew)
			case <-stop:
				return
			}
		}
	}()
	return func() { close(stop) }
}

func (p *Pool) runHealthCheck(renew TokenRenewer) {
	for _, acct := range p.ExpiredAccounts() {
		if acct.Status == StatusExpired && renew != nil {
			if renew(acct) {
				p.mu.Lock()
				acct.Status = StatusActive
				p.mu.Unlock()
				log.Printf("[health] account %s renewed successfully", acct.ID[:8])
			}
		}
	}
}

// ── 临时账号管理 (外部传入的 accessToken) ─────────────────────────

// GetOrCreateTempAccount 获取或创建外部 token 对应的临时账号。
// 命中已有临时账号:刷新 LastUsed;未命中:用 fingerprint 创建新的,绑代理、随机 UA、InitClient。
// 池里池外都查(为了去重)。
func (p *Pool) GetOrCreateTempAccount(token, userAgent string, proxyURL string) *Account {
	tokenHash := tokenHashOf(token)

	// 1) 查已有 (命中的同时刷新 LastUsed)
	p.tempMu.RLock()
	if existing, ok := p.temporary[tokenHash]; ok {
		p.tempMu.RUnlock()
		existing.LastUsed = time.Now()
		return existing
	}
	p.tempMu.RUnlock()

	// 2) 没命中,创建一个
	fp := BrowserFingerprint{
		OaiDeviceID:  uuid.NewString(),
		OaiSessionID: uuid.NewString(),
		UserAgent:    userAgent,
		ScreenWidth:         1920,
		ScreenHeight:        1080,
		HardwareConcurrency: 8,
		Platform:            "Win32",
		TLSProfileName:      "chrome_146",
	}
	// 如果请求没传 User-Agent,用默认
	if fp.UserAgent == "" {
		fp.UserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/148.0.0.0 Safari/537.36"
	}
	// 解析 JWT 拿 chatgpt_account_id
	chatGPTID := ExtractChatGPTAccountID(token)

	acct := NewAccount(uuid.NewString(), TypeFree, token)
	acct.ChatGPTAccountID = chatGPTID
	acct.Fingerprint = fp
	acct.Proxy = proxyURL
	acct.Status = StatusActive
	acct.LastUsed = time.Now()
	acct.IsTemporary = true
	// 初始化专属 TLS client
	_ = acct.InitClient()

	// 3) 放入 map (可能有并发竞争,后写覆盖先写,反正同一个 token 效果一样)
	p.tempMu.Lock()
	if existing, ok := p.temporary[tokenHash]; ok {
		p.tempMu.Unlock()
		existing.LastUsed = time.Now()
		return existing
	}
	p.temporary[tokenHash] = acct
	p.tempMu.Unlock()
	return acct
}

// TouchAccount 刷新临时账号的 LastUsed,让 GC 知道它在用。
func (p *Pool) TouchAccount(account *Account) {
	if account == nil {
		return
	}
	account.LastUsed = time.Now()
}

// StartTempAccountGC 启动后台 GC,清理 idle 超过 idleTimeout 的临时账号。
// 返回 stop 函数。
func (p *Pool) StartTempAccountGC(idleTimeout time.Duration, interval time.Duration) func() {
	if idleTimeout <= 0 {
		idleTimeout = 10 * time.Minute
	}
	if interval <= 0 {
		interval = 1 * time.Minute
	}
	stop := make(chan struct{})
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				p.evictIdleTempAccounts(idleTimeout)
			case <-stop:
				return
			}
		}
	}()
	return func() { close(stop) }
}

func (p *Pool) evictIdleTempAccounts(idleTimeout time.Duration) {
	now := time.Now()
	p.tempMu.Lock()
	defer p.tempMu.Unlock()
	for hash, acct := range p.temporary {
		if now.Sub(acct.LastUsed) > idleTimeout {
			delete(p.temporary, hash)
		}
	}
}

// TokenHashOf 暴露给外部包 (handler) 使用
func TokenHashOf(token string) string {
	return tokenHashOf(token)
}

func tokenHashOf(token string) string {
	h := sha256.Sum256([]byte(token))
	return fmt.Sprintf("%x", h[:16])
}