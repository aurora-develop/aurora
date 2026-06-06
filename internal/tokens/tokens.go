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

	if len(a.tokens) == 0 {
		return &Secret{}
	}

	secret := a.tokens[0]
	a.tokens = append(a.tokens[1:], secret)
	return secret
}

func (a *AccessToken) GetPaidSecret() *Secret {
	if a == nil {
		return &Secret{}
	}

	a.lock.Lock()
	defer a.lock.Unlock()

	for i, secret := range a.tokens {
		if secret == nil || secret.IsFree || secret.Token == "" {
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

func (a *AccessToken) GenerateTempToken(token string) *Secret {
	return &Secret{Token: token, PUID: "", IsFree: false}
}

func (a *AccessToken) GenerateDeviceId(token string) *Secret {
	return &Secret{Token: token, PUID: "", IsFree: true}
}
