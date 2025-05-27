package logger

import (
	"fmt"
	"net/url"
	"os"
	"time"

	"github.com/grafana/loki-client-go/loki"
	"github.com/grafana/loki-client-go/pkg/backoff"
	"github.com/grafana/loki-client-go/pkg/urlutil"
	"github.com/prometheus/common/model"
	"github.com/sirupsen/logrus"
)

var Log = logrus.New()

// logrusToKitLogger adapts logrus.Logger to go-kit log.Logger
type logrusToKitLogger struct {
	logrus.FieldLogger
}

func (l logrusToKitLogger) Log(keyvals ...interface{}) error {
	fields := make(logrus.Fields)
	for i := 0; i < len(keyvals); i += 2 {
		if i+1 < len(keyvals) {
			fields[fmt.Sprint(keyvals[i])] = keyvals[i+1]
		} else {
			fields[fmt.Sprint(keyvals[i])] = "MISSING"
		}
	}
	l.WithFields(fields).Info()
	return nil
}

type LokiHook struct {
	client *loki.Client
	labels model.LabelSet
}

func InitLogger(lokiURL, appName, env string) error {
	Log.SetOutput(os.Stdout)
	Log.SetFormatter(&logrus.JSONFormatter{TimestampFormat: time.RFC3339Nano})
	Log.SetLevel(logrus.InfoLevel)

	// Create Loki client configuration
	cfg := loki.Config{
		URL: urlutil.URLValue{
			URL: mustParseURL(lokiURL),
		},
		BatchWait: 1 * time.Second, // Wait up to 1s before sending a batch
		BatchSize: 1024 * 1024,     // Send batch when it reaches 1MB
		Timeout:   5 * time.Second, // Timeout for sending logs
		BackoffConfig: backoff.BackoffConfig{
			MinBackoff: 100 * time.Millisecond,
			MaxBackoff: 5 * time.Second,
			MaxRetries: 5,
		},
	}

	// Create adapter for go-kit logger
	kitLogger := logrusToKitLogger{Log}

	// Create Loki client
	client, err := loki.NewWithLogger(cfg, kitLogger)
	if err != nil {
		return fmt.Errorf("failed to create Loki client: %w", err)
	}

	// Create and add hook
	hook := &LokiHook{
		client: client,
		labels: model.LabelSet{
			"app": model.LabelValue(appName),
			"env": model.LabelValue(env),
		},
	}
	Log.AddHook(hook)

	return nil
}

func mustParseURL(rawURL string) *url.URL {
	u, err := url.Parse(rawURL)
	if err != nil {
		panic(fmt.Sprintf("failed to parse Loki URL: %v", err))
	}
	return u
}

func (h *LokiHook) Levels() []logrus.Level {
	return logrus.AllLevels
}

func (h *LokiHook) Fire(entry *logrus.Entry) error {
	// Start with our base labels
	labels := make(model.LabelSet, len(h.labels)+len(entry.Data))
	for k, v := range h.labels {
		labels[k] = v
	}

	// Add logrus fields as labels
	for k, v := range entry.Data {
		// Skip special fields that are not meant to be labels
		if k == "time" || k == "msg" || k == "level" {
			continue
		}
		labels[model.LabelName(k)] = model.LabelValue(fmt.Sprintf("%v", v))
	}

	// Convert log entry to string
	line, err := entry.String()
	if err != nil {
		return fmt.Errorf("failed to format log entry: %w", err)
	}

	// Send to Loki
	h.client.Handle(labels, entry.Time, line)
	return nil
}

// Close should be called before application exit to flush any pending logs
func Close() error {
	if Log != nil {
		if hooks := Log.Hooks; hooks != nil {
			for _, levelHooks := range hooks {
				for _, hook := range levelHooks {
					if lokiHook, ok := hook.(*LokiHook); ok && lokiHook.client != nil {
						lokiHook.client.Stop()
					}
				}
			}
		}
	}
	return nil
}
