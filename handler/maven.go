package handler

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	"maven_repo/config"
	"maven_repo/storage"

	"github.com/gin-gonic/gin"
)

type MavenHandler struct {
	Store  storage.StorageProvider
	Config *config.Config
	Client *http.Client
}

func NewMavenHandler(store storage.StorageProvider, cfg *config.Config) *MavenHandler {
	return &MavenHandler{
		Store:  store,
		Config: cfg,
		Client: &http.Client{},
	}
}

func (h *MavenHandler) HandleDownload(c *gin.Context) {
	path := strings.TrimPrefix(c.Request.URL.Path, "/")

	// Check if this is a directory listing request
	// We optimize by checking List first or Head?
	// If path ends in /, definitely directory?
	// Maven paths usually file.
	// Let's assume file first, then directory if file fails?
	// Or check List.

	// Try to list first. If it returns entries, it's a directory.
	entries, err := h.Store.List(path)
	if err == nil && entries != nil {
		// It is a directory, verify it's not empty or just treat as dir
		// Render HTML
		c.Header("Content-Type", "text/html")
		c.Writer.WriteHeader(http.StatusOK)
		fmt.Fprintf(c.Writer, "<html><body><h1>Index of /%s</h1><hr><ul>", path)
		fmt.Fprintf(c.Writer, "<li><a href=\"../\">../</a></li>")
		for _, e := range entries {
			slash := ""
			if e.IsDir {
				slash = "/"
			}
			fmt.Fprintf(c.Writer, "<li><a href=\"%s%s\">%s%s</a> (Size: %d)</li>", e.Name, slash, e.Name, slash, e.Size)
		}
		fmt.Fprintf(c.Writer, "</ul><hr></body></html>")
		return
	}

	// If not directory, try file
	reader, found, err := h.Store.Get(path)
	if err == nil && found {
		defer reader.Close()
		c.DataFromReader(http.StatusOK, -1, "application/octet-stream", reader, nil)
		return
	}

	// Not found locally, try proxy
	if len(h.Config.ProxyURLs) > 0 {
		// We need to strip the local repository path prefix (e.g. repository/develop/)
		// to get the actual artifact path (com/...)
		// The path variable currently is "repository/develop/com/..." or "public/com/..."
		// We assume the upstream is a root maven repo.
		// We can try to split by "/" and ignore first 2 parts if it starts with "repository/"?
		// Or ignore first 1 part if "public/"?
		// A cleaner way relies on us knowing the structure.
		// Let's assume standard layout:
		// If path starts with "repository/", skip 2 segments.
		// If path starts with "public/", skip 1 segment.

		parts := strings.Split(path, "/")
		var artifactPath string
		if strings.HasPrefix(path, "repository/") && len(parts) > 2 {
			artifactPath = strings.Join(parts[2:], "/")
		} else if strings.HasPrefix(path, "public/") && len(parts) > 1 {
			artifactPath = strings.Join(parts[1:], "/")
		} else {
			// fallback, maybe it is direct?
			artifactPath = path
		}

		for _, proxy := range h.Config.ProxyURLs {
			url := strings.TrimRight(proxy, "/") + "/" + artifactPath
			resp, err := h.Client.Get(url)
			if err == nil && resp.StatusCode == http.StatusOK {
				contentType := resp.Header.Get("Content-Type")
				if strings.HasPrefix(contentType, "text/html") {
					resp.Body.Close()
					// If upstream returns HTML (directory listing), we don't want to cache it as a file.
					// We treat this as not found (or effectively not a valid artifact download).
					continue
				}

				defer resp.Body.Close()

				// Cache it
				// We need to stream to both Response and Storage
				// io.TeeReader writes to a writer mainly...
				// But TeeReader(r, w) returns a Reader. When we read from it, it writes to w.
				// But Storage.Save takes a Reader.
				// We can use Pipe.
				pr, pw := io.Pipe()

				go func() {
					if err := h.Store.Save(path, pr); err != nil {
						// log error?
					}
				}()

				// Tee: Write to Pipe (which goes to storage) AND return reader for response
				// wait, Save consumes reader fully.
				// We accept request body -> Save.
				// Here we have Response Body -> Save AND Response Writer.
				// Gin DataFromReader consumes reader.
				// So we want: Resp.Body -> Tee(PipeWriter) -> Gin Response
				// and PipeReader -> Save.

				tee := io.TeeReader(resp.Body, pw)

				// We need to ensure Pipe is closed when reading finishes
				// But DataFromReader will read until EOF.
				// After EOF, we close PW.

				// Actually, Save runs in goroutine reading from PipeReader.
				// We read from TeeReader (which is Resp body + write to PipeWriter).
				// When TeeReader is done, we close PipeWriter.

				// Wrap tee to close pw on EOF
				wrappedReader := &NotifyReader{Reader: tee, OnEOF: func() { pw.Close() }}

				c.DataFromReader(http.StatusOK, resp.ContentLength, contentType, wrappedReader, nil)
				return
			}
		}
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNotFound)
}

// NotifyReader closes pipe on completion
type NotifyReader struct {
	io.Reader
	OnEOF func()
}

func (n *NotifyReader) Read(p []byte) (int, error) {
	read, err := n.Reader.Read(p)
	if err == io.EOF {
		n.OnEOF()
	}
	return read, err
}

