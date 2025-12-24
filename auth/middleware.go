package auth

import (
	"fmt"
	"maven_repo/config"
	"net/http"

	"github.com/gin-gonic/gin"
)

func BasicAuth(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Anonymous Access Check
		if cfg.AnonymousAccess {
			if c.Request.Method == http.MethodGet || c.Request.Method == http.MethodHead {
				// If no auth header provided, allow through
				if c.GetHeader("Authorization") == "" {
					c.Next()
					return
				}
			}
		}

		var accounts gin.Accounts
		var err error
		if cfg.AccountsFile != "" {
			accounts, err = LoadAccounts(cfg.AccountsFile)
			if err != nil {
				// Log error but maybe fail safe?
				// For now panic as it is configuration error
				panic(fmt.Sprintf("Failed to load accounts file: %v", err))
			}
		} else {
			accounts = gin.Accounts{
				cfg.Username: cfg.Password,
			}
		}

		// Use manual basic auth verification to avoid gin.BasicAuth which validates header
		// actually gin.BasicAuth is fine, we just didn't call it if anonymous.
		// If header IS provided, we want to verify it?
		// Yes, if user provides creds, we verify them.

		authHandler := gin.BasicAuth(accounts)
		authHandler(c)
	}
}
