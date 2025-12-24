# Maven Repository Service

A lightweight, file-based Maven repository server written in Go.

## Features
- **Maven Protocol**: Supports `mvn deploy` and resolution.
- **Multi-Repository**: configurable via `/repository/:repoName`.
- **Proxy/Caching**: Fallback to upstream repositories (e.g., Maven Central).
- **Web UI**: Simple directory browsing.
- **Aggregate Routing**: `/repository/maven-public` automatically aggregates all local repositories (e.g., `maven-releases`, `develop`, etc.) with prioritized release lookup.
- **Log Rotation**: Daily automated log rollout and retention management.
- **Authentication**: Basic Auth (Env vars or File-based).

## Aggregate Routing
The server supports a virtual aggregate repository at `/repository/maven-public`. 

**How it works:**
- **Dynamic Discovery**: It automatically scans the storage directory (typically `artifacts/repository/`) for all available sub-repositories.
- **Prioritization**: `maven-releases` is always searched first to ensure stable artifacts are preferred. All other repositories (like `develop`, `staging`, etc.) are then searched in discovery order (treated as snapshots).
- **Single Entry Point**: Clients can use this single URL in their `settings.xml` or `pom.xml` to resolve all project dependencies without worrying about which specific repository they reside in.

## Building
```bash
go build -o maven_server
```

## Running

### Basic
```bash
./maven_server
```
Default credentials: `admin` / `password`.
Port: `8080`.

### Configuration
Environment variables:
- `MAVEN_PORT`: Server port (default 8080).
- `MAVEN_USERNAME`: Default admin username.
- `MAVEN_PASSWORD`: Default admin password.
- `MAVEN_ACCOUNTS_FILE`: Path to file with `user:pass` lines.
- `MAVEN_PROXY_URLS`: Comma-separated list of upstream proxy URLs.
- `MAVEN_STORAGE_PATH`: Location to store artifacts (default `./artifacts`).
- `MAVEN_ANONYMOUS_ACCESS`: Enable anonymous read access (default `false`).
- `MAVEN_SNAPSHOT_CLEANUP_ENABLED`: Enable background cleanup of snapshots (default `false`).
- `MAVEN_SNAPSHOT_CLEANUP_INTERVAL`: Interval between cleanup runs (default `1h`).
- `MAVEN_SNAPSHOT_KEEP_DAYS`: Retention period for snapshots in days (default `30`).
- `MAVEN_SNAPSHOT_KEEP_LATEST_ONLY`: If `true`, keep only the most recent snapshot file per artifact type/extension (default `false`).
- `MAVEN_LOG_PATH`: Path to the server log file (default `./server.log`).
- `MAVEN_LOG_KEEP_DAYS`: Number of days to keep rotated logs (default `7`).

### Example
```bash
export MAVEN_PORT=9090
export MAVEN_ANONYMOUS_ACCESS=true
export MAVEN_PROXY_URLS="https://repo.maven.apache.org/maven2"
./maven_server
```

### Admin API (Snapshot Cleanup)
The following endpoints require Basic Auth:
- `POST /admin/snapshots/cleanup/pause`: Pause the background cleanup task.
- `POST /admin/snapshots/cleanup/resume`: Resume the background cleanup task.
- `GET /admin/snapshots/cleanup/status`: Return the current status (`running` or `paused`).
- `POST /admin/snapshots/cleanup/trigger`: Manually trigger a cleanup run immediately.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
