package server

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/jaredwarren/rpi_music/config"
	"github.com/jaredwarren/rpi_music/player"
)

// Config provides the settings needed to start the HTTP server.
type Config struct {
	AppConfig    *config.Config
	Context      context.Context
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	Db           Store
	Logger       *slog.Logger
	Player       *player.Player
}

// HTMLServer is the running HTTP server lifecycle handle.
type HTMLServer struct {
	server *http.Server
	wg     sync.WaitGroup
	logger *slog.Logger
	cancel context.CancelFunc
}

// StartHTTPServer registers routes and starts listening.
func StartHTTPServer(cfg *Config) (*HTMLServer, error) {
	cfg.Logger.Info("StartHTTPServer")

	serverCtx := cfg.Context
	if serverCtx == nil {
		serverCtx = context.Background()
	}
	serverCtx, cancel := context.WithCancel(serverCtx)

	s, err := New(serverCtx, cfg.AppConfig, cfg.Db, cfg.Player, cfg.Logger)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("StartHTTPServer|New|%w", err)
	}

	mux := http.NewServeMux()
	s.registerRoutes(mux)

	var handler http.Handler = mux
	handler = s.loggingMiddleware(handler)

	htmlServer := HTMLServer{
		logger: cfg.Logger,
		cancel: cancel,
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

func (s *Server) registerRoutes(mux *http.ServeMux) {
	// Static assets
	mux.Handle("GET /public/", http.StripPrefix("/public/", http.FileServer(http.Dir("./public"))))
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.Dir("./static"))))
	mux.Handle("GET /song_files/", http.StripPrefix("/song_files/", http.FileServer(http.Dir(s.cfg.Player.SongRoot))))
	mux.Handle("GET /thumb_files/", http.StripPrefix("/thumb_files/", http.FileServer(http.Dir(s.cfg.Player.ThumbRoot))))

	// Misc
	mux.HandleFunc("POST /log", s.withError(s.LogE))
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
	mux.HandleFunc("POST /song/new", s.withError(s.NewSongHandlerE))
	mux.HandleFunc("POST /song", s.withError(s.NewSongHandlerE))

	// Song — download (async)
	mux.HandleFunc("POST /download", s.withError(s.DownloadSongE))

	// SSE for browser notifications
	mux.HandleFunc("GET /events", s.EventsSSE)

	// Song — RFID assignment
	mux.HandleFunc("GET /song/{song_id}/rfid", s.AssignRFIDToSongFormHandler)
	mux.HandleFunc("POST /song/{song_id}/rfid", s.AssignRFIDToSongHandler)

	// Song — actions
	mux.HandleFunc("DELETE /song/{song_id}", s.withError(s.DeleteSongHandlerE))
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
	mux.HandleFunc("POST /config", s.withError(s.ConfigHandlerE))

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
	if h.cancel != nil {
		h.cancel()
	}

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
