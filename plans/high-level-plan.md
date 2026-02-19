# Plant Monitor — Specific Implementation Plan

This document captures the exact steps, file paths, interfaces, and commands needed to complete and deploy a three-service plant monitoring system with mTLS on Kubernetes.

---

## Current State

| Phase | Step | Status |
|---|---|---|
| **0 — Finish light-service** | Domain, ports, adapter implementations | ✅ Done |
| | gRPC handler (all three RPCs) | ✅ Done |
| | Dockerfile (multi-stage build) | ✅ Done |
| | Domain tests (`reading_test.go`) | ✅ Done |
| | 0a. Fix range boundary inconsistency (memory vs SQLite) | ✅ Done |
| | 0b. Configurable adapter selection via env vars (`REPO_TYPE`, `SENSOR_TYPE`) | ✅ Done |
| | 0c. TLS configuration in `main.go` | ❌ Missing |
| | 0d. Schedule periodic `DeleteOldReadings` in recorder | ❌ Missing |
| | 0e. Adapter and integration tests (memory, SQLite, gRPC handler) | ❌ Missing |
| | 0f. `buf.yaml` / `buf.gen.yaml` for reproducible proto generation | ❌ Missing |
| **1 — Build infrastructure** | `Makefile` | ❌ Not started |
| | `docker-compose.yml` | ❌ Not started |
| **2 — Plant service** | 2a. `plant.proto` definition | ❌ Not started |
| | 2b. Domain layer (`analysis.go` + tests) | ❌ Not started |
| | 2c. `LightClient` port interface | ❌ Not started |
| | 2d. gRPC client adapter → light-service | ❌ Not started |
| | 2e. gRPC server handler | ❌ Not started |
| | 2f. `main.go` wiring | ❌ Not started |
| | 2g. Dockerfile | ❌ Not started |
| **3 — Dashboard service** | 3a. HTTP handler (`/`, `/api/status`, `/api/history`) | ❌ Not started |
| | 3b. gRPC client adapter → plant-service | ❌ Not started |
| | 3c. Web UI (`index.html` with Chart.js) | ❌ Not started |
| | 3d. `main.go` wiring | ❌ Not started |
| | 3e. Dockerfile | ❌ Not started |
| **4 — mTLS** | `scripts/gen-certs.sh` | ❌ Not started |
| | `pkg/tlsconfig` package (per service) | ❌ Not started |
| | mTLS wired into all three services | ❌ Not started |
| **5 — Kubernetes (Minikube)** | Namespace manifest | ❌ Not started |
| | TLS secret | ❌ Not started |
| | light-service manifests (Deployment, Service, ConfigMap) | ❌ Not started |
| | plant-service manifests (Deployment, Service, ConfigMap) | ❌ Not started |
| | dashboard-service manifests (Deployment, Service, ConfigMap) | ❌ Not started |
| **6 — Envoy sidecars** *(optional)* | Envoy ConfigMaps (one per service) | ❌ Not started |
| | Sidecar containers added to Deployments | ❌ Not started |
| **7 — Raspberry Pi / K3s** | 7a. GPIO adapter (`bh1750.go` via periph.io) | ❌ Not started |
| | 7b. Build-tag separation (gpio vs mock) | ❌ Not started |
| | 7c. Cross-compilation for ARM64 | ❌ Not started |
| | 7d. K3s deployment | ❌ Not started |
| | 7e. Physical wiring (BH1750 → Pi GPIO) | ❌ Not started |

---

## Phase 0 — Finish light-service

**Goal:** Close the known gaps before building on top of it. Nothing downstream should depend on a broken foundation.

### 0a. Fix the range boundary inconsistency

`GetReadingsInRange` uses exclusive bounds in the memory adapter but inclusive bounds in SQLite. Standardise on **inclusive start, exclusive end** (the conventional half-open interval), which matches how time ranges are naturally expressed.

**`internal/adapters/memory/reading_repository.go:64`** — change:
```go
// current (exclusive on both ends)
if reading.Timestamp.After(start) && reading.Timestamp.Before(end) {

// fix (inclusive start, exclusive end)
if !reading.Timestamp.Before(start) && reading.Timestamp.Before(end) {
```

**`internal/adapters/sqlite/reading_repository.go:86`** — change query to:
```sql
WHERE timestamp >= ? AND timestamp < ?
```

Update `domain.ReadingRepository` interface comment to document the chosen convention so both adapters stay in sync.

### 0b. Add configurable adapter selection to main.go

Replace the hard-wired `memory` + `mock` instantiation with env-var-driven selection:

