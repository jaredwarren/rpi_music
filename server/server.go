package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"github.com/jaredwarren/rpi_music/db"
	"github.com/jaredwarren/rpi_music/downloader"
	"github.com/jaredwarren/rpi_music/log"
	"github.com/jaredwarren/rpi_music/model"
	"github.com/spf13/viper"
)

// TemplateTag is the key used in template data for the CSRF field (empty when auth is disabled).
const TemplateTag = "csrfField"

// getCSRFField returns an empty HTML fragment so templates that reference .csrfField still render.
func (s *Server) getCSRFField() template.HTML {
	return template.HTML("")
}

// notifyEvent is sent to browser clients over SSE for Web Notifications (e.g. Chrome on Android).
type notifyEvent struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

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
	cfg.Logger.Info("->StartHTTPServer")

	s := New(cfg.Db, cfg.Logger)

	// Setup Handlers
	r := mux.NewRouter()
	r.Use(s.loggingMiddleware)
	r.Use(mux.CORSMethodMiddleware(r))

	// Public
	r.PathPrefix("/public/").Handler(http.StripPrefix("/public/", http.FileServer(http.Dir("./public"))))

	// login-required methods
	sub := r.PathPrefix("/").Subrouter()
	// sub.Use(s.requireLoginMiddleware)

	sub.HandleFunc("/log", s.Log)
	sub.HandleFunc("/logs", s.Logs)
	sub.HandleFunc("/stop", s.StopSongHandler)

	// list songs
	sub.HandleFunc("/", s.ListSongHandler).Methods(http.MethodGet)
	sub.HandleFunc("/songs", s.ListSongHandler).Methods(http.MethodGet)
	sub.HandleFunc("/rfids", s.EditRFIDSongFormHandler).Methods(http.MethodGet)
	sub.HandleFunc("/rfid/{rfid}/{song_id}", s.UnassignRFIDSongHandler).Methods(http.MethodDelete)
	sub.HandleFunc("/{rfid}/json", s.JSONGetSongByRFID).Methods(http.MethodGet)

	// Song
	ssub := sub.PathPrefix("/song").Subrouter()
	ssub.HandleFunc("", s.NewSongHandler).Methods(http.MethodPost)

	// Download Song
	ssub.HandleFunc(fmt.Sprintf("/%s", model.NewSongID), s.NewSongFormHandler).Methods(http.MethodGet) // GET /song/new
	ssub.HandleFunc(fmt.Sprintf("/%s", model.NewSongID), s.NewSongHandler).Methods(http.MethodPost)    // POST /song/new
	sub.HandleFunc("/download", s.DownloadSong).Methods(http.MethodPost)                               // Raw download
	sub.HandleFunc("/events", s.EventsSSE).Methods(http.MethodGet)                                     // SSE for browser notifications

	// Assign rfid to song
	ssub.HandleFunc("/{song_id}/rfid", s.AssignRFIDToSongFormHandler).Methods(http.MethodGet)
	ssub.HandleFunc("/{song_id}/rfid", s.AssignRFIDToSongHandler).Methods(http.MethodPost)

	// Play
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

	// Admin
	asub := sub.PathPrefix("/admin").Subrouter()
	asub.HandleFunc("", s.RawHandler).Methods(http.MethodGet)
	asub.HandleFunc("/song/{song_id}", s.AdminEditSong).Methods(http.MethodGet)
	asub.HandleFunc("/song/{song_id}", s.AdminInsertSong).Methods(http.MethodPost)
	asub.HandleFunc("/song/{song_id}", s.AdminUpdateSong).Methods(http.MethodPatch)
	asub.HandleFunc("/song/{song_id}", s.AdminDelete).Methods(http.MethodDelete)

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

	cfg.Logger.Info("-> HTMLServer")
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
		s.logger.Debug("[request] - " + r.RequestURI)
		// s.logger.Debug(r.RequestURI, log.Any("r", r))
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
	db            db.DBer
	logger        log.Logger
	downloader    downloader.Downloader
	templates     map[string]*template.Template
	notifySubsMu  sync.Mutex
	notifySubs    map[chan notifyEvent]struct{}
}

func New(db db.DBer, l log.Logger) *Server {
	var dl downloader.Downloader
	if viper.GetString("downloader") == "ytdl" {
		dl = &downloader.YoutubeDownloader{}
		l.Info("using 'ytdl' downloader")
	} else {
		cfg := &downloader.YoutubeDLConfig{
			SongRoot:  viper.GetString("player.song_root"),
			ThumbRoot: viper.GetString("player.thumb_root"),
		}
		ytdl := downloader.NewYoutubeDLDownloader(cfg)
		if err := ytdl.EnsureAvailable(); err != nil {
			l.Info("yt-dlp not in PATH; downloads will fail until installed", log.Error(err))
		}
		dl = ytdl
		l.Info("using 'youtube-dl' downloader")
	}

	l.Info("-> New server")

	srv := &Server{
		db:         db,
		logger:     l,
		downloader: dl,
		notifySubs: make(map[chan notifyEvent]struct{}),
	}
	srv.templates = srv.loadTemplates()
	return srv
}

