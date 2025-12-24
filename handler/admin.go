package handler

import (
	"net/http"

	"maven_repo/service"

	"github.com/gin-gonic/gin"
)

type AdminHandler struct {
	CleanupService *service.SnapshotCleanupService
}

func NewAdminHandler(cleanupService *service.SnapshotCleanupService) *AdminHandler {
	return &AdminHandler{
		CleanupService: cleanupService,
	}
}

func (h *AdminHandler) PauseCleanup(c *gin.Context) {
	h.CleanupService.Pause()
	c.JSON(http.StatusOK, gin.H{"status": "paused"})
}

func (h *AdminHandler) ResumeCleanup(c *gin.Context) {
	h.CleanupService.Resume()
	c.JSON(http.StatusOK, gin.H{"status": "running"})
}

func (h *AdminHandler) CleanupStatus(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": h.CleanupService.Status()})
}

func (h *AdminHandler) TriggerCleanup(c *gin.Context) {
	go func() {
		h.CleanupService.RunCleanup()
	}()
	c.JSON(http.StatusOK, gin.H{"message": "Cleanup triggered manually"})
}
