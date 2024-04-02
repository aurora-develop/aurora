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
	lock   sync.Mutex
}

func NewAccessToken(tokens map[string]Secret) AccessToken {
	return AccessToken{
		tokens: tokens,
	}
}

func (a *AccessToken) Set(name string, token string, puid string) {
	a.tokens[name] = Secret{Token: token, PUID: puid}
}

func (a *AccessToken) GetKeys() []string {
	keys := []string{}
	for k := range a.tokens {
		keys = append(keys, k)
	}
	return keys
}

func (a *AccessToken) Delete(name string) {
	delete(a.tokens, name)
}

func (a *AccessToken) Save() bool {
	file, err := os.OpenFile("access_tokens.json", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return false
	}
	defer file.Close()
	encoder := json.NewEncoder(file)
	err = encoder.Encode(a.tokens)
	return err == nil
}

func (a *AccessToken) GetSecret(account string) (string, string) {
	a.lock.Lock()
	defer a.lock.Unlock()

	if len(a.tokens) == 0 {
		return "", ""
	}
	secret := a.tokens[account]
	return secret.Token, secret.PUID
}
