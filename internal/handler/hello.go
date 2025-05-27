package handler

import (
	"fmt"
	"net/http"

	"github.com/snykk/simple-grafana-loki/internal/logger"
)

func HelloHandler(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("name")
	if name == "" {
		name = "world"
	}

	logger.Log.WithFields(map[string]interface{}{
		"path":   r.URL.Path,
		"method": r.Method,
		"name":   name,
	}).Info("Processing /hello")

	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"message":"Hello, %s!"}`, name)
}
