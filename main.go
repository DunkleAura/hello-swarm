package main

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"sort"
	"strings"
	"syscall"
	"time"
)

var version = "dev"

var startedAt = time.Now().UTC()

//go:embed web
var webFiles embed.FS

type config struct {
	HTTPAddr string
}

type instanceInfo struct {
	InstanceID   string   `json:"instance_id"`
	Hostname     string   `json:"hostname"`
	IPAddresses  []string `json:"ip_addresses"`
	Version      string   `json:"version"`
	GoVersion    string   `json:"go_version"`
	OS           string   `json:"os"`
	Architecture string   `json:"architecture"`
	StartedAt    string   `json:"started_at"`
	NodeName     string   `json:"node_name,omitempty"`
	ServiceName  string   `json:"service_name,omitempty"`
	TaskName     string   `json:"task_name,omitempty"`
	TaskSlot     string   `json:"task_slot,omitempty"`
}

func main() {
	if len(os.Args) == 2 && os.Args[1] == "healthcheck" {
		if err := checkHealth(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg := loadConfig()
	info, err := collectInstanceInfo()
	if err != nil {
		logger.Error("collect instance metadata", "error", err)
		os.Exit(1)
	}

	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           requestLogger(logger, newHandler(info)),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			logger.Error("graceful shutdown", "error", err)
		}
	}()

	logger.Info("server started", "address", cfg.HTTPAddr, "version", version, "instance_id", info.InstanceID)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("server stopped unexpectedly", "error", err)
		os.Exit(1)
	}
	logger.Info("server stopped")
}

func loadConfig() config {
	addr := strings.TrimSpace(os.Getenv("HTTP_ADDR"))
	if addr == "" {
		addr = ":8080"
	}
	return config{HTTPAddr: addr}
}

func checkHealth() error {
	url := strings.TrimSpace(os.Getenv("HEALTHCHECK_URL"))
	if url == "" {
		url = "http://127.0.0.1:8080/healthz"
	}
	client := http.Client{Timeout: 2 * time.Second}
	response, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("healthcheck request: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("healthcheck returned %s", response.Status)
	}
	return nil
}

func collectInstanceInfo() (instanceInfo, error) {
	hostname, err := os.Hostname()
	if err != nil {
		return instanceInfo{}, fmt.Errorf("read hostname: %w", err)
	}

	addresses, err := localIPAddresses()
	if err != nil {
		return instanceInfo{}, err
	}

	taskName := strings.TrimSpace(os.Getenv("SWARM_TASK_NAME"))
	instanceID := taskName
	if instanceID == "" {
		instanceID = hostname
	}

	return instanceInfo{
		InstanceID:   instanceID,
		Hostname:     hostname,
		IPAddresses:  addresses,
		Version:      version,
		GoVersion:    runtime.Version(),
		OS:           runtime.GOOS,
		Architecture: runtime.GOARCH,
		StartedAt:    startedAt.Format(time.RFC3339),
		NodeName:     strings.TrimSpace(os.Getenv("SWARM_NODE_NAME")),
		ServiceName:  strings.TrimSpace(os.Getenv("SWARM_SERVICE_NAME")),
		TaskName:     taskName,
		TaskSlot:     strings.TrimSpace(os.Getenv("SWARM_TASK_SLOT")),
	}, nil
}

func localIPAddresses() ([]string, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("list network interfaces: %w", err)
	}

	seen := make(map[string]struct{})
	for _, networkInterface := range interfaces {
		if networkInterface.Flags&net.FlagUp == 0 || networkInterface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addresses, err := networkInterface.Addrs()
		if err != nil {
			return nil, fmt.Errorf("list addresses for %s: %w", networkInterface.Name, err)
		}
		for _, address := range addresses {
			ip, _, err := net.ParseCIDR(address.String())
			if err != nil || ip.IsLoopback() || ip.IsUnspecified() || ip.IsLinkLocalUnicast() {
				continue
			}
			seen[ip.String()] = struct{}{}
		}
	}

	addresses := make([]string, 0, len(seen))
	for address := range seen {
		addresses = append(addresses, address)
	}
	sort.Slice(addresses, func(i, j int) bool {
		left := net.ParseIP(addresses[i])
		right := net.ParseIP(addresses[j])
		leftV4 := left.To4() != nil
		rightV4 := right.To4() != nil
		if leftV4 != rightV4 {
			return leftV4
		}
		return addresses[i] < addresses[j]
	})
	return addresses, nil
}

func newHandler(info instanceInfo) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/info", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Connection", "close")
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		if err := json.NewEncoder(w).Encode(info); err != nil {
			return
		}
	})
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok\n"))
	})

	content, err := fs.Sub(webFiles, "web")
	if err != nil {
		panic(fmt.Sprintf("load embedded web files: %v", err))
	}
	mux.Handle("GET /", http.FileServer(http.FS(content)))

	return securityHeaders(mux)
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy", "default-src 'self'; connect-src 'self'; script-src 'self'; style-src 'self'; img-src 'self'; base-uri 'none'; frame-ancestors 'none'")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		next.ServeHTTP(w, r)
	})
}

type responseRecorder struct {
	http.ResponseWriter
	status int
}

func (w *responseRecorder) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func requestLogger(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		recorder := &responseRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(recorder, r)
		logger.Info("request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", recorder.status,
			"duration_ms", time.Since(started).Milliseconds(),
			"remote_addr", r.RemoteAddr,
		)
	})
}
