package auth

import (
	"bufio"
	"log"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
)

// LoadAccounts reads lines in "username:password" format
func LoadAccounts(path string) (gin.Accounts, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	accounts := make(gin.Accounts)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			log.Printf("Skipping invalid line: %s\n", line)
			continue
		}
		accounts[parts[0]] = parts[1]
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return accounts, nil
}
