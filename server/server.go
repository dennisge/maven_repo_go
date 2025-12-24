package server

import (
	"context"
	"log"
	"net/http"

	"maven_repo/auth"
	"maven_repo/config"
	"maven_repo/handler"
	"maven_repo/service"
	"maven_repo/storage"

	"github.com/gin-gonic/gin"
	"go.uber.org/fx"
)

func NewGinEngine(cfg *config.Config, h *handler.MavenHandler, admin *handler.AdminHandler) *gin.Engine {
	r := gin.Default()

	// Public repository (Aggregates all repos under repository/)
	mavenPublic := r.Group("/repository/maven-public", auth.BasicAuth(cfg))
	{
		mavenPublic.GET("/*path", h.HandleAggregateDownload("repository"))
		mavenPublic.HEAD("/*path", h.HandleAggregateHead("repository"))
	}

	// Dynamic repository (handles /repository/develop, /repository/staging, /repository/whatever)
	repos := r.Group("/repository/:repoName", auth.BasicAuth(cfg))
	{
		repos.PUT("/*path", h.HandleUpload)
		repos.GET("/*path", h.HandleDownload)
		repos.HEAD("/*path", h.HandleHead)
	}

	// Admin API for snapshots
	adminRoutes := r.Group("/admin/snapshots/cleanup", auth.BasicAuth(cfg))
	{
		adminRoutes.POST("/pause", admin.PauseCleanup)
		adminRoutes.POST("/resume", admin.ResumeCleanup)
		adminRoutes.GET("/status", admin.CleanupStatus)
		adminRoutes.POST("/trigger", admin.TriggerCleanup)
	}

	return r
}

func StartHTTPServer(lc fx.Lifecycle, cfg *config.Config, engine *gin.Engine) {
	srv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: engine,
	}

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			go func() {
				if err := engine.Run(":" + cfg.Port); err != nil {
					log.Printf("Listen: %s\n", err)
				}
			}()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			return srv.Shutdown(ctx)
		},
	})
}

func StartCleanupService(lc fx.Lifecycle, svc *service.SnapshotCleanupService) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			svc.Start()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			svc.Stop()
			return nil
		},
	})
}

var Module = fx.Options(
	fx.Provide(
		config.New,
		func(cfg *config.Config) storage.StorageProvider {
			return storage.NewLocalStorage(cfg.StoragePath)
		},
		func(store storage.StorageProvider, cfg *config.Config) *handler.MavenHandler {
			return handler.NewMavenHandler(store, cfg)
		},
		service.NewSnapshotCleanupService,
		handler.NewAdminHandler,
		NewGinEngine,
	),
	fx.Invoke(StartHTTPServer, StartCleanupService),
)