func (h *MavenHandler) HandleHead(c *gin.Context) {
	path := strings.TrimPrefix(c.Request.URL.Path, "/")
	found, err := h.Store.Head(path)
	if err == nil && found {
		c.Status(http.StatusOK)
		return
	}

	// Try proxy
	if len(h.Config.ProxyURLs) > 0 {
		for _, proxy := range h.Config.ProxyURLs {
			url := strings.TrimRight(proxy, "/") + "/" + path
			resp, err := h.Client.Head(url)
			if err == nil && resp.StatusCode == http.StatusOK {
				c.Status(http.StatusOK)
				return
			}
		}
	}

	if err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}
	c.Status(http.StatusNotFound)
}

func (h *MavenHandler) HandleUpload(c *gin.Context) {
	path := strings.TrimPrefix(c.Request.URL.Path, "/")

	// Ensure body is closed
	defer c.Request.Body.Close()

	if err := h.Store.Save(path, c.Request.Body); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to save artifact: %v", err)})
		return
	}

	c.Status(http.StatusCreated)
}

func (h *MavenHandler) HandleAggregateDownload(basePath string) gin.HandlerFunc {
	return func(c *gin.Context) {
		artifactPath := strings.TrimPrefix(c.Param("path"), "/")

		// Discover repos in the base path (e.g., repository/)
		repos := h.getAggregateRepos(basePath)

		// 1. Try to list (directory) first across all repos
		var allEntries []storage.Entry
		foundDir := false
		for _, repo := range repos {
			fullPath := strings.TrimRight(repo, "/") + "/" + artifactPath
			entries, err := h.Store.List(fullPath)
			if err == nil && entries != nil {
				foundDir = true
				allEntries = append(allEntries, entries...)
			}
		}

		if foundDir {
			// Deduplicate and render
			c.Header("Content-Type", "text/html")
			c.Writer.WriteHeader(http.StatusOK)
			fmt.Fprintf(c.Writer, "<html><body><h1>Index of /repository/maven-public/%s (Aggregated)</h1><hr><ul>", artifactPath)
			fmt.Fprintf(c.Writer, "<li><a href=\"../\">../</a></li>")
			seen := make(map[string]bool)
			for _, e := range allEntries {
				if seen[e.Name] {
					continue
				}
				seen[e.Name] = true
				slash := ""
				if e.IsDir {
					slash = "/"
				}
				fmt.Fprintf(c.Writer, "<li><a href=\"%s%s\">%s%s</a> (Size: %d)</li>", e.Name, slash, e.Name, slash, e.Size)
			}
			fmt.Fprintf(c.Writer, "</ul><hr></body></html>")
			return
		}

		// 2. Try to get file across all repos
		for _, repo := range repos {
			fullPath := strings.TrimRight(repo, "/") + "/" + artifactPath
			reader, found, err := h.Store.Get(fullPath)
			if err == nil && found {
				defer reader.Close()
				c.DataFromReader(http.StatusOK, -1, "application/octet-stream", reader, nil)
				return
			}
		}

		// 3. Not found locally, try proxying the artifactPath directly
		if len(h.Config.ProxyURLs) > 0 {
			for _, proxy := range h.Config.ProxyURLs {
				url := strings.TrimRight(proxy, "/") + "/" + artifactPath
				resp, err := h.Client.Get(url)
				if err == nil && resp.StatusCode == http.StatusOK {
					contentType := resp.Header.Get("Content-Type")
					if strings.HasPrefix(contentType, "text/html") {
						resp.Body.Close()
						continue
					}
					defer resp.Body.Close()

					cachePath := "repository/maven-public/" + artifactPath

					pr, pw := io.Pipe()
					go func() {
						if err := h.Store.Save(cachePath, pr); err != nil {
						}
					}()
					tee := io.TeeReader(resp.Body, pw)
					wrappedReader := &NotifyReader{Reader: tee, OnEOF: func() { pw.Close() }}
					c.DataFromReader(http.StatusOK, resp.ContentLength, contentType, wrappedReader, nil)
					return
				}
			}
		}

		c.Status(http.StatusNotFound)
	}
}

func (h *MavenHandler) HandleAggregateHead(basePath string) gin.HandlerFunc {
	return func(c *gin.Context) {
		artifactPath := strings.TrimPrefix(c.Param("path"), "/")
		repos := h.getAggregateRepos(basePath)

		// Check local repos
		for _, repo := range repos {
			fullPath := strings.TrimRight(repo, "/") + "/" + artifactPath
			found, err := h.Store.Head(fullPath)
			if err == nil && found {
				c.Status(http.StatusOK)
				return
			}
		}

		// Try proxy
		if len(h.Config.ProxyURLs) > 0 {
			for _, proxy := range h.Config.ProxyURLs {
				url := strings.TrimRight(proxy, "/") + "/" + artifactPath
				resp, err := h.Client.Head(url)
				if err == nil && resp.StatusCode == http.StatusOK {
					c.Status(http.StatusOK)
					return
				}
			}
		}

		c.Status(http.StatusNotFound)
	}
}

func (h *MavenHandler) getAggregateRepos(basePath string) []string {
	entries, err := h.Store.List(basePath)
	if err != nil {
		return nil
	}
	var repos []string
	hasReleases := false
	for _, e := range entries {
		if e.IsDir {
			// Don't aggregate the aggregate itself if it were a physical dir
			if e.Name == "maven-public" {
				continue
			}
			if e.Name == "maven-releases" {
				hasReleases = true
				continue
			}
			repos = append(repos, strings.TrimRight(basePath, "/")+"/"+e.Name)
		}
	}
	if hasReleases {
		repos = append([]string{strings.TrimRight(basePath, "/") + "/maven-releases"}, repos...)
	}
	return repos
}