func (s *Server) loadTemplates() map[string]*template.Template {
	layout := "templates/layout.html"
	m := map[string]*template.Template{
		"index":        template.Must(template.ParseFiles("templates/index.html", layout)),
		"editSong":     template.Must(template.ParseFiles("templates/edit_song.html", layout)),
		"newSong":      template.Must(template.New("base").ParseFiles("templates/new_song.html", layout)),
		"playVideo":    template.Must(template.New("base").Funcs(template.FuncMap{}).ParseFiles("templates/play_video.html", layout)),
		"editRfid":     template.Must(template.ParseFiles("templates/edit_rfid.html", layout)),
		"assignSong":   template.Must(template.ParseFiles("templates/assign_song.html", layout)),
		"raw":          template.Must(template.ParseFiles("templates/raw.html", layout)),
		"admin":        template.Must(template.ParseFiles("templates/admin.html", layout)),
		"adminEditSong": template.Must(template.ParseFiles("templates/editSong.html", layout)),
		"player":       template.Must(template.New("base").ParseFiles("templates/player.html", layout)),
		"print":        template.Must(template.New("base").ParseFiles("templates/print.html", layout)),
	}
	configFuncs := template.FuncMap{
		"ConfigString": func(feature string) template.HTML {
			v := viper.GetString(feature)
			return template.HTML(fmt.Sprintf(`<label for="%s">%s</label><input id="%s" type="text" value="%s" name="%s">`, feature, feature, feature, v, feature))
		},
		"ConfigBool": func(feature string) template.HTML {
			v := viper.GetBool(feature)
			checked := ""
			if v {
				checked = `checked`
			}
			return template.HTML(fmt.Sprintf(`<input type="checkbox" name="%s" %s><i class="form-icon"></i> %s`, feature, checked, feature))
		},
		"ConfigInt": func(feature string) template.HTML {
			v := viper.GetInt(feature)
			return template.HTML(fmt.Sprintf(`<label for="%s">%s</label><input class="form-input" id="%s" type="number" placeholder="00" value="%d" name="%s">`, feature, feature, feature, v, feature))
		},
	}
	m["config"] = template.Must(template.New("base").Funcs(configFuncs).ParseFiles("templates/config.html", layout))
	return m
}

// notifyBroadcast sends a notification to all connected SSE clients (e.g. Chrome on Android).
func (s *Server) notifyBroadcast(title, body string) {
	ev := notifyEvent{Title: title, Body: body}
	s.notifySubsMu.Lock()
	defer s.notifySubsMu.Unlock()
	for ch := range s.notifySubs {
		select {
		case ch <- ev:
		default:
			// client slow; skip
		}
	}
}

// EventsSSE streams server-sent events so the browser can show Web Notifications when downloads complete.
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
	go func() {
		<-r.Context().Done()
		s.notifySubsMu.Lock()
		delete(s.notifySubs, ch)
		s.notifySubsMu.Unlock()
		close(ch)
	}()
	defer func() {
		s.notifySubsMu.Lock()
		delete(s.notifySubs, ch)
		s.notifySubsMu.Unlock()
	}()

	enc := json.NewEncoder(&streamWriter{w: w})
	for ev := range ch {
		fmt.Fprint(w, "event: notification\ndata: ")
		_ = enc.Encode(ev)
		fmt.Fprint(w, "\n")
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
	}
}

// streamWriter wraps http.ResponseWriter so json.Encoder writes to the response.
type streamWriter struct {
	w http.ResponseWriter
}

func (s streamWriter) Write(p []byte) (n int, err error) {
	return s.w.Write(p)
}

// Render a template, or server error.
func (s *Server) render(w http.ResponseWriter, r *http.Request, tpl *template.Template, data any) {
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

func (s *Server) Logs(w http.ResponseWriter, r *http.Request) {
	flog, ok := s.logger.(*log.FileLogger)
	if !ok {
		fmt.Fprintf(w, "cannot convert logger")
		return
	}
	dat, err := os.ReadFile(flog.Path)
	if err != nil {
		fmt.Fprintln(w, err.Error())
		return
	}
	fmt.Fprint(w, string(dat))
}