| Variable | Values | Default | Purpose |
|---|---|---|---|
| `REPO_TYPE` | `memory`, `sqlite` | `memory` | Which repository adapter to use |
| `DB_PATH` | file path | `./light.db` | SQLite database file (only used when `REPO_TYPE=sqlite`) |
| `SENSOR_TYPE` | `mock`, `gpio` | `mock` | Which sensor adapter to use (gpio added in Phase 7) |

```go
// In loadConfig():
RepoType   string  // "memory" | "sqlite"
DBPath     string
SensorType string  // "mock" | "gpio"

// In main():
var repo domain.ReadingRepository
switch config.RepoType {
case "sqlite":
    r, err := sqlite.NewReadingRepository(config.DBPath)
    // handle err
    defer r.Close()
    repo = r
default:
    repo = memory.NewReadingRepository()
}

var sensor ports.LightSensor
switch config.SensorType {
case "gpio":
    // Phase 7 — placeholder, fatal error for now
    log.Fatal().Msg("gpio sensor not yet implemented; set SENSOR_TYPE=mock")
default:
    sensor = mock.NewFakeSensor(500.0, 100.0)
}
```

### 0c. Add TLS configuration to main.go

Read `TLS_CERT`, `TLS_KEY`, `TLS_CA` from the environment. If all three are set, start with mTLS; otherwise start insecure. This is the pattern all three services will follow.

Add to `loadConfig()`:
```go
TLSCert string  // path to this service's certificate
TLSKey  string  // path to this service's private key
TLSCA   string  // path to the CA certificate
```

Add to `main()`:
```go
var serverOpts []grpc.ServerOption
if config.TLSCert != "" {
    tlsCfg, err := tlsconfig.LoadServerTLS(config.TLSCert, config.TLSKey, config.TLSCA)
    if err != nil {
        log.Fatal().Err(err).Msg("failed to load TLS config")
    }
    serverOpts = append(serverOpts, grpc.Creds(credentials.NewTLS(tlsCfg)))
    log.Info().Msg("mTLS enabled")
} else {
    log.Warn().Msg("TLS_CERT not set — starting without TLS (dev mode only)")
}
grpcServer := grpc.NewServer(serverOpts...)
```

The `tlsconfig` package lives at `services/light-service/pkg/tlsconfig/tlsconfig.go` — it will be duplicated into plant-service and dashboard-service (same code, same package, separate modules). See Phase 4 for the full implementation.

### 0d. Schedule periodic DeleteOldReadings

Add to `Recorder.Start()` in `ports/reader.go` — run cleanup once per day:

```go
cleanupTicker := time.NewTicker(24 * time.Hour)
defer cleanupTicker.Stop()

for {
    select {
    case <-ticker.C:
        r.recordOnce(ctx)
    case <-cleanupTicker.C:
        if err := r.repo.DeleteOldReadings(ctx, 30*24*time.Hour); err != nil {
            log.Error().Err(err).Msg("failed to delete old readings")
        } else {
            log.Info().Msg("deleted readings older than 30 days")
        }
    case <-ctx.Done():
        return
    }
}
```

### 0e. Add adapter and integration tests

Files to create:

```
services/light-service/internal/adapters/memory/reading_repository_test.go
services/light-service/internal/adapters/sqlite/reading_repository_test.go
services/light-service/internal/adapters/grpc/handler_test.go
```

**`memory/reading_repository_test.go`** — test the interface contract:
- `TestSaveAndGetReading` — save a reading, retrieve by ID
- `TestGetLatestReading_Empty` — returns `ErrReadingNotFound`
- `TestGetReadingsInRange` — boundary inclusive/exclusive behaviour
- `TestDeleteOldReadings` — old entries removed, recent ones kept

**`sqlite/reading_repository_test.go`** — same test cases, using `t.TempDir()` for the database file. Verifies parity with the memory adapter.

**`grpc/handler_test.go`** — create a real in-process gRPC server using `net.Pipe()` (no port needed):
- `TestGetCurrentLight_NoReadings` — triggers sensor read, returns a reading
- `TestRecordReading_ThenGetCurrent` — records a reading, verifies it comes back
- `TestGetHistory_TimeRange` — seeds readings, verifies correct range returned

### 0f. Add buf config for reproducible proto generation

```
services/light-service/buf.yaml
services/light-service/buf.gen.yaml
```

```yaml
# buf.yaml
version: v2
modules:
  - path: api/proto
```

```yaml
# buf.gen.yaml
version: v2
plugins:
  - remote: buf.build/protocolbuffers/go
    out: pkg/pb
    opt: paths=source_relative
  - remote: buf.build/grpc/go
    out: pkg/pb
    opt: paths=source_relative
```

