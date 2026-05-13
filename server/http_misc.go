package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"os"
)

// render executes a template into a buffer then writes to w.
func (s *Server) render(w http.ResponseWriter, r *http.Request, tpl *template.Template, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	buf := new(bytes.Buffer)
	if err := tpl.Execute(buf, data); err != nil {
		s.logger.Error("template render error", "err", err)
		return
	}
	_, _ = w.Write(buf.Bytes())
}

// Message is the shape of log messages POSTed from JavaScript clients.
type Message struct {
	Command string            `json:"command"`
	Data    map[string]string `json:"data"`
	Error   string            `json:"error"`
}

func (s *Server) LogE(w http.ResponseWriter, r *http.Request) error {
	msg := &Message{}
	if err := json.NewDecoder(r.Body).Decode(msg); err != nil {
		s.logger.Error("body parse error", "err", err)
		return asHTTPError(http.StatusInternalServerError, fmt.Errorf("log|%w", err))
	}
	if level, ok := msg.Data["level"]; ok {
		switch level {
		case "warn":
			s.logger.Warn("log", "message", msg)
		case "err":
			s.logger.Error("log", "message", msg)
		default:
			s.logger.Info("log", "message", msg)
		}
	} else {
		s.logger.Info("log", "message", msg)
	}
	return nil
}

func (s *Server) Log(w http.ResponseWriter, r *http.Request) {
	if err := s.LogE(w, r); err != nil {
		var httpErr *HTTPError
		if errors.As(err, &httpErr) {
			s.httpError(w, httpErr.Err, httpErr.Code)
			return
		}
		s.httpError(w, err, http.StatusInternalServerError)
	}
}

// Logs is intentionally removed — file-based log tailing is no longer supported.
func (s *Server) Logs(w http.ResponseWriter, _ *http.Request) {
	http.Error(w, "log tailing not available", http.StatusNotImplemented)
}

// readDir lists non-directory entries in dir, returning an empty slice on error.
func readDir(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			out = append(out, e.Name())
		}
	}
	return out
}
