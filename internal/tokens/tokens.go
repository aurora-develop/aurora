package tokens

import (
	"strings"
	"sync"
)

type Secret struct {
	Token      string `json:"token"`
	PUID       string `json:"puid"`
	TeamUserID string `json:"team_uid,omitempty"`
	IsFree     bool   `json:"isFree"`
	Disabled   bool   `json:"disabled,omitempty"`
}
type AccessToken struct {
	tokens []*Secret
	lock   sync.Mutex
}

func NewSecret(token string) *Secret {
	return &Secret{Token: token, PUID: "", IsFree: false}
}

func NewSecretWithTeam(token string, teamUserID string) *Secret {
	return &Secret{Token: token, PUID: "", TeamUserID: strings.TrimSpace(teamUserID), IsFree: false}
}

func NewSecretWithFree(token string) *Secret {
	return &Secret{Token: token, PUID: "", IsFree: true}
}

func (s *Secret) WithTeamUserID(teamUserID string) *Secret {
	if s == nil {
		return nil
	}
	teamUserID = strings.TrimSpace(teamUserID)
	cloned := *s
	cloned.TeamUserID = teamUserID
	return &cloned
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

	a.lock.Lock()
	defer a.lock.Unlock()

	if len(a.tokens) == 0 {
		return &Secret{}
	}

	for i, secret := range a.tokens {
		if !secret.Disabled {
			a.tokens = append(a.tokens[:i], a.tokens[i+1:]...)
			a.tokens = append(a.tokens, secret)
			return secret
		}
	}
	return &Secret{}
}

func (a *AccessToken) GetPaidSecret() *Secret {
	if a == nil {
		return &Secret{}
	}

	a.lock.Lock()
	defer a.lock.Unlock()

	for i, secret := range a.tokens {
		if secret == nil || secret.IsFree || secret.Token == "" || secret.Disabled {
			continue
		}
		a.tokens = append(a.tokens[:i], a.tokens[i+1:]...)
		a.tokens = append(a.tokens, secret)
		return secret
	}
	return &Secret{}
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

// DisableSecret 标记指定 token 为禁用，轮询时自动跳过
// 返回 true 表示找到并禁用了该 token，false 表示 token 不在池中
func (a *AccessToken) DisableSecret(token string) bool {
	if a == nil || token == "" {
		return false
	}
	a.lock.Lock()
	defer a.lock.Unlock()
	for _, secret := range a.tokens {
		if secret.Token == token {
			secret.Disabled = true
			return true
		}
	}
	return false
}

func (a *AccessToken) GenerateTempToken(token string) *Secret {
	return &Secret{Token: token, PUID: "", IsFree: false}
}

func (a *AccessToken) GenerateDeviceId(token string) *Secret {
	return &Secret{Token: token, PUID: "", IsFree: true}
}