**Validation for Phase 0:**
```bash
cd services/light-service && go test ./...           # all tests pass
REPO_TYPE=sqlite DB_PATH=/tmp/test.db go run ./cmd/server  # starts with SQLite
TLS_CERT=... TLS_KEY=... TLS_CA=... go run ./cmd/server    # starts with mTLS (after Phase 4)
```

---

## Phase 1 — Root Build Infrastructure

**Goal:** One command builds everything. Local dev runs with `docker compose up`.

### Files to create

```
plant-monitor/
├── Makefile
└── docker-compose.yml
```

### Makefile targets

```makefile
# Key targets to implement:
proto        # Regenerate all protobuf/gRPC code
build        # Build all three services
test         # Run all tests
docker-build # Build all Docker images
up           # Start with docker compose
certs        # Generate mTLS certificates (Phase 4)
```

Full target list with exact commands:

```makefile
SERVICES := light-service plant-service dashboard-service

proto:
    cd services/light-service && buf generate
    cd services/plant-service && buf generate

build:
    for svc in $(SERVICES); do \
        go build -o bin/$$svc ./services/$$svc/cmd/server; \
    done

test:
    for svc in $(SERVICES); do \
        cd services/$$svc && go test ./... && cd ../..; \
    done

docker-build:
    for svc in $(SERVICES); do \
        docker build -t plant-monitor/$$svc:latest ./services/$$svc; \
    done

up:
    docker compose up --build

certs:
    bash scripts/gen-certs.sh
```

### docker-compose.yml structure

Three services with shared network, volume-mounted certs:

```yaml
services:
  light-service:
    build: ./services/light-service
    environment:
      PORT: "50051"
      RECORD_INTERVAL: "30s"
      TLS_CERT: /certs/light-service.crt
      TLS_KEY: /certs/light-service.key
      TLS_CA: /certs/ca.crt
    volumes:
      - ./certs:/certs:ro
    ports:
      - "50051:50051"

  plant-service:
    build: ./services/plant-service
    environment:
      PORT: "50052"
      LIGHT_SERVICE_ADDR: "light-service:50051"
      TLS_CERT: /certs/plant-service.crt
      TLS_KEY: /certs/plant-service.key
      TLS_CA: /certs/ca.crt
    volumes:
      - ./certs:/certs:ro
    depends_on: [light-service]

  dashboard-service:
    build: ./services/dashboard-service
    environment:
      PORT: "8080"
      PLANT_SERVICE_ADDR: "plant-service:50052"
      TLS_CERT: /certs/dashboard-service.crt
      TLS_KEY: /certs/dashboard-service.key
      TLS_CA: /certs/ca.crt
    volumes:
      - ./certs:/certs:ro
    ports:
      - "8080:8080"
    depends_on: [plant-service]
```

**Validation:** `docker compose up` → `curl http://localhost:8080` returns HTML.

---

## Phase 2 — Plant Service

**Goal:** A service that calls light-service, analyzes lighting patterns, and serves plant care recommendations via gRPC.

### Directory structure

```
services/plant-service/
├── Dockerfile
├── go.mod
├── api/
│   └── proto/
│       └── plant.proto
├── cmd/
│   └── server/
│       └── main.go
├── internal/
│   ├── domain/
│   │   ├── analysis.go           # Domain model + business logic
│   │   ├── analysis_test.go      # Unit tests
│   │   └── errors.go
│   ├── ports/
│   │   └── light_client.go       # LightClient interface (port)
│   └── adapters/
│       └── grpc/
│           ├── handler.go         # PlantService gRPC server
│           └── light_client.go    # gRPC client calling light-service
└── pkg/
    └── pb/                        # Generated protobuf (gitignored raw, committed generated)
        ├── plant.pb.go
        └── plant_grpc.pb.go
```

### 2a. Proto definition — `api/proto/plant.proto`

```proto
syntax = "proto3";
package plant.v1;
option go_package = "github.com/quentinrf/plant-monitor/services/plant-service/pkg/pb";

service PlantService {
  rpc GetPlantStatus(GetPlantStatusRequest) returns (GetPlantStatusResponse);
  rpc GetHistory(GetHistoryRequest)         returns (GetHistoryResponse);
}

message GetPlantStatusRequest {}

message GetPlantStatusResponse {
  PlantStatus status = 1;
}

message GetHistoryRequest {
  int64 start_time = 1;
  int64 end_time   = 2;
}

message GetHistoryResponse {
  repeated HistoryPoint points   = 1;
  string                trend    = 2;  // "stable" | "brightening" | "darkening"
}

message PlantStatus {
  string recommendation = 1;  // e.g. "Low Light — most houseplants will thrive here"
  string light_category = 2;  // "Low Light" | "Medium Light" | "High Light"
  double current_lux    = 3;
  string trend          = 4;
  int64  timestamp      = 5;
}

message HistoryPoint {
  int64  timestamp = 1;
  double lux       = 2;
  string category  = 3;
}
```

