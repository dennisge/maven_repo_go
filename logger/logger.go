package logger

import (
	"io"
	"log"
	"os"

	"maven_repo/config"

	"go.uber.org/fx"
	"gopkg.in/natefinch/lumberjack.v2"
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

	lj := &lumberjack.Logger{
		Filename:   l.Cfg.LogPath,
		MaxSize:    l.Cfg.LogMaxSize, // megabytes
		MaxBackups: l.Cfg.LogMaxBackups,
		MaxAge:     l.Cfg.LogKeepDays, // days
		Compress:   true,              // disabled by default
		LocalTime:  true,
	}

	// Set multi-writer for standard logger (file + stdout)
	mw := io.MultiWriter(os.Stdout, lj)
	log.SetOutput(mw)

	log.Printf("Logging initialized to %s (MaxSize: %dMB, Keep: %d days, MaxBackups: %d)\n",
		l.Cfg.LogPath, l.Cfg.LogMaxSize, l.Cfg.LogKeepDays, l.Cfg.LogMaxBackups)
}

func (l *LogManager) Start(lc fx.Lifecycle) {
	// No background loop needed, lumberjack handles it on Write()
}

var Module = fx.Options(
	fx.Provide(NewLogManager),
	fx.Invoke(func(l *LogManager, lc fx.Lifecycle) {
		l.Setup()
		l.Start(lc)
	}),
)
