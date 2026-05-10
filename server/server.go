package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/jaredwarren/rpi_music/config"
	"github.com/jaredwarren/rpi_music/db"
	"github.com/jaredwarren/rpi_music/downloader"
	"github.com/jaredwarren/rpi_music/player"
)

// TemplateTag is the key used in template data maps for the CSRF field placeholder.
const TemplateTag = "csrfField"

// notifyEvent is sent to browser clients over SSE.
type notifyEvent struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

// Config provides the settings needed to start the HTTP server.
type Config struct {
	AppConfig    *config.Config
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	Db           db.DBer
	Logger       *slog.Logger
	Player       *player.Player
}

// HTMLServer is the running HTTP server lifecycle handle.
type HTMLServer struct {
	server *http.Server
	wg     sync.WaitGroup
	logger *slog.Logger
}

// StartHTTPServer registers routes and starts listening.
func StartHTTPServer(cfg *Config) (*HTMLServer, error) {
	cfg.Logger.Info("StartHTTPServer")

	s, err := New(cfg.AppConfig, cfg.Db, cfg.Player, cfg.Logger)
	if err != nil {
		return nil, fmt.Errorf("StartHTTPServer|New|%w", err)
	}

	mux := http.NewServeMux()

	// Static assets
	mux.Handle("GET /public/", http.StripPrefix("/public/", http.FileServer(http.Dir("./public"))))
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.Dir("./static"))))
	mux.Handle("GET /song_files/", http.StripPrefix("/song_files/", http.FileServer(http.Dir(cfg.AppConfig.Player.SongRoot))))
	mux.Handle("GET /thumb_files/", http.StripPrefix("/thumb_files/", http.FileServer(http.Dir(cfg.AppConfig.Player.ThumbRoot))))

	// Misc
	mux.HandleFunc("POST /log", s.Log)
	mux.HandleFunc("GET /stop", s.StopSongHandler)

	// Songs list
	mux.HandleFunc("GET /", s.ListSongHandler)
	mux.HandleFunc("GET /songs", s.ListSongHandler)

	// RFID management
	mux.HandleFunc("GET /rfids", s.EditRFIDSongFormHandler)
	mux.HandleFunc("DELETE /rfid/{rfid}/{song_id}", s.UnassignRFIDSongHandler)
	mux.HandleFunc("GET /rfid/{rfid}/json", s.JSONGetSongByRFID)

	// Song — new
	mux.HandleFunc("GET /song/new", s.NewSongFormHandler)
	mux.HandleFunc("POST /song/new", s.NewSongHandler)
	mux.HandleFunc("POST /song", s.NewSongHandler)

	// Song — download (async)
	mux.HandleFunc("POST /download", s.DownloadSong)

	// SSE for browser notifications
	mux.HandleFunc("GET /events", s.EventsSSE)

	// Song — RFID assignment
	mux.HandleFunc("GET /song/{song_id}/rfid", s.AssignRFIDToSongFormHandler)
	mux.HandleFunc("POST /song/{song_id}/rfid", s.AssignRFIDToSongHandler)

	// Song — actions
	mux.HandleFunc("DELETE /song/{song_id}", s.DeleteSongHandler)
	mux.HandleFunc("GET /song/{song_id}/play", s.PlaySongHandler)
	mux.HandleFunc("GET /song/{song_id}/delete", s.DeleteSongHandler)
	mux.HandleFunc("GET /song/{song_id}/stop", s.StopSongHandler)
	mux.HandleFunc("GET /song/{song_id}/play_video", s.PlayVideoHandler)
	mux.HandleFunc("GET /song/{song_id}/redownload", s.RedownloadSongAssetsHandler)
	mux.HandleFunc("GET /song/{song_id}/print", s.PrintHandler)
	mux.HandleFunc("GET /song/{song_id}/json", s.JSONHandler)
	mux.HandleFunc("GET /song/json", s.JSONHandler)

	// Config
	mux.HandleFunc("GET /config", s.ConfigFormHandler)
	mux.HandleFunc("POST /config", s.ConfigHandler)

	// Player
	mux.HandleFunc("GET /player/", s.PlayerHandler)

	// Admin
	mux.HandleFunc("GET /admin", s.RawHandler)
	mux.HandleFunc("GET /admin/song/{song_id}", s.AdminEditSong)
	mux.HandleFunc("POST /admin/song/{song_id}", s.AdminInsertSong)
	mux.HandleFunc("PATCH /admin/song/{song_id}", s.AdminUpdateSong)
	mux.HandleFunc("DELETE /admin/song/{song_id}", s.AdminDelete)

	// Raw debug view
	mux.HandleFunc("GET /raw", s.RawHandler)

	var handler http.Handler = mux
	handler = s.loggingMiddleware(handler)

	htmlServer := HTMLServer{
		logger: cfg.Logger,
		server: &http.Server{
			Addr:           cfg.AppConfig.Host,
			Handler:        handler,
			ReadTimeout:    cfg.ReadTimeout,
			WriteTimeout:   cfg.WriteTimeout,
			MaxHeaderBytes: 1 << 20,
		},
	}

	htmlServer.wg.Add(1)
	go func() {
		defer htmlServer.wg.Done()
		cfg.Logger.Info("HTTP server listening", "host", cfg.AppConfig.Host, "https", cfg.AppConfig.HTTPS)
		if cfg.AppConfig.HTTPS {
			_ = htmlServer.server.ListenAndServeTLS("localhost.crt", "localhost.key")
		} else {
			_ = htmlServer.server.ListenAndServe()
		}
	}()

	return &htmlServer, nil
}

func (s *Server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.logger.Debug("request", "uri", r.RequestURI)
		next.ServeHTTP(w, r)
	})
}