### 2b. Domain layer — `internal/domain/analysis.go`

Key types and functions to implement:

```go
package domain

// LightAnalysis is the result of analyzing a set of light readings
type LightAnalysis struct {
    CurrentLux     float64
    AverageLux     float64
    Trend          string   // "stable" | "brightening" | "darkening"
    Category       string
    Recommendation string
}

// Thresholds — keep in sync with light-service domain
const (
    LowLightMax    = 200.0   // lux
    MediumLightMax = 2500.0  // lux
)

// Analyze produces a LightAnalysis from a slice of lux readings (oldest first)
// Business rules:
//   - Trend: compare first-half avg vs second-half avg; >10% change = trending
//   - Recommendation: mapped from category (see recommendationForCategory)
func Analyze(currentLux float64, history []float64) LightAnalysis

// recommendationForCategory maps light level to plant care advice
// Low Light  → "Low Light — most houseplants will thrive here"
// Medium     → "Medium Light — ideal for most tropical plants"
// High Light → "High Light — perfect for succulents and cacti"
func recommendationForCategory(category string) string

// categoryFromLux returns the light category string from a lux value
func categoryFromLux(lux float64) string
```

Unit tests to write in `analysis_test.go`:
- `TestAnalyze_LowLight` — lux=100, expect recommendation contains "houseplants"
- `TestAnalyze_Trend_Brightening` — increasing sequence, expect trend="brightening"
- `TestAnalyze_Trend_Darkening` — decreasing sequence, expect trend="darkening"
- `TestAnalyze_Trend_Stable` — flat sequence, expect trend="stable"
- `TestAnalyze_EmptyHistory` — no history, no panic, sensible defaults

### 2c. Port interface — `internal/ports/light_client.go`

```go
package ports

import (
    "context"
    "time"
)

// LightReading is a simplified reading as seen by plant-service
type LightReading struct {
    Lux       float64
    Timestamp time.Time
    Category  string
}

// LightClient is the port for fetching data from light-service
// Implemented by: grpc.LightClientAdapter (real), mock.LightClient (tests)
type LightClient interface {
    GetCurrentLux(ctx context.Context) (*LightReading, error)
    GetHistory(ctx context.Context, start, end time.Time) ([]LightReading, error)
    Close() error
}
```

### 2d. gRPC light-service client adapter — `internal/adapters/grpc/light_client.go`

```go
// LightClientAdapter implements ports.LightClient using gRPC
type LightClientAdapter struct {
    conn   *grpc.ClientConn
    client lightpb.LightServiceClient
}

// NewLightClientAdapter creates the gRPC client
// addr format: "host:port"
// tlsConfig: nil for insecure, non-nil for mTLS
func NewLightClientAdapter(addr string, tlsConfig *tls.Config) (*LightClientAdapter, error)
```

Key implementation detail: the adapter imports light-service's generated pb package. Use a `replace` directive in go.mod during development, then switch to a versioned module path or copy the pb files.

**go.mod approach (simplest for a mono-repo):**

```
// In services/plant-service/go.mod
require (
    github.com/quentinrf/plant-monitor/services/light-service v0.0.0
    ...
)

replace github.com/quentinrf/plant-monitor/services/light-service => ../light-service
```

### 2e. gRPC server handler — `internal/adapters/grpc/handler.go`

```go
type PlantServiceHandler struct {
    pb.UnimplementedPlantServiceServer
    lightClient ports.LightClient
}

func NewPlantServiceHandler(lightClient ports.LightClient) *PlantServiceHandler

// GetPlantStatus: fetch current + last hour of history → Analyze → return PlantStatus
func (h *PlantServiceHandler) GetPlantStatus(ctx context.Context, ...) (...)

// GetHistory: fetch last 24h from light-service → map to HistoryPoints + compute trend
func (h *PlantServiceHandler) GetHistory(ctx context.Context, ...) (...)
```

### 2f. main.go wiring

Environment variables to read:

| Variable | Default | Purpose |
|---|---|---|
| `PORT` | `50052` | gRPC listen port |
| `LIGHT_SERVICE_ADDR` | `localhost:50051` | Address of light-service |
| `TLS_CERT` | `` | Path to this service's cert |
| `TLS_KEY` | `` | Path to this service's key |
| `TLS_CA` | `` | Path to CA cert for verifying peers |

If `TLS_CERT` is empty → start insecure (dev mode). If set → enable mTLS.

