package tokens

import (
	"encoding/json"
	"os"
	"sync"
)

type Secret struct {
	Token string `json:"token"`
	PUID  string `json:"puid"`
}

type AccessToken struct {
	tokens map[string]Secret
	cache  map[string]Secret // 添加一个用于缓存的字段
	lock   sync.Mutex
}

func NewAccessToken(tokens map[string]Secret) AccessToken {
	cache := make(map[string]Secret) // 初始化缓存
	for k, v := range tokens {       // 在启动时加载现有的tokens到缓存中
		cache[k] = v
	}
	return AccessToken{
		tokens: tokens,
		cache:  cache,
	}
}

func (a *AccessToken) Set(name string, token string, puid string) {
	a.lock.Lock()
	defer a.lock.Unlock()

	secret := Secret{Token: token, PUID: puid}
	a.tokens[name] = secret // 更新内存中的tokens
	a.cache[name] = secret  // 同时更新缓存
}

func (a *AccessToken) GetKeys() []string {
	keys := []string{}
	a.lock.Lock()
	defer a.lock.Unlock()

	for k := range a.cache { // 使用缓存
		keys = append(keys, k)
	}
	return keys
}

func (a *AccessToken) Delete(name string) {
	a.lock.Lock()
	defer a.lock.Unlock()

	delete(a.tokens, name) // 从内存中删除
	delete(a.cache, name)  // 同时从缓存中删除
}

func (a *AccessToken) Save() bool {
	a.lock.Lock()
	defer a.lock.Unlock()

	file, err := os.OpenFile("access_tokens.json", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return false
	}
	defer file.Close()
	encoder := json.NewEncoder(file)
	err = encoder.Encode(a.tokens) // 保存tokens到文件
	return err == nil
}

func (a *AccessToken) GetSecret(account string) (string, string) {
	a.lock.Lock()
	defer a.lock.Unlock()

	if secret, exists := a.cache[account]; exists { // 从缓存中获取
		return secret.Token, secret.PUID
	}
	return "", ""
}
