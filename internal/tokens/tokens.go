package tokens

import (
	"sync"
)

type Secret struct {
	Token  string `json:"token"`
	PUID   string `json:"puid"`
	IsFree bool   `json:"isFree"`
}
type AccessToken struct {
	tokens []*Secret
	lock   sync.Mutex
}

func NewSecret(token string) *Secret {
	return &Secret{Token: token, PUID: "", IsFree: false}
}

func NewSecretWithFree(token string) *Secret {
	return &Secret{Token: token, PUID: "", IsFree: true}
}

func NewAccessToken(tokens []*Secret) AccessToken {
	return AccessToken{
		tokens: tokens,
	}
}

func (a *AccessToken) GetSecret() *Secret {
	if a == nil {
		return &Secret{}
	}

	if len(a.tokens) == 0 {
		return &Secret{}
	}

	secret := a.tokens[0]
	a.tokens = append(a.tokens[1:], secret)
	return secret
}

// UpdateSecret 更新tokens
func (a *AccessToken) UpdateSecret(tokens []*Secret) {
	a.lock.Lock()
	defer a.lock.Unlock()
	if len(tokens) == 0 {
		return
	}
	a.tokens = tokens
}

func (a *AccessToken) GenerateTempToken(token string) *Secret {
	return &Secret{Token: token, PUID: "", IsFree: false}
}

func (a *AccessToken) GenerateDeviceId(token string) *Secret {
	return &Secret{Token: token, PUID: "", IsFree: true}
}