### 2g. Dockerfile

Identical pattern to light-service's multi-stage Dockerfile. Key difference: no CGO needed (no SQLite), so add `CGO_ENABLED=0` for a cleaner static binary.

**Validation commands:**
```bash
cd services/plant-service && go test ./...
docker build -t plant-monitor/plant-service ./services/plant-service
# With light-service running:
grpcurl -plaintext localhost:50052 plant.v1.PlantService/GetPlantStatus
```

---

## Phase 3 — Dashboard Service

**Goal:** A browser-accessible HTTP server that renders a live Chart.js dashboard showing lux readings and plant recommendations.

### Directory structure

```
services/dashboard-service/
├── Dockerfile
├── go.mod
├── cmd/
│   └── server/
│       └── main.go
└── internal/
    ├── adapters/
    │   ├── grpc/
    │   │   └── plant_client.go    # gRPC client → plant-service
    │   └── http/
    │       └── handler.go         # HTTP handler serving UI + JSON API
    └── web/
        └── templates/
            └── index.html         # Single-page UI with Chart.js
```

### 3a. HTTP handler — `internal/adapters/http/handler.go`

Three endpoints:

| Method | Path | Returns |
|---|---|---|
| GET | `/` | HTML page (index.html template) |
| GET | `/api/status` | JSON: current PlantStatus |
| GET | `/api/history` | JSON: last 24h of HistoryPoints |

```go
type Handler struct {
    plantClient ports.PlantClient
    templates   *template.Template
}

func NewHandler(plantClient ports.PlantClient) *Handler

// ServeHTTP routes requests to the correct method
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request)
```

### 3b. gRPC plant-service client — `internal/adapters/grpc/plant_client.go`

```go
type PlantClientAdapter struct {
    conn   *grpc.ClientConn
    client pb.PlantServiceClient
}

func NewPlantClientAdapter(addr string, tlsConfig *tls.Config) (*PlantClientAdapter, error)

// GetCurrentStatus fetches PlantStatus from plant-service
func (c *PlantClientAdapter) GetCurrentStatus(ctx context.Context) (*ports.PlantStatus, error)

// GetHistory fetches 24h of history
func (c *PlantClientAdapter) GetHistory(ctx context.Context) ([]ports.HistoryPoint, error)
```

### 3c. Web UI — `internal/web/templates/index.html`

Single HTML file with embedded Chart.js (CDN). Structure:

```html
<!DOCTYPE html>
<html>
<head>
  <title>Plant Monitor</title>
  <!-- Chart.js from CDN (pinned version) -->
  <script src="https://cdn.jsdelivr.net/npm/chart.js@4.4.0/dist/chart.umd.min.js"></script>
</head>
<body>
  <!-- Status card: recommendation + category badge + current lux -->
  <div id="status-card">...</div>

  <!-- Time-series chart: lux over last 24h -->
  <canvas id="lux-chart"></canvas>

  <script>
    // Poll /api/status every 10s, /api/history every 60s
    // Update status card and chart data
    async function refresh() { ... }
    setInterval(refresh, 10000);
    refresh(); // Initial load
  </script>
</body>
</html>
```

Key JS behavior:
- On load: fetch both endpoints, render chart and status card
- Every 10s: re-fetch `/api/status`, update status card (lux, category, recommendation)
- Every 60s: re-fetch `/api/history`, update chart data
- Chart type: line chart, x-axis = timestamps, y-axis = lux, threshold lines at 200 and 2500

### 3d. main.go

Environment variables:

| Variable | Default | Purpose |
|---|---|---|
| `PORT` | `8080` | HTTP listen port |
| `PLANT_SERVICE_ADDR` | `localhost:50052` | Address of plant-service |
| `TLS_CERT` | `` | Client cert for mTLS to plant-service |
| `TLS_KEY` | `` | Client key |
| `TLS_CA` | `` | CA cert |

**Validation:**
```bash
docker compose up
open http://localhost:8080
# Cover sensor with hand → status should change within 30s
```

---

## Phase 4 — mTLS

**Goal:** Every service-to-service gRPC connection is mutually authenticated with TLS. Servers require client certs; clients present their cert.

### Certificate structure

```
certs/                  # .gitignored — never commit private keys
├── ca.crt              # CA certificate (trusted by all services)
├── ca.key              # CA private key (never leaves dev machine / K8s secret)
├── light-service.crt
├── light-service.key
├── plant-service.crt
├── plant-service.key
├── dashboard-service.crt
└── dashboard-service.key
```

### `scripts/gen-certs.sh`

Generates a self-signed CA and per-service leaf certificates. Key parameters:

