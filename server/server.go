package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"github.com/jaredwarren/rpi_music/db"
	"github.com/jaredwarren/rpi_music/downloader"
	"github.com/jaredwarren/rpi_music/log"
	"github.com/jaredwarren/rpi_music/model"
	"github.com/jaredwarren/rpi_music/player"
	"github.com/spf13/viper"
)

// Config provides basic configuration
type Config struct {
	Host         string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	Db           db.DBer
	Logger       log.Logger
}

// HTMLServer represents the web service that serves up HTML
type HTMLServer struct {
	server *http.Server
	wg     sync.WaitGroup
	logger log.Logger
}

// Start launches the HTML Server
func StartHTTPServer(cfg *Config) *HTMLServer {
	// Setup Context
	_, cancel := context.WithCancel(context.Background())
	defer cancel()

	// init server
	s := New(cfg.Db, cfg.Logger)

	// Setup Handlers
	r := mux.NewRouter()
	r.Use(s.loggingMiddleware)
	r.Use(mux.CORSMethodMiddleware(r))

	// Public Methods
	r.HandleFunc("/login", s.LoginForm).Methods(http.MethodGet, http.MethodOptions)
	r.HandleFunc("/logout", s.Logout).Methods(http.MethodGet, http.MethodOptions)
	r.HandleFunc("/login", s.Login).Methods(http.MethodPost, http.MethodOptions)
	r.PathPrefix("/public/").Handler(http.StripPrefix("/public/", http.FileServer(http.Dir("./public"))))

	// login-required methods
	sub := r.PathPrefix("/").Subrouter()
	// sub.Use(s.requireLoginMiddleware)

	sub.HandleFunc("/log", s.Log)
	sub.HandleFunc("/stop", s.StopSongHandler)

	// list songs
	sub.HandleFunc("/", s.ListSongHandler).Methods(http.MethodGet)
	sub.HandleFunc("/songs", s.ListSongHandler).Methods(http.MethodGet)
	sub.HandleFunc("/rfids", s.EditRFIDSongFormHandler).Methods(http.MethodGet)
	sub.HandleFunc("/rfid/{rfid}/{song_id}", s.UnassignRFIDSongHandler).Methods(http.MethodDelete)

	// Song
	ssub := sub.PathPrefix("/song").Subrouter()
	ssub.HandleFunc("", s.NewSongHandler).Methods(http.MethodPost)

	// Download Song
	ssub.HandleFunc(fmt.Sprintf("/%s", model.NewSongID), s.NewSongFormHandler).Methods(http.MethodGet) // GET /song/new
	ssub.HandleFunc(fmt.Sprintf("/%s", model.NewSongID), s.NewSongHandler).Methods(http.MethodPost)    // POST /song/new

	// Assign rfid to song
	ssub.HandleFunc("/{song_id}/rfid", s.AssignRFIDToSongFormHandler).Methods(http.MethodGet)
	ssub.HandleFunc("/{song_id}/rfid", s.AssignRFIDToSongHandler).Methods(http.MethodPost)

	// Play
	// ssub.HandleFunc("/{song_id}", s.EditSongFormHandler).Methods(http.MethodGet)
	// ssub.HandleFunc("/{song_id}", s.UpdateSongHandler).Methods(http.MethodPut, http.MethodPost)
	ssub.HandleFunc("/{song_id}", s.DeleteSongHandler).Methods(http.MethodDelete)
	ssub.HandleFunc("/{song_id}/play", s.PlaySongHandler).Methods(http.MethodGet)
	ssub.HandleFunc("/{song_id}/delete", s.DeleteSongHandler).Methods(http.MethodGet)
	ssub.HandleFunc("/{song_id}/stop", s.StopSongHandler).Methods(http.MethodGet)
	ssub.HandleFunc("/{song_id}/play_video", s.PlayVideoHandler).Methods(http.MethodGet)
	ssub.HandleFunc("/{song_id}/print", s.PrintHandler).Methods(http.MethodGet)
	ssub.HandleFunc("/{song_id}/json", s.JSONHandler).Methods(http.MethodGet)
	ssub.HandleFunc("/json", s.JSONHandler).Methods(http.MethodGet)

	// Config Endpoints
	csub := sub.PathPrefix("/config").Subrouter()
	csub.HandleFunc("", s.ConfigFormHandler).Methods(http.MethodGet)
	csub.HandleFunc("", s.ConfigHandler).Methods(http.MethodPost)

	// Player Endpoints
	psub := sub.PathPrefix("/player").Subrouter()
	psub.HandleFunc("/", s.PlayerHandler).Methods(http.MethodGet)

	// Static files
	sub.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("./static"))))
	sub.PathPrefix("/song_files/").Handler(http.StripPrefix("/song_files/", http.FileServer(http.Dir(viper.GetString("player.song_root")))))
	sub.PathPrefix("/thumb_files/").Handler(http.StripPrefix("/thumb_files/", http.FileServer(http.Dir(viper.GetString("player.thumb_root")))))

	rawsub := sub.PathPrefix("/raw").Subrouter()
	rawsub.HandleFunc("", s.RawHandler)

	// Handle everything else
	r.PathPrefix("/").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/songs", http.StatusFound)
	})

	// Create the HTML Server
	htmlServer := HTMLServer{
		logger: cfg.Logger,
		server: &http.Server{
			Addr:           cfg.Host,
			Handler:        r,
			ReadTimeout:    cfg.ReadTimeout,
			WriteTimeout:   cfg.WriteTimeout,
			MaxHeaderBytes: 1 << 20,
		},
	}

	// Start the listener
	htmlServer.wg.Add(1)
	go func() {
		cfg.Logger.Info("Starting HTTP server", log.Any("host", cfg.Host), log.Any("https", viper.GetBool("https")))
		if viper.GetBool("https") {
			htmlServer.server.ListenAndServeTLS("localhost.crt", "localhost.key")
		} else {
			htmlServer.server.ListenAndServe()
		}
		htmlServer.wg.Done()
	}()

	return &htmlServer
}

