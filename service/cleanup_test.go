package service

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"maven_repo/config"
	"maven_repo/storage"
)

func TestSnapshotCleanupService_RunCleanup(t *testing.T) {
	// Setup temporary storage
	if err := os.MkdirAll("tmp_storage", 0755); err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll("tmp_storage")

	store := storage.NewLocalStorage("tmp_storage")
	cfg := &config.Config{
		SnapshotCleanupEnabled:  true,
		SnapshotCleanupInterval: "1s",
		SnapshotKeepDays:        7,
		SnapshotKeepLatestOnly:  true,
	}

	svc := NewSnapshotCleanupService(store, cfg)

	// Create some dummy artifacts
	// 1. Snapshot directory
	dir := "com/example/app/1.0-SNAPSHOT"

	now := time.Now()

	// Create multiple versions of the same snapshot
	files := []struct {
		Name string
		Age  time.Duration
	}{
		{"app-1.0-20231020.120000-1.jar", 30 * 24 * time.Hour}, // Very old
		{"app-1.0-20250101.120000-2.jar", 10 * 24 * time.Hour}, // Older than 7 days
		{"app-1.0-20251219.120000-3.jar", 1 * 24 * time.Hour},  // Recent
		{"app-1.0-20251220.120000-4.jar", 0},                   // Most recent
		{"app-1.0-SNAPSHOT.pom", 1 * 24 * time.Hour},           // Pom
	}

	for _, f := range files {
		path := filepath.Join(dir, f.Name)
		if err := store.Save(path, strings.NewReader("dummy content")); err != nil {
			t.Fatal(err)
		}
		// Adjust mod time
		fullPath := filepath.Join("tmp_storage", path)
		if err := os.Chtimes(fullPath, now.Add(-f.Age), now.Add(-f.Age)); err != nil {
			t.Fatal(err)
		}
	}

	// Run cleanup
	if err := svc.RunCleanup(); err != nil {
		t.Fatal(err)
	}

	// Verify results
	entries, err := store.List(dir)
	if err != nil {
		t.Fatal(err)
	}

	// We expect:
	// - jar: Only the latest one (app-1.0-20251220.120000-4.jar) should remain because KeepLatestOnly=true
	// - pom: The newest pom (if multiple) or the current one if not deleted by age.
	// Actually, my implementation groups by extension.

	remainingJar := 0
	remainingPom := 0
	for _, e := range entries {
		if strings.HasSuffix(e.Name, ".jar") {
			remainingJar++
			if e.Name != "app-1.0-20251220.120000-4.jar" {
				t.Errorf("Unexpected jar remained: %s", e.Name)
			}
		}
		if strings.HasSuffix(e.Name, ".pom") {
			remainingPom++
		}
	}

	if remainingJar != 1 {
		t.Errorf("Expected 1 jar, got %d", remainingJar)
	}
}
