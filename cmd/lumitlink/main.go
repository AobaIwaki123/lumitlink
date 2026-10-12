// Package main starts the lumitlink HTTP API and browserless Lit.link adapter.
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
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/AobaIwaki123/lumitlink/internal/litlink"
)

//go:embed web/index.html openapi.yaml
var assets embed.FS

const (
	serviceName = "lumitlink"
	trueValues  = "1,true,yes,on"
)

type config struct {
	host      string
	port      int
	cacheTTL  time.Duration
	timeout   time.Duration
	userAgent string
	logLevel  slog.Level
}

type application struct {
	service *litlink.ProfileService
	logger  *slog.Logger
}

type trackingWriter struct {
	http.ResponseWriter
	status  int
	started time.Time
}

func (w *trackingWriter) WriteHeader(status int) {
	if w.status != 0 {
		return
	}
	w.status = status
	w.Header().Set("X-Runtime-Ms", strconv.FormatInt(time.Since(w.started).Milliseconds(), 10))
	w.ResponseWriter.WriteHeader(status)
}

func (w *trackingWriter) Write(body []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.ResponseWriter.Write(body)
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	cfg := loadConfig()
	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: cfg.logLevel}))
	client := litlink.NewClient(cfg.timeout, cfg.userAgent)
	application := &application{service: litlink.NewProfileService(client, cfg.cacheTTL), logger: logger}
	server := &http.Server{
		Addr:              net.JoinHostPort(cfg.host, strconv.Itoa(cfg.port)),
		Handler:           application,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       20 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	shutdownCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	go func() {
		<-shutdownCtx.Done()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(ctx); err != nil {
			logger.Error("server shutdown failed", "error", err)
		}
	}()

	logger.Info("server listening", "service", serviceName, "addr", server.Addr, "cache_ttl", cfg.cacheTTL.String(), "upstream_timeout", cfg.timeout.String())
	err := server.ListenAndServe()
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("server failed: %w", err)
	}
	return nil
}

func loadConfig() config {
	return config{
		host:      envString("LUMITLINK_HOST", "0.0.0.0"),
		port:      envInt("LUMITLINK_PORT", 8080, 1, 65535),
		cacheTTL:  envDuration("LUMITLINK_CACHE_TTL", 10*time.Minute),
		timeout:   envDuration("LUMITLINK_TIMEOUT", litlink.DefaultTimeout),
		userAgent: envString("LUMITLINK_USER_AGENT", litlink.DefaultUserAgent),
		logLevel:  envLogLevel("LOG_LEVEL", slog.LevelInfo),
	}
}

func envString(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}

func envInt(name string, fallback, minimum, maximum int) int {
	value, err := strconv.Atoi(os.Getenv(name))
	if err != nil || value < minimum || value > maximum {
		return fallback
	}
	return value
}

func envDuration(name string, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil || parsed < 0 {
		return fallback
	}
	return parsed
}

func envLogLevel(name string, fallback slog.Level) slog.Level {
	switch strings.ToUpper(strings.TrimSpace(os.Getenv(name))) {
	case "DEBUG":
		return slog.LevelDebug
	case "WARN", "WARNING":
		return slog.LevelWarn
	case "ERROR":
		return slog.LevelError
	default:
		return fallback
	}
}

