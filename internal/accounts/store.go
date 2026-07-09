package accounts

import (
	"encoding/json"
	"os"
	"sync"
)

// Store 定义账号持久化接口
type Store interface {
	Load() ([]*Account, error)
	Save(accounts []*Account) error
}

// JSONStore 实现 JSON 文件持久化
type JSONStore struct {
	path string
	mu   sync.Mutex
}

func NewJSONStore(path string) *JSONStore {
	return &JSONStore{path: path}
}

func (s *JSONStore) Load() ([]*Account, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return []*Account{}, nil
		}
		return nil, err
	}

	var accounts []*Account
	if err := json.Unmarshal(data, &accounts); err != nil {
		return nil, err
	}
	if accounts == nil {
		accounts = []*Account{}
	}
	return accounts, nil
}

func (s *JSONStore) Save(accounts []*Account) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := json.MarshalIndent(accounts, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(s.path, data, 0644)
}
