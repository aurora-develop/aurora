package main

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"freechatgpt/internal/tokens"

	"github.com/xqdoo00o/OpenAIAuth/auth"
)

var accounts []Account

var validAccounts []string

const interval = time.Hour * 24 * 10

type Account struct {
	Email    string `json:"username"`
	Password string `json:"password"`
}

type TokenExp struct {
	Exp int64 `json:"exp"`
	Iat int64 `json:"iat"`
}

func getTokenExpire(tokenstring string) (time.Time, error) {
	payLoadData := strings.Split(tokenstring, ".")[1]
	// Decode payload
	payload, err := base64.RawStdEncoding.DecodeString(payLoadData)
	if err != nil {
		return time.Time{}, err
	}
	// Unmarshal payload
	var tokenExp TokenExp
	err = json.Unmarshal(payload, &tokenExp)
	if err != nil {
		return time.Time{}, err
	}
	return time.Unix(tokenExp.Exp, 0), nil
}

func AppendIfNone(slice []string, i string) []string {
	for _, ele := range slice {
		if ele == i {
			return slice
		}
	}
	return append(slice, i)
}

func newTimeFunc(account Account, token_list map[string]tokens.Secret, cron bool) func() {
	return func() {
		updateSingleToken(account, token_list, cron)
	}
}

func scheduleTokenPUID() {
	// Check if access_tokens.json exists
	if stat, err := os.Stat("access_tokens.json"); os.IsNotExist(err) {
		// Create the file
		file, err := os.Create("access_tokens.json")
		if err != nil {
			panic(err)
		}
		defer file.Close()
		updateToken()
	} else {
		file, err := os.Open("access_tokens.json")
		if err != nil {
			panic(err)
		}
		defer file.Close()
		decoder := json.NewDecoder(file)
		var token_list map[string]tokens.Secret
		err = decoder.Decode(&token_list)
		if err != nil {
			updateToken()
			return
		}
		if len(token_list) == 0 {
			updateToken()
		} else {
			ACCESS_TOKENS = tokens.NewAccessToken(token_list)
			validAccounts = []string{}
			for _, account := range accounts {
				token := token_list[account.Email].Token
				if token == "" {
					updateSingleToken(account, nil, true)
				} else {
					var toPUIDExpire time.Duration
					var puidTime time.Time
					var toExpire time.Duration
					if token_list[account.Email].PUID != "" {
						re := regexp.MustCompile(`\d{10,}`)
						puidIat := re.FindString(token_list[account.Email].PUID)
						if puidIat != "" {
							puidIatInt, _ := strconv.ParseInt(puidIat, 10, 64)
							puidTime = time.Unix(puidIatInt, 0)
							toPUIDExpire = interval - time.Since(puidTime)
							if toPUIDExpire < 0 {
								updateSingleToken(account, nil, false)
							}
						}
					}
				tokenProcess:
					token, _ = ACCESS_TOKENS.GetSecret(account.Email)
					expireTime, err := getTokenExpire(token)
					nowTime := time.Now()
					if err != nil {
						toExpire = interval - nowTime.Sub(stat.ModTime())
					} else {
						toExpire = expireTime.Sub(nowTime)
						if toExpire > 0 {
							toExpire = toExpire % interval
						}
					}
					if toPUIDExpire > 0 {
						toPUIDExpire = interval - nowTime.Sub(puidTime)
						if toExpire-toPUIDExpire > 2e9 {
							updateSingleToken(account, nil, false)
							toPUIDExpire = 0
							goto tokenProcess
						}
					}
					if toExpire > 0 {
						validAccounts = AppendIfNone(validAccounts, account.Email)
						f := newTimeFunc(account, nil, true)
						time.AfterFunc(toExpire+time.Second, f)
					} else {
						updateSingleToken(account, nil, true)
					}
				}
			}
		}
	}
}

func updateSingleToken(account Account, token_list map[string]tokens.Secret, cron bool) {
	if os.Getenv("CF_PROXY") != "" {
		// exec warp-cli disconnect and connect
		exec.Command("warp-cli", "disconnect").Run()
		exec.Command("warp-cli", "connect").Run()
		time.Sleep(5 * time.Second)
	}
	println("Updating access token for " + account.Email)
	var proxy_url string
	if len(proxies) == 0 {
		proxy_url = ""
	} else {
		proxy_url = proxies[0]
		// Push used proxy to the back of the list
		proxies = append(proxies[1:], proxies[0])
	}
	authenticator := auth.NewAuthenticator(account.Email, account.Password, proxy_url)
	err := authenticator.RenewWithCookies()
	if err != nil {
		authenticator.ResetCookies()
		err := authenticator.Begin()
		if err != nil {
			if token_list == nil {
				ACCESS_TOKENS.Delete(account.Email)
				for i, v := range validAccounts {
					if v == account.Email {
						validAccounts = append(validAccounts[:i], validAccounts[i+1:]...)
						break
					}
				}
			}
			println("Location: " + err.Location)
			println("Status code: " + strconv.Itoa(err.StatusCode))
			println("Details: " + err.Details)
			return
		}
	}
	access_token := authenticator.GetAccessToken()
	puid, _ := authenticator.GetPUID()
	if token_list != nil {
		token_list[account.Email] = tokens.Secret{Token: access_token, PUID: puid}
	} else {
		ACCESS_TOKENS.Set(account.Email, access_token, puid)
		ACCESS_TOKENS.Save()
	}
	validAccounts = AppendIfNone(validAccounts, account.Email)
	println("Success!")
	err = authenticator.SaveCookies()
	if err != nil {
		println(err.Details)
	}
	if cron {
		f := newTimeFunc(account, token_list, cron)
		time.AfterFunc(interval+time.Second, f)
	}
}

func updateToken() {
	token_list := map[string]tokens.Secret{}
	validAccounts = []string{}
	// Loop through each account
	for _, account := range accounts {
		updateSingleToken(account, token_list, false)
	}
	// Append access token to access_tokens.json
	ACCESS_TOKENS = tokens.NewAccessToken(token_list)
	ACCESS_TOKENS.Save()
	time.AfterFunc(interval, updateToken)
}
