package server

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// notifyEvent is sent to browser clients over SSE.
type notifyEvent struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

// notifyBroadcast sends a notification to all connected SSE clients.
func (s *Server) notifyBroadcast(title, body string) {
	ev := notifyEvent{Title: title, Body: body}
	s.notifySubsMu.Lock()
	defer s.notifySubsMu.Unlock()
	for ch := range s.notifySubs {
		select {
		case ch <- ev:
		default:
		}
	}
}

// EventsSSE streams server-sent events for browser Web Notifications.
func (s *Server) EventsSSE(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}

	ch := make(chan notifyEvent, 8)
	s.notifySubsMu.Lock()
	s.notifySubs[ch] = struct{}{}
	s.notifySubsMu.Unlock()

	defer func() {
		s.notifySubsMu.Lock()
		delete(s.notifySubs, ch)
		s.notifySubsMu.Unlock()
		close(ch)
	}()

	enc := json.NewEncoder(w)
	for {
		select {
		case <-r.Context().Done():
			return
		case <-s.ctx.Done():
			return
		case ev := <-ch:
			_, _ = fmt.Fprint(w, "event: notification\ndata: ")
			_ = enc.Encode(ev)
			_, _ = fmt.Fprint(w, "\n")
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}
		}
	}
}
