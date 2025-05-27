package logger

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/sirupsen/logrus"
)

var Log = logrus.New()

type LokiHook struct {
	LokiURL string
	Labels  map[string]string
}

func InitLogger(lokiURL, appName, env string) {
	Log.SetOutput(os.Stdout)
	Log.SetFormatter(&logrus.JSONFormatter{TimestampFormat: time.RFC3339Nano})
	Log.SetLevel(logrus.InfoLevel)

	hook := &LokiHook{
		LokiURL: lokiURL,
		Labels: map[string]string{
			"app": appName,
			"env": env,
		},
	}
	Log.AddHook(hook)
}

func (h *LokiHook) Levels() []logrus.Level {
	return logrus.AllLevels
}

func (h *LokiHook) Fire(entry *logrus.Entry) error {
	line, err := entry.String()
	if err != nil {
		return err
	}

	// Convert labels to string-string map
	labels := make(map[string]string)
	for k, v := range h.Labels {
		labels[k] = v
	}
	for k, v := range entry.Data {
		labels[k] = fmt.Sprintf("%v", v)
	}

	stream := map[string]interface{}{
		"streams": []map[string]interface{}{
			{
				"stream": labels,
				"values": [][]string{
					{
						formatNano(entry.Time),
						line,
					},
				},
			},
		},
	}

	body, err := json.Marshal(stream)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", h.LokiURL, bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 5 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		log.Println("Failed to send log to Loki:", err.Error())
		return err
	}
	defer res.Body.Close()

	log.Println("Sending log to Loki:", h.LokiURL, "Status Code:", res.StatusCode)
	if res.StatusCode >= 400 {
		respBody, _ := io.ReadAll(res.Body)
		log.Printf("Loki response body: %s\n", string(respBody))
		return fmt.Errorf("bad status from Loki: %d", res.StatusCode)
	}

	return nil
}

// func formatNano(t time.Time) string {
// 	return time.Unix(0, t.UnixNano()).Format("2006-01-02T15:04:05.000000000Z")
// }

func formatNano(t time.Time) string {
	return fmt.Sprintf("%d", t.UnixNano())
}
