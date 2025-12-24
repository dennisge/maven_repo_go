package service

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"maven_repo/config"
	"maven_repo/storage"
	"regexp"
)

var (
	uniqueSnapshotRegex    = regexp.MustCompile(`^(.+)-(\d{8}\.\d{6}-\d+)(.*)$`)
	nonUniqueSnapshotRegex = regexp.MustCompile(`^(.+)-(SNAPSHOT)(.*)$`)
)

type SnapshotCleanupService struct {
	Store  storage.StorageProvider
	Config *config.Config
	Mu     sync.Mutex
	Paused bool
	Ctx    context.Context
	Cancel context.CancelFunc
}

func NewSnapshotCleanupService(store storage.StorageProvider, cfg *config.Config) *SnapshotCleanupService {
	ctx, cancel := context.WithCancel(context.Background())
	return &SnapshotCleanupService{
		Store:  store,
		Config: cfg,
		Ctx:    ctx,
		Cancel: cancel,
	}
}

func (s *SnapshotCleanupService) Start() {
	if !s.Config.SnapshotCleanupEnabled {
		log.Println("Snapshot cleanup task is disabled")
		return
	}

	interval, err := time.ParseDuration(s.Config.SnapshotCleanupInterval)
	if err != nil {
		log.Printf("Invalid snapshot cleanup interval: %v, using default 1h\n", err)
		interval = time.Hour
	}

	ticker := time.NewTicker(interval)
	go func() {
		defer ticker.Stop() // Ensure ticker is stopped when goroutine exits
		for {
			select {
			case <-ticker.C:
				s.Mu.Lock()
				paused := s.Paused
				s.Mu.Unlock()

				if !paused {
					log.Println("Starting snapshot cleanup...")
					if err := s.RunCleanup(); err != nil {
						log.Printf("Snapshot cleanup failed: %v\n", err)
					}
					log.Println("Snapshot cleanup finished.")
				}
			case <-s.Ctx.Done():
				ticker.Stop()
				return
			}
		}
	}()
}

func (s *SnapshotCleanupService) Stop() {
	s.Cancel()
}

func (s *SnapshotCleanupService) Pause() {
	s.Mu.Lock()
	defer s.Mu.Unlock()
	s.Paused = true
}

func (s *SnapshotCleanupService) Resume() {
	s.Mu.Lock()
	defer s.Mu.Unlock()
	s.Paused = false
}

func (s *SnapshotCleanupService) Status() string {
	s.Mu.Lock()
	defer s.Mu.Unlock()
	if s.Paused {
		return "paused"
	}
	return "running"
}

func (s *SnapshotCleanupService) RunCleanup() error {
	// Find all directories ending in -SNAPSHOT
	snapshotDirs := make(map[string]bool)
	err := s.Store.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Continue walk
		}
		if info.IsDir() && strings.HasSuffix(path, "-SNAPSHOT") {
			snapshotDirs[path] = true
		}
		return nil
	})
	if err != nil {
		return err
	}

	log.Printf("Found %d snapshot directories to check\n", len(snapshotDirs))
	for dir := range snapshotDirs {
		log.Printf("Cleaning up snapshot directory: %s\n", dir)
		if err := s.cleanupDir(dir); err != nil {
			log.Printf("Failed to cleanup directory %s: %v\n", dir, err)
		}
	}

	return nil
}

func (s *SnapshotCleanupService) cleanupDir(dir string) error {
	entries, err := s.Store.List(dir)
	if err != nil {
		return err
	}

	// Maven snapshots: artifactId-version-timestamp-buildnumber.ext
	// Or artifactId-version-SNAPSHOT.ext

	type fileInfo struct {
		Name    string
		ModTime time.Time
	}

	// Group by "artifact-version" part.
	// Since we are in a directory like "com/example/my-app/1.0-SNAPSHOT",
	// the files are likely "my-app-1.0-..."
	// We can group by extension to be safe, or by the part before the timestamp.
	// Group by "snapshot version" part.
	// For unique snapshots: artifactId-version-timestamp-buildNumber.ext
	// For non-unique: artifactId-version-SNAPSHOT.ext

	groups := make(map[string][]fileInfo)

	// regex to find version identifier like 20231027.123456-1 or SNAPSHOT
	// We look for the part between the last two hyphens if it matches a pattern,
	// or just the part before the first extension.
	// Actually, a simpler way: group by the part before the FIRST dot that is not part of the version.

	for _, e := range entries {
		if e.IsDir {
			continue
		}

		if strings.HasPrefix(e.Name, "maven-metadata") {
			continue
		}

		// Extract version identifier
		version := s.extractVersion(e.Name)
		groups[version] = append(groups[version], fileInfo{Name: e.Name, ModTime: e.ModTime})
	}

	now := time.Now()
	keepDays := time.Duration(s.Config.SnapshotKeepDays) * 24 * time.Hour

	log.Printf("Processing directory %s: %d snapshot versions found\n", dir, len(groups))

	// Create a list of versions to sort them by their latest file mod time
	type versionInfo struct {
		Name    string
		MaxTime time.Time
		Files   []fileInfo
	}
	var versions []versionInfo
	for name, files := range groups {
		maxTime := time.Time{}
		for _, f := range files {
			if f.ModTime.After(maxTime) {
				maxTime = f.ModTime
			}
		}
		versions = append(versions, versionInfo{Name: name, MaxTime: maxTime, Files: files})
	}

	// Sort versions descending (newest first)
	sort.Slice(versions, func(i, j int) bool {
		return versions[i].MaxTime.After(versions[j].MaxTime)
	})

	if s.Config.SnapshotKeepDays > 0 {
		log.Printf("  Retention policy: keep versions newer than %d days\n", s.Config.SnapshotKeepDays)
	}
	if s.Config.SnapshotKeepLatestOnly {
		log.Printf("  Retention policy: keep only the latest snapshot version\n")
	}

	for i, v := range versions {
		shouldDelete := false
		reason := ""

		// Check age (based on the newest file in this version)
		if s.Config.SnapshotKeepDays > 0 && now.Sub(v.MaxTime) > keepDays {
			shouldDelete = true
			reason = "expired"
		}

		// Check keep latest
		if s.Config.SnapshotKeepLatestOnly && i > 0 {
			shouldDelete = true
			if reason == "" {
				reason = "not latest"
			}
		}

		if shouldDelete {
			log.Printf("    Deleting snapshot version %s (Reason: %s, MaxAge: %v)\n", v.Name, reason, now.Sub(v.MaxTime))
			for _, f := range v.Files {
				relPath := filepath.Join(dir, f.Name)
				log.Printf("      Deleting file: %s\n", f.Name)
				if err := s.Store.Delete(relPath); err != nil {
					log.Printf("      Failed to delete %s: %v\n", relPath, err)
				}
			}
		} else {
			log.Printf("    Keeping snapshot version: %s (%d files)\n", v.Name, len(v.Files))
		}
	}

	return nil
}

func (s *SnapshotCleanupService) extractVersion(name string) string {
	// Try unique snapshot pattern first: artifactId-version-YYYYMMDD.HHMMSS-buildNumber
	if m := uniqueSnapshotRegex.FindStringSubmatch(name); m != nil {
		return m[1] + "-" + m[2]
	}

	// Try non-unique snapshot pattern: artifactId-version-SNAPSHOT
	if m := nonUniqueSnapshotRegex.FindStringSubmatch(name); m != nil {
		return m[1] + "-" + m[2]
	}

	// Fallback: strip extensions
	dotIdx := strings.Index(name, ".")
	if dotIdx != -1 {
		return name[:dotIdx]
	}

	return name
}