func (s *Server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// s.logger.Debug(r.RequestURI, log.Any("r", r))
		s.logger.Debug("[request] - " + r.RequestURI)
		// Call the next handler, which can be another middleware in the chain, or the final handler.
		next.ServeHTTP(w, r)
	})
}

// Stop turns off the HTML Server
func (htmlServer *HTMLServer) StopHTTPServer() error {
	// Create a context to attempt a graceful 5 second shutdown.
	const timeout = 5 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	htmlServer.logger.Info("Stopping HTTP service...")

	// Attempt the graceful shutdown by closing the listener
	// and completing all inflight requests
	if err := htmlServer.server.Shutdown(ctx); err != nil {
		// Looks like we timed out on the graceful shutdown. Force close.
		if err := htmlServer.server.Close(); err != nil {
			htmlServer.logger.Error("error stopping HTML service", log.Error(err))
			return err
		}
	}

	// Wait for the listener to report that it is closed.
	htmlServer.wg.Wait()
	htmlServer.logger.Info("HTTP service stopped")
	return nil
}

type Server struct {
	db         db.DBer
	logger     log.Logger
	downloader downloader.Downloader
}

func New(db db.DBer, l log.Logger) *Server {
	var dl downloader.Downloader
	if viper.GetString("downloader") == "ytdl" {
		dl = &downloader.YoutubeDownloader{}
		l.Info("using 'ytdl' downloader")
	} else {
		dl = &downloader.YoutubeDLDownloader{}
		l.Info("using 'youtube-dl' downloader")
	}

	return &Server{
		db:         db,
		logger:     l, // TODO: move this to context
		downloader: dl,
	}
}

// Render a template, or server error.
func (s *Server) render(w http.ResponseWriter, r *http.Request, tpl *template.Template, data interface{}) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	buf := new(bytes.Buffer)
	if err := tpl.Execute(buf, data); err != nil {
		s.logger.Error("template render error", log.Error(err), log.Any("data", data))
		return
	}
	w.Write(buf.Bytes())
}

// Push the given resource to the client.
func (s *Server) push(w http.ResponseWriter, resource string) {
	pusher, ok := w.(http.Pusher)
	if ok {
		err := pusher.Push(resource, nil)
		if err != nil {
			s.logger.Error("push error", log.Error(err))
		}
		return
	}
}

func (s *Server) PlayerHandler(w http.ResponseWriter, r *http.Request) {
	cp := player.GetPlayer()
	song := player.GetPlaying()

	fullData := map[string]interface{}{
		"Player":    cp,
		"Song":      song,
		TemplateTag: s.GetToken(w, r),
	}
	files := []string{
		"templates/player.html",
		"templates/layout.html",
	}
	tpl := template.Must(template.New("base").ParseFiles(files...))
	s.render(w, r, tpl, fullData)
}

func (s *Server) PlaySongHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	songID := vars["song_id"]

	song, err := s.db.GetSong(songID)
	if err != nil {
		s.httpError(w, fmt.Errorf("PlaySongHandler|db.View|%w", err), http.StatusInternalServerError)
		return
	}
	player.Beep()
	err = player.Play(song)
	if err != nil {
		// TODO: check if err is user error or system error
		s.httpError(w, fmt.Errorf("PlaySongHandler|player.Play|%w", err), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/songs", http.StatusFound)
}

func (s *Server) StopSongHandler(w http.ResponseWriter, r *http.Request) {
	player.Stop()
	http.Redirect(w, r, "/songs", http.StatusFound)
}

type Message struct {
	Command string            `json:"command"`
	Data    map[string]string `json:"data"`
	Error   string            `json:"error"`
}

func (s *Server) Log(w http.ResponseWriter, r *http.Request) {
	msg := &Message{}
	err := json.NewDecoder(r.Body).Decode(&msg)
	if err != nil {
		s.logger.Error("body parse error", log.Error(err))
		s.httpError(w, fmt.Errorf("log|%w", err), http.StatusInternalServerError)
		return
	}
	if level, ok := msg.Data["level"]; ok {
		switch level {
		case "warn":
			s.logger.Warn("log", log.Any("message", msg))
		case "err":
			s.logger.Error("log", log.Any("message", msg))
		default:
			s.logger.Info("log", log.Any("message", msg))
		}
	} else {
		s.logger.Info("log", log.Any("message", msg))
	}
}
