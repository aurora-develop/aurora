package initialize

import (
	"bufio"
	"fmt"
	"os"
	"strconv"

	"aurora/internal/tokens"

	"github.com/google/uuid"
)

func readAccessToken() *tokens.AccessToken {
	var Secrets []*tokens.Secret
	// Read accounts.txt and create a list of accounts
	if _, err := os.Stat("access_tokens.txt"); err == nil {
		// Each line is a proxy, put in proxies array
		file, _ := os.Open("access_tokens.txt")
		defer file.Close()
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			// Split by :
			token := scanner.Text()
			if len(token) == 0 {
				continue
			}
			// Append to accounts
			Secrets = append(Secrets, tokens.NewSecret(token))
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

	if os.Getenv("FREE_ACCOUNTS") == "" || os.Getenv("FREE_ACCOUNTS") == "true" {
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