// StopHTTPServer gracefully shuts down the HTTP server.
func (h *HTMLServer) StopHTTPServer() error {
	const timeout = 5 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	h.logger.Info("Stopping HTTP server...")

	if err := h.server.Shutdown(ctx); err != nil {
		if err := h.server.Close(); err != nil {
			h.logger.Error("force-close HTTP server", "err", err)
			return err
		}
	}
	h.wg.Wait()
	h.logger.Info("HTTP server stopped")
	return nil
}

// Server is the application handler with all dependencies injected.
type Server struct {
	cfg          *config.Config
	db           db.DBer
	logger       *slog.Logger
	downloader   downloader.Downloader
	player       *player.Player
	templates    map[string]*template.Template
	notifySubsMu sync.Mutex
	notifySubs   map[chan notifyEvent]struct{}
}

// New constructs a Server with all dependencies.
func New(cfg *config.Config, database db.DBer, p *player.Player, l *slog.Logger) (*Server, error) {
	var dl downloader.Downloader
	if cfg.Downloader == "ytdl" {
		dl = &downloader.YoutubeDownloader{
			SongRoot:  cfg.Player.SongRoot,
			ThumbRoot: cfg.Player.ThumbRoot,
		}
		l.Info("using 'ytdl' downloader")
	} else {
		dlCfg := &downloader.YoutubeDLConfig{
			SongRoot:  cfg.Player.SongRoot,
			ThumbRoot: cfg.Player.ThumbRoot,
		}
		ytdl := downloader.NewYoutubeDLDownloader(dlCfg)
		if err := ytdl.EnsureAvailable(); err != nil {
			return nil, fmt.Errorf("server: youtube-dl backend unavailable: %w", err)
		}
		l.Info("using 'youtube-dl' downloader", "backend", ytdl.BackendDescription())
		dl = ytdl
	}

	srv := &Server{
		cfg:        cfg,
		db:         database,
		logger:     l,
		downloader: dl,
		player:     p,
		notifySubs: make(map[chan notifyEvent]struct{}),
	}
	srv.templates = srv.loadTemplates()
	return srv, nil
}

func (s *Server) loadTemplates() map[string]*template.Template {
	layout := "templates/layout.html"
	m := map[string]*template.Template{
		"index":         template.Must(template.ParseFiles("templates/index.html", layout)),
		"editSong":      template.Must(template.ParseFiles("templates/edit_song.html", layout)),
		"newSong":       template.Must(template.New("base").ParseFiles("templates/new_song.html", layout)),
		"playVideo":     template.Must(template.New("base").Funcs(template.FuncMap{}).ParseFiles("templates/play_video.html", layout)),
		"editRfid":      template.Must(template.ParseFiles("templates/edit_rfid.html", layout)),
		"assignSong":    template.Must(template.ParseFiles("templates/assign_song.html", layout)),
		"raw":           template.Must(template.ParseFiles("templates/raw.html", layout)),
		"admin":         template.Must(template.ParseFiles("templates/admin.html", layout)),
		"adminEditSong": template.Must(template.ParseFiles("templates/editSong.html", layout)),
		"player":        template.Must(template.New("base").ParseFiles("templates/player.html", layout)),
		"print":         template.Must(template.New("base").ParseFiles("templates/print.html", layout)),
	}
	cfgMap := s.cfg.ToMap()
	configFuncs := template.FuncMap{
		"ConfigString": func(feature string) template.HTML {
			v := fmt.Sprint(cfgMap[feature])
			return template.HTML(fmt.Sprintf(`<label for="%s">%s</label><input id="%s" type="text" value="%s" name="%s">`, feature, feature, feature, v, feature))
		},
		"ConfigBool": func(feature string) template.HTML {
			checked := ""
			if v, ok := cfgMap[feature].(bool); ok && v {
				checked = "checked"
			}
			return template.HTML(fmt.Sprintf(`<input type="checkbox" name="%s" %s><i class="form-icon"></i> %s`, feature, checked, feature))
		},
		"ConfigInt": func(feature string) template.HTML {
			v := fmt.Sprint(cfgMap[feature])
			return template.HTML(fmt.Sprintf(`<label for="%s">%s</label><input class="form-input" id="%s" type="number" placeholder="00" value="%s" name="%s">`, feature, feature, feature, v, feature))
		},
	}
	m["config"] = template.Must(template.New("base").Funcs(configFuncs).ParseFiles("templates/config.html", layout))
	return m
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
	}()

	go func() {
		<-r.Context().Done()
		s.notifySubsMu.Lock()
		delete(s.notifySubs, ch)
		close(ch)
		s.notifySubsMu.Unlock()
	}()

	enc := json.NewEncoder(w)
	for ev := range ch {
		fmt.Fprint(w, "event: notification\ndata: ")
		_ = enc.Encode(ev)
		fmt.Fprint(w, "\n")
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
	}
}

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

func (s *Server) push(w http.ResponseWriter, resource string) {
	if pusher, ok := w.(http.Pusher); ok {
		if err := pusher.Push(resource, nil); err != nil {
			s.logger.Error("push error", "err", err)
		}
	}
}

// Message is the shape of log messages POSTed from JavaScript clients.
type Message struct {
	Command string            `json:"command"`
	Data    map[string]string `json:"data"`
	Error   string            `json:"error"`
}

func (s *Server) Log(w http.ResponseWriter, r *http.Request) {
	msg := &Message{}
	if err := json.NewDecoder(r.Body).Decode(msg); err != nil {
		s.logger.Error("body parse error", "err", err)
		s.httpError(w, fmt.Errorf("log|%w", err), http.StatusInternalServerError)
		return
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