func (a *application) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	started := time.Now()
	tracked := &trackingWriter{ResponseWriter: writer, started: started}
	cacheState := "-"
	defer func() {
		a.logger.Info("request",
			"method", request.Method,
			"path", request.URL.RequestURI(),
			"status", tracked.status,
			"duration_ms", time.Since(started).Milliseconds(),
			"cache", cacheState,
			"remote", request.RemoteAddr,
		)
	}()

	path := strings.TrimSuffix(request.URL.Path, "/")
	if path == "" {
		path = "/"
	}
	if request.Method == http.MethodOptions {
		setCORS(tracked)
		tracked.WriteHeader(http.StatusNoContent)
		return
	}
	if path == "/" || path == "/index.html" {
		a.serveAsset(tracked, request, "web/index.html", "text/html; charset=utf-8")
		return
	}
	if path == "/openapi.yaml" {
		a.serveAsset(tracked, request, "openapi.yaml", "application/yaml; charset=utf-8")
		return
	}
	if path == "/healthz" || path == "/api/health" {
		if request.Method != http.MethodGet {
			methodNotAllowed(tracked, http.MethodGet)
			return
		}
		writeJSON(tracked, http.StatusOK, map[string]any{"status": "ok", "service": serviceName})
		return
	}

	username, ok := profileUsername(path)
	if !ok {
		writeAPIError(tracked, http.StatusNotFound, "not_found", "endpoint not found")
		return
	}
	setCORS(tracked)
	if request.Method != http.MethodGet {
		methodNotAllowed(tracked, http.MethodGet)
		return
	}
	validated, err := litlink.ValidateUsername(username)
	if err != nil {
		writeAPIError(tracked, http.StatusBadRequest, "invalid_username", err.Error())
		return
	}
	refresh := isTrue(request.URL.Query().Get("refresh"))
	response, state, err := a.service.Get(request.Context(), validated, refresh)
	if err != nil {
		var fetchErr *litlink.FetchError
		if errors.As(err, &fetchErr) {
			writeAPIError(tracked, fetchErr.HTTPStatus, "upstream_fetch_failed", fetchErr.Error())
		} else {
			writeAPIError(tracked, http.StatusBadGateway, "upstream_fetch_failed", err.Error())
		}
		return
	}
	cacheState = string(state)
	tracked.Header().Set("X-Cache", cacheState)
	writeJSON(tracked, http.StatusOK, response)
}

func profileUsername(path string) (string, bool) {
	prefixes := []string{"/api/v1/profiles/", "/v1/profiles/"}
	for _, prefix := range prefixes {
		if !strings.HasPrefix(path, prefix) {
			continue
		}
		encoded := strings.TrimPrefix(path, prefix)
		if encoded == "" || strings.Contains(encoded, "/") {
			return "", false
		}
		username, err := url.PathUnescape(encoded)
		return username, err == nil
	}
	return "", false
}

func isTrue(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	for _, accepted := range strings.Split(trueValues, ",") {
		if value == accepted {
			return true
		}
	}
	return false
}

func setCORS(writer http.ResponseWriter) {
	writer.Header().Set("Access-Control-Allow-Origin", "*")
	writer.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	writer.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	writer.Header().Set("Access-Control-Expose-Headers", "X-Cache, X-Runtime-Ms")
}

func methodNotAllowed(writer http.ResponseWriter, allowed string) {
	writer.Header().Set("Allow", allowed)
	writeAPIError(writer, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
}

func writeAPIError(writer http.ResponseWriter, status int, code, message string) {
	setCORS(writer)
	writeJSON(writer, status, map[string]any{"error": map[string]string{"code": code, "message": message}})
}

func writeJSON(writer http.ResponseWriter, status int, payload any) {
	body, err := json.Marshal(payload)
	if err != nil {
		status = http.StatusInternalServerError
		body = []byte(`{"error":{"code":"serialization_failed","message":"could not encode response"}}`)
	}
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.Header().Set("Content-Length", strconv.Itoa(len(body)))
	writer.WriteHeader(status)
	_, _ = writer.Write(body)
}

func (a *application) serveAsset(writer http.ResponseWriter, request *http.Request, name, contentType string) {
	if request.Method != http.MethodGet && request.Method != http.MethodHead {
		methodNotAllowed(writer, http.MethodGet)
		return
	}
	body, err := fs.ReadFile(assets, name)
	if err != nil {
		writeAPIError(writer, http.StatusInternalServerError, "asset_not_found", "embedded asset not found")
		return
	}
	writer.Header().Set("Content-Type", contentType)
	writer.Header().Set("Content-Length", strconv.Itoa(len(body)))
	writer.WriteHeader(http.StatusOK)
	if request.Method == http.MethodGet {
		_, _ = writer.Write(body)
	}
}
