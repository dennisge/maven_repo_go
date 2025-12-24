package config

import (
	"fmt"
	"os"
	"strings"
)

type Config struct {
	Username                string
	Password                string
	StoragePath             string
	Port                    string
	AccountsFile            string
	ProxyURLs               []string
	AnonymousAccess         bool
	SnapshotCleanupEnabled  bool
	SnapshotCleanupInterval string // Using string for duration parsing later or just "1h"
	SnapshotKeepDays        int
	SnapshotKeepLatestOnly  bool
	LogPath                 string
	LogKeepDays             int
}

func New() *Config {
	proxyEnv := getEnv("MAVEN_PROXY_URLS", "")
	var proxies []string
	if proxyEnv != "" {
		proxies = split(proxyEnv)
	}

	return &Config{
		Username:                getEnv("MAVEN_USERNAME", "admin"),
		Password:                getEnv("MAVEN_PASSWORD", "password"),
		StoragePath:             getEnv("MAVEN_STORAGE_PATH", "./artifacts"),
		Port:                    getEnv("MAVEN_PORT", "8080"),
		AccountsFile:            getEnv("MAVEN_ACCOUNTS_FILE", ""),
		ProxyURLs:               proxies,
		AnonymousAccess:         getEnv("MAVEN_ANONYMOUS_ACCESS", "false") == "true",
		SnapshotCleanupEnabled:  getEnv("MAVEN_SNAPSHOT_CLEANUP_ENABLED", "false") == "true",
		SnapshotCleanupInterval: getEnv("MAVEN_SNAPSHOT_CLEANUP_INTERVAL", "1h"),
		SnapshotKeepDays:        getEnvInt("MAVEN_SNAPSHOT_KEEP_DAYS", 30),
		SnapshotKeepLatestOnly:  getEnv("MAVEN_SNAPSHOT_KEEP_LATEST_ONLY", "false") == "true",
		LogPath:                 getEnv("MAVEN_LOG_PATH", "./server.log"),
		LogKeepDays:             getEnvInt("MAVEN_LOG_KEEP_DAYS", 7),
	}
}

func getEnvInt(key string, fallback int) int {
	if val, ok := os.LookupEnv(key); ok {
		var i int
		if _, err := fmt.Sscanf(val, "%d", &i); err == nil {
			return i
		}
	}
	return fallback
}

func split(s string) []string {
	var res []string
	for _, p := range strings.Split(s, ",") {
		if val := strings.TrimSpace(p); val != "" {
			res = append(res, val)
		}
	}
	return res
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}