```bash
# CA
openssl genrsa -out certs/ca.key 4096
openssl req -new -x509 -key certs/ca.key -out certs/ca.crt -days 3650 \
  -subj "/CN=plant-monitor-ca"

# Per service (repeat for each of light-service, plant-service, dashboard-service)
SERVICE=light-service
openssl genrsa -out certs/$SERVICE.key 2048
openssl req -new -key certs/$SERVICE.key -out certs/$SERVICE.csr \
  -subj "/CN=$SERVICE"
openssl x509 -req -in certs/$SERVICE.csr -CA certs/ca.crt -CAkey certs/ca.key \
  -CAcreateserial -out certs/$SERVICE.crt -days 365 \
  -extfile <(printf "subjectAltName=DNS:$SERVICE,DNS:localhost")
```

The SAN `DNS:<service-name>` must match the Kubernetes Service hostname. `DNS:localhost` enables local testing.

### mTLS Go configuration (shared helper pattern)

Create `pkg/tlsconfig/tlsconfig.go` in each service (or a shared internal package):

```go
package tlsconfig

import (
    "crypto/tls"
    "crypto/x509"
    "os"
)

// LoadServerTLS creates a tls.Config for a gRPC server requiring client certs
func LoadServerTLS(certFile, keyFile, caFile string) (*tls.Config, error) {
    cert, err := tls.LoadX509KeyPair(certFile, keyFile)
    // ...
    caCert, err := os.ReadFile(caFile)
    // ...
    caPool := x509.NewCertPool()
    caPool.AppendCertsFromPEM(caCert)
    return &tls.Config{
        Certificates: []tls.Certificate{cert},
        ClientCAs:    caPool,
        ClientAuth:   tls.RequireAndVerifyClientCert,
    }, nil
}

// LoadClientTLS creates a tls.Config for a gRPC client presenting a cert
func LoadClientTLS(certFile, keyFile, caFile string) (*tls.Config, error) {
    cert, err := tls.LoadX509KeyPair(certFile, keyFile)
    // ...
    // RootCAs verifies the server cert
    return &tls.Config{
        Certificates: []tls.Certificate{cert},
        RootCAs:      caPool,
    }, nil
}
```

### Changes to existing services

**light-service `main.go`:** Add TLS branch:

```go
var serverOpts []grpc.ServerOption
if config.TLSCert != "" {
    tlsCfg, err := tlsconfig.LoadServerTLS(config.TLSCert, config.TLSKey, config.TLSCA)
    // handle err
    serverOpts = append(serverOpts, grpc.Creds(credentials.NewTLS(tlsCfg)))
}
grpcServer := grpc.NewServer(serverOpts...)
```

**plant-service gRPC client:** Add TLS branch when connecting to light-service.

**dashboard-service gRPC client:** Add TLS branch when connecting to plant-service.

**Validation:**
```bash
make certs
docker compose up
# Verify mTLS is required:
grpcurl -plaintext localhost:50051 light.v1.LightService/GetCurrentLight
# ^ should fail with: "transport: Error while dialing..."
grpcurl -cacert certs/ca.crt -cert certs/plant-service.crt -key certs/plant-service.key \
  localhost:50051 light.v1.LightService/GetCurrentLight
# ^ should succeed
```

---

## Phase 5 — Kubernetes Manifests (Minikube)

