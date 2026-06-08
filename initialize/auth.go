package initialize

import (
	"aurora/internal/tokens"
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/google/uuid"
)

func parseAccessTokenLine(line string) *tokens.Secret {
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "#") {
		return nil
	}
	parts := strings.SplitN(line, ":", 2)
	token := strings.TrimSpace(parts[0])
	if token == "" {
		return nil
	}
	return tokens.NewSecret(token)
}

func readAccessToken() *tokens.AccessToken {
	var Secrets []*tokens.Secret
	// Read accounts.txt and create a list of accounts
	if _, err := os.Stat("access_tokens.txt"); err == nil {
		// Each line is a proxy, put in proxies array
		file, _ := os.Open("access_tokens.txt")
		defer file.Close()
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			if secret := parseAccessTokenLine(scanner.Text()); secret != nil {
				Secrets = append(Secrets, secret)
			}
		}
	}

	// 增加自定义free_tokens.txt，支持文件设置每个账号的uuid
	if _, err := os.Stat("free_tokens.txt"); err == nil {
		// Each line is a proxy, put in proxies array
		file, _ := os.Open("free_tokens.txt")
		defer file.Close()
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			// Split by :
			token := scanner.Text()
			if len(token) == 0 {
				continue
			}
			// Append to accounts
			Secrets = append(Secrets, tokens.NewSecretWithFree(token))
		}
	}

	if os.Getenv("FREE_ACCOUNTS") == "true" {
		freeAccountsNumStr := os.Getenv("FREE_ACCOUNTS_NUM")
		numAccounts := 1024
		if freeAccountsNumStr != "" {
			if freeAccountsNum, err := strconv.Atoi(freeAccountsNumStr); err == nil && freeAccountsNum > 0 {
				numAccounts = freeAccountsNum
			} else {
				fmt.Println("Invalid FREE_ACCOUNTS_NUM:", err)
			}
		}
		for i := 0; i < numAccounts; i++ {
			uid := uuid.NewString()
			Secrets = append(Secrets, tokens.NewSecretWithFree(uid))
		}
	}

	token := tokens.NewAccessToken(Secrets)
	return &token
}
