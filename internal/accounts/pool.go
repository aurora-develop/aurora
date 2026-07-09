package accounts

import (
	"errors"
	"log"
	"sync"
	"time"
)

var ErrNoAvailable = errors.New("no available account of the requested type")

// Pool 账号池管理，按类型分三个数组，Acquire 直接取无需遍历
type Pool struct {
	mu       sync.Mutex
	noauth   []*Account
	free     []*Account
	puid     []*Account
	cursors  [3]int // 0=noauth,1=free,2=puid
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
	pool := &Pool{}
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
