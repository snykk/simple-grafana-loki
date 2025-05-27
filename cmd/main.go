package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/snykk/simple-grafana-loki/internal/handler"
	"github.com/snykk/simple-grafana-loki/internal/logger"

	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()

	port := getEnv("PORT", "8080")
	lokiURL := getEnv("LOKI_URL", "http://localhost:3100/loki/api/v1/push")
	appName := getEnv("APP_NAME", "simple-api")
	env := getEnv("ENV", "dev")

	// Initialize logger
	if err := logger.InitLogger(lokiURL, appName, env); err != nil {
		logger.Log.Fatalf("Failed to initialize logger: %v", err)
	}
	defer logger.Close()

	mux := http.NewServeMux()
	mux.HandleFunc("/hello", handler.HelloHandler)

	srv := &http.Server{
		Addr:    ":" + port,
		Handler: loggingMiddleware(mux),
	}

	// Graceful shutdown
	go func() {
		logger.Log.Infof("Starting server on :%s", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Log.Fatalf("ListenAndServe: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Log.Info("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		logger.Log.Fatalf("Server forced to shutdown: %v", err)
	}
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

// Logging middleware
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		logger.Log.WithFields(map[string]interface{}{
			"method": r.Method,
			"path":   r.URL.Path,
			"time":   time.Since(start).String(),
		}).Info("Handled request")
	})
}
