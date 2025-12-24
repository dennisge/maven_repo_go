package logger

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"maven_repo/config"

	"go.uber.org/fx"
)

type LogManager struct {
	Cfg *config.Config
}

func NewLogManager(cfg *config.Config) *LogManager {
	return &LogManager{Cfg: cfg}
}

func (l *LogManager) Setup() {
	if l.Cfg.LogPath == "" {
		return
	}

	// Ensure directory exists
	dir := filepath.Dir(l.Cfg.LogPath)
	if dir != "." {
		os.MkdirAll(dir, 0755)
	}

	// Open log file in append mode
	file, err := os.OpenFile(l.Cfg.LogPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Printf("Failed to open log file %s: %v\n", l.Cfg.LogPath, err)
		return
	}

	// Set multi-writer for standard logger (file + stdout)
	mw := io.MultiWriter(os.Stdout, file)
	log.SetOutput(mw)

	log.Printf("Logging initialized to %s (Retention: %d days)\n", l.Cfg.LogPath, l.Cfg.LogKeepDays)
}

func (l *LogManager) Start(lc fx.Lifecycle) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			go l.runCleanupLoop()
			return nil
		},
	})
}

func (l *LogManager) runCleanupLoop() {
	// Initial cleanup
	l.cleanupOldLogs()

	// Initial rollout check (if file was from previous day)
	l.checkAndRollout()

	ticker := time.NewTicker(1 * time.Hour) // Check more frequently for rotation
	for range ticker.C {
		l.cleanupOldLogs()
		l.checkAndRollout()
	}
}

func (l *LogManager) checkAndRollout() {
	if l.Cfg.LogPath == "" {
		return
	}

	info, err := os.Stat(l.Cfg.LogPath)
	if err != nil {
		if os.IsNotExist(err) {
			return
		}
		log.Printf("Failed to stat log file for rollout check: %v\n", err)
		return
	}

	// If the file was last modified on a different day than now, rollout.
	// We use the modification time of the current log file as the indicator.
	// If it's empty, we don't rollout.
	if info.Size() == 0 {
		return
	}

	now := time.Now()
	// Use UTC for consistent day comparison if needed, but local is fine for "daily" rotation
	modYear, modMonth, modDay := info.ModTime().Date()
	nowYear, nowMonth, nowDay := now.Date()

	if modYear != nowYear || modMonth != nowMonth || modDay != nowDay {
		l.rollout(info.ModTime())
	}
}

func (l *LogManager) rollout(t time.Time) {
	oldPath := l.Cfg.LogPath
	ext := filepath.Ext(oldPath)
	prefix := strings.TrimSuffix(oldPath, ext)
	// Create name like server.2023-10-27.log
	newPath := fmt.Sprintf("%s.%s%s", prefix, t.Format("2006-01-02"), ext)

	log.Printf("Starting log rollout (rotation) for %s...\n", oldPath)

	// Since we are using standard log.SetOutput, we don't have an easy way to close the file.
	// On Unix, we can rename an open file.
	err := os.Rename(oldPath, newPath)
	if err != nil {
		log.Printf("Log rollout FAILED: rename %s to %s error: %v\n", oldPath, newPath, err)
		return
	}

	// Re-initialize logging to create a new file and set it as output
	l.Setup()

	log.Printf("Log rollout completed: %s -> %s\n", oldPath, newPath)
}

func (l *LogManager) cleanupOldLogs() {
	if l.Cfg.LogKeepDays <= 0 {
		return
	}

	log.Println("Checking for old log files to clean up...")

	dir := filepath.Dir(l.Cfg.LogPath)
	baseName := filepath.Base(l.Cfg.LogPath)
	ext := filepath.Ext(baseName)
	prefix := strings.TrimSuffix(baseName, ext)

	entries, err := os.ReadDir(dir)
	if err != nil {
		log.Printf("Failed to read log directory: %v\n", err)
		return
	}

	now := time.Now()
	retention := time.Duration(l.Cfg.LogKeepDays) * 24 * time.Hour

	for _, e := range entries {
		if e.IsDir() {
			continue
		}

		// Check if it's a log file (either the main one or a rotated one)
		// Usually rotated logs might have a date suffix like server.log.2023-10-27
		if !strings.HasPrefix(e.Name(), prefix) {
			continue
		}

		info, err := e.Info()
		if err != nil {
			continue
		}

		if now.Sub(info.ModTime()) > retention {
			fullPath := filepath.Join(dir, e.Name())
			log.Printf("Deleting old log file: %s (Age: %v)\n", fullPath, now.Sub(info.ModTime()))
			if err := os.Remove(fullPath); err != nil {
				log.Printf("Failed to delete log file %s: %v\n", fullPath, err)
			}
		}
	}
}

var Module = fx.Options(
	fx.Provide(NewLogManager),
	fx.Invoke(func(l *LogManager, lc fx.Lifecycle) {
		l.Setup()
		l.Start(lc)
	}),
)