**Goal:** All three services run in Minikube with mTLS enforced via native gRPC TLS (no Envoy yet — that's a follow-on).

### Directory structure

```
k8s/
├── namespace.yaml
├── secrets/
│   └── tls-certs.yaml          # Created by: make k8s-certs (never committed with real data)
├── light-service/
│   ├── configmap.yaml
│   ├── deployment.yaml
│   └── service.yaml
├── plant-service/
│   ├── configmap.yaml
│   ├── deployment.yaml
│   └── service.yaml
└── dashboard-service/
    ├── configmap.yaml
    ├── deployment.yaml
    └── service.yaml
```

### Namespace

```yaml
# k8s/namespace.yaml
apiVersion: v1
kind: Namespace
metadata:
  name: plant-monitor
```

### TLS Secret (one shared secret for all certs)

```bash
# Generate the secret from certs directory (add to Makefile as k8s-certs target)
kubectl create secret generic tls-certs -n plant-monitor \
  --from-file=ca.crt=certs/ca.crt \
  --from-file=light-service.crt=certs/light-service.crt \
  --from-file=light-service.key=certs/light-service.key \
  --from-file=plant-service.crt=certs/plant-service.crt \
  --from-file=plant-service.key=certs/plant-service.key \
  --from-file=dashboard-service.crt=certs/dashboard-service.crt \
  --from-file=dashboard-service.key=certs/dashboard-service.key \
  --dry-run=client -o yaml > k8s/secrets/tls-certs.yaml
# NOTE: Add k8s/secrets/tls-certs.yaml to .gitignore — contains private keys
```

### light-service Deployment

```yaml
# k8s/light-service/deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: light-service
  namespace: plant-monitor
spec:
  replicas: 1
  selector:
    matchLabels:
      app: light-service
  template:
    metadata:
      labels:
        app: light-service
    spec:
      containers:
        - name: light-service
          image: plant-monitor/light-service:latest
          imagePullPolicy: Never          # Use local Minikube image registry
          ports:
            - containerPort: 50051
          env:
            - name: PORT
              value: "50051"
            - name: RECORD_INTERVAL
              valueFrom:
                configMapKeyRef:
                  name: light-service-config
                  key: RECORD_INTERVAL
            - name: TLS_CERT
              value: /certs/light-service.crt
            - name: TLS_KEY
              value: /certs/light-service.key
            - name: TLS_CA
              value: /certs/ca.crt
          volumeMounts:
            - name: tls-certs
              mountPath: /certs
              readOnly: true
      volumes:
        - name: tls-certs
          secret:
            secretName: tls-certs
```

```yaml
# k8s/light-service/service.yaml
apiVersion: v1
kind: Service
metadata:
  name: light-service
  namespace: plant-monitor
spec:
  selector:
    app: light-service
  ports:
    - port: 50051
      targetPort: 50051
  type: ClusterIP
```

```yaml
# k8s/light-service/configmap.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: light-service-config
  namespace: plant-monitor
data:
  RECORD_INTERVAL: "30s"
```

plant-service and dashboard-service follow the same pattern. dashboard-service uses `type: NodePort` (or `LoadBalancer` on Minikube) for external access.

### Deployment commands

```bash
# One-time setup
minikube start
eval $(minikube docker-env)     # Point Docker CLI to Minikube's registry

# Build images into Minikube's registry
make docker-build

# Apply manifests
kubectl apply -f k8s/namespace.yaml
kubectl apply -f k8s/secrets/tls-certs.yaml
kubectl apply -f k8s/light-service/
kubectl apply -f k8s/plant-service/
kubectl apply -f k8s/dashboard-service/

# Access dashboard
minikube service dashboard-service -n plant-monitor
```

### Makefile targets to add (Phase 5)

```makefile
k8s-deploy:
    kubectl apply -f k8s/namespace.yaml
    kubectl apply -f k8s/secrets/tls-certs.yaml
    kubectl apply -f k8s/light-service/
    kubectl apply -f k8s/plant-service/
    kubectl apply -f k8s/dashboard-service/

k8s-delete:
    kubectl delete namespace plant-monitor

k8s-status:
    kubectl get pods -n plant-monitor
    kubectl get services -n plant-monitor
```

**Validation:**
```bash
kubectl get pods -n plant-monitor        # All pods Running
kubectl logs -n plant-monitor deploy/light-service
# Open Minikube dashboard URL in browser → see live chart
```

---

## Phase 6 — Envoy Sidecar (Optional, for Portfolio Depth)

Demonstrates the service mesh pattern. Add after Phase 5 is working cleanly.

Each pod gets a second container: the Envoy proxy. gRPC traffic flows:

```
plant-service app → localhost:9001 (Envoy) → light-service:50051
```

Envoy handles the mTLS termination; app containers communicate in plaintext on localhost. This mirrors how Istio/Linkerd work.

### Files to add

```
k8s/envoy/
├── light-service-envoy.yaml      # Envoy ConfigMap for light-service's sidecar
├── plant-service-envoy.yaml      # Envoy ConfigMap for plant-service's sidecar
└── dashboard-service-envoy.yaml
```

Each Envoy ConfigMap contains a `envoy.yaml` with:
- Listener on `0.0.0.0:<grpc-port>` (inbound, terminates mTLS from callers)
- Cluster pointing to `127.0.0.1:<app-port>` (plaintext to app)
- Outbound cluster for each upstream service with mTLS origination

Deployment change: add the Envoy sidecar container to each Deployment's pod spec.

---

## Phase 7 — Raspberry Pi / K3s

**Goal:** Run the full system on real hardware with a real BH1750 sensor.

### 7a. GPIO adapter — `services/light-service/internal/adapters/gpio/bh1750.go`

```go
// BH1750Sensor implements ports.LightSensor using periph.io
type BH1750Sensor struct {
    dev *bh1750.Dev
}

func NewBH1750Sensor() (*BH1750Sensor, error) {
    // 1. host.Init() — initializes periph.io host drivers
    // 2. i2creg.Open("") — opens default I2C bus (/dev/i2c-1 on Pi)
    // 3. bh1750.New(bus, &bh1750.DefaultOpts) — creates device
    // Returns sensor or error
}

func (s *BH1750Sensor) ReadLux(ctx context.Context) (float64, error) {
    env, err := s.dev.SenseContinuous(ctx)
    return float64(env.Lux) / 1000.0, err   // millilux → lux
}

func (s *BH1750Sensor) Close() error { return s.dev.Halt() }
```

**Dependencies to add to light-service go.mod:**
```
periph.io/x/periph v3.6.8
periph.io/x/devices/bh1750 v0.0.0-...
```

### 7b. Build-tag separation

Use Go build tags to compile the right sensor for each target:

```go
// internal/adapters/gpio/bh1750.go
//go:build linux && arm64
```

```go
// internal/adapters/mock/fake_sensor.go
//go:build !linux || !arm64
```

**main.go** detects at runtime which adapter to use via `SENSOR_TYPE` env var (`mock` | `gpio`):

```go
sensorType := os.Getenv("SENSOR_TYPE")
if sensorType == "gpio" {
    sensor, err = gpio.NewBH1750Sensor()
} else {
    sensor = mock.NewFakeSensor(500.0, 100.0)
}
```

### 7c. Cross-compilation

From the laptop, build an ARM64 binary:

```bash
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 \
  go build -o bin/light-service-arm64 \
  ./services/light-service/cmd/server
```

Note: CGO must be disabled for cross-compilation. The SQLite adapter uses CGO; on the Pi, either:
- Use the in-memory adapter (loses data on restart)
- Use the pure-Go SQLite driver: `modernc.org/sqlite` (drop-in replacement, no CGO)

**Recommendation:** Switch to `modernc.org/sqlite` now (during Phase 2) to avoid this constraint entirely.

### 7d. K3s deployment

```bash
# On the Raspberry Pi
curl -sfL https://get.k3s.io | sh -

# Copy manifests and images from laptop
scp -r k8s/ pi@raspberrypi:~/
scp bin/*-arm64 pi@raspberrypi:~/

# Import images (or use a local registry)
k3s ctr images import light-service.tar

# Deploy (same manifests, different image names)
kubectl apply -f k8s/
```

Additional change: light-service Deployment needs `SENSOR_TYPE: gpio` in the ConfigMap.

### 7e. Physical wiring (BH1750 / GY-302 module)

| Pi Pin | Name | BH1750 Pin |
|---|---|---|
| 1 | 3.3V | VCC |
| 6 | GND | GND |
| 3 | GPIO2 / SDA1 | SDA |
| 5 | GPIO3 / SCL1 | SCL |

Enable I2C: `sudo raspi-config → Interface Options → I2C → Enable`

Verify sensor detected: `i2cdetect -y 1` (should show `0x23`)

---

## Implementation Order

This is the recommended sequence to always have a running, demonstrable system:

1. **Finish light-service (Phase 0)** — Fix boundary bug, add env-var adapter selection, add TLS skeleton, schedule cleanup, add adapter tests, add buf config. Produces a solid, tested foundation.
2. **Makefile + Docker Compose (Phase 1)** — 1 session. Unlocks `make test` and `docker compose up`.
3. **Plant Service domain + tests (Phase 2a–2c)** — 1 session. Pure Go, no external dependencies.
4. **Plant Service gRPC wiring (Phase 2d–2g)** — 1 session. End-to-end gRPC call chain working.
5. **Dashboard Service (Phase 3)** — 1 session. Full system visible in browser.
6. **mTLS (Phase 4)** — 1 session. Cert generation + TLS config wired into all three services.
7. **Kubernetes on Minikube (Phase 5)** — 1 session. Portable demo on laptop.
8. **Raspberry Pi (Phase 7)** — After hardware arrives. Swap mock sensor for GPIO adapter.
9. **Envoy sidecars (Phase 6)** — Optional polish if time permits.

---

## Key Architecture Decisions to Preserve

- **Hexagonal architecture:** Every service has `domain/`, `ports/`, `adapters/` layers. No domain code imports adapters.
- **Interface-driven adapters:** All external dependencies (sensor, storage, upstream services) hidden behind interfaces. Enables unit testing without real infrastructure.
- **Zero-value mTLS:** If TLS env vars are absent → insecure mode. Makes local `go run` work without certs.
- **Zerolog everywhere:** Structured JSON logs in production, console-pretty in dev. Same initialization pattern as light-service.
- **No Envoy required for mTLS demo:** Native gRPC TLS is sufficient to show mTLS. Envoy adds the service-mesh story if time permits.
