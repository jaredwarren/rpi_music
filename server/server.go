package server

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"github.com/jaredwarren/rpi_music/log"
	"github.com/jaredwarren/rpi_music/model"
	"github.com/jaredwarren/rpi_music/player"
	"github.com/kkdai/youtube/v2"
	"github.com/spf13/viper"
	bolt "go.etcd.io/bbolt"
)

const (
	SongBucket = "SongBucket"
)

// Config provides basic configuration
type Config struct {
	Host         string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	Db           *bolt.DB
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

	// list songs
	r.HandleFunc("/songs", s.ListSongHandler).Methods("GET")
	// new song form
	r.HandleFunc("/song/new", s.NewSongFormHandler).Methods("GET")
	// submit new song
	r.HandleFunc("/song", s.NewSongHandler).Methods("POST")
	r.HandleFunc("/song/new", s.NewSongHandler).Methods("POST")
	// Edit Song Form
	r.HandleFunc("/song/{song_id}", s.EditSongFormHandler).Methods("GET")
	// new link
	r.HandleFunc("/song/{song_id}", s.UpdateSongHandler).Methods("PUT", "POST")
	// delete song
	r.HandleFunc("/song/{song_id}", s.DeleteSongHandler).Methods("DELETE")
	r.HandleFunc("/song/{song_id}/delete", s.DeleteSongHandler).Methods("GET") // temp unitl I can get a better UI

	r.HandleFunc("/song/{song_id}/play", s.PlaySongHandler)
	r.HandleFunc("/song/{song_id}/stop", s.StopSongHandler)

	r.HandleFunc("/config", s.ConfigFormHandler).Methods("GET")
	r.HandleFunc("/config", s.ConfigHandler).Methods("POST")

	r.HandleFunc("/player", s.PlayerHandler).Methods("GET")
	// TODO:play locally or remotely
	// remote media controls (WS?) (play, pause, volume +/-)

	r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("./static"))))
	r.PathPrefix("/song_files/").Handler(http.StripPrefix("/song_files/", http.FileServer(http.Dir(viper.GetString("player.song_root")))))
	r.PathPrefix("/thumb_files/").Handler(http.StripPrefix("/thumb_files/", http.FileServer(http.Dir(viper.GetString("player.thumb_root")))))

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

// Templates
var (
	homepageTpl     *template.Template
	editSongFormTpl *template.Template
	newSongFormTpl  *template.Template
	playerTpl       *template.Template
)

func init() {
	homepageTpl = template.Must(template.ParseFiles("templates/index.html"))
	newSongFormTpl = template.Must(template.ParseFiles("templates/new_song.html"))
	editSongFormTpl = template.Must(template.ParseFiles("templates/edit_song.html"))
	playerTpl = template.Must(template.ParseFiles("templates/player.html"))
}

type Server struct {
	db     *bolt.DB
	logger log.Logger
}

func New(db *bolt.DB, l log.Logger) *Server {
	err := db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(SongBucket))
		if err != nil {
			return fmt.Errorf("create bucket: %s", err)
		}
		return nil
	})
	if err != nil {
		panic(err)
	}

	return &Server{
		db:     db,
		logger: l,
	}
}

// Render a template, or server error.
func render(w http.ResponseWriter, r *http.Request, tpl *template.Template, data interface{}) {
	buf := new(bytes.Buffer)
	if err := tpl.Execute(buf, data); err != nil {
		fmt.Printf("\nRender Error: %v\n", err)
		return
	}
	w.Write(buf.Bytes())
}

// Push the given resource to the client.
func push(w http.ResponseWriter, resource string) {
	pusher, ok := w.(http.Pusher)
	if ok {
		err := pusher.Push(resource, nil)
		if err != nil {
			fmt.Println("push error:", err)
		}
		return
	}
}

func downloadVideo(videoID string) (string, *youtube.Video, error) {
	client := youtube.Client{}
	video, err := client.GetVideo(videoID)
	if err != nil {
		fmt.Println("downloadVideo|client.GetVideo|", err)
		return "", video, err
	}

	formats := video.Formats.WithAudioChannels() // only get videos with audio
	formats.Sort()                               // I think this sorts best > worst

	bestFormat := formats[0]
	ext := getExt(bestFormat.MimeType)
	sEnc := base64.StdEncoding.EncodeToString([]byte(video.Title))
	fileName := filepath.Join(viper.GetString("player.song_root"), fmt.Sprintf("%s%s", sEnc, ext))

	fmt.Println("downloading...", video.Title, "->", fileName)

	if _, err := os.Stat(fileName); errors.Is(err, os.ErrNotExist) {
		stream, _, err := client.GetStream(video, &bestFormat)
		if err != nil {
			fmt.Println("downloadVideo|client.GetStream|", err)
			return fileName, video, err
		}

		file, err := os.Create(fileName)
		if err != nil {
			fmt.Println("downloadVideo|os.Create|", err)
			return fileName, video, err
		}
		defer file.Close()

		_, err = io.Copy(file, stream)
		if err != nil {
			fmt.Println("downloadVideo|io.Copy|", err)
			return fileName, video, err
		}
	} else if err != nil {
		fmt.Println("downloadVideo|os.Stat|", err)
		return fileName, video, err
	} else {
		fmt.Println("file already exists:", fileName)
	}
	return fileName, video, nil
}

func getExt(mimeType string) string {
	ls := strings.ToLower(mimeType)
	if strings.Contains(ls, "video/mp4") {
		return ".mp4"
	}
	if strings.Contains(ls, "video/webm") {
		return ".webm"
	}
	fmt.Println("~~~~ unknown::", mimeType)
	return "" //
}

func downloadThumb(video *youtube.Video) (string, error) {
	if len(video.Thumbnails) == 0 {
		return "", fmt.Errorf("no thumbs for video")
	}

	// find biggest
	thumb := video.Thumbnails[0]
	for _, t := range video.Thumbnails {
		if t.Width > thumb.Width {
			thumb = t
		}
	}

	fileURL := thumb.URL

	// clean up `.../hqdefault.jpg?sqp=-oaymwEj...`
	ext := filepath.Ext(fileURL)
	ext = strings.Split(ext, "?")[0]
	sEnc := base64.StdEncoding.EncodeToString([]byte(video.Title))
	fileName := filepath.Join(viper.GetString("player.thumb_root"), fmt.Sprintf("%s%s", sEnc, ext))

	err := downloadFile(fileURL, fileName)
	if err != nil {
		return "", err
	}

	return fileName, nil
}

func downloadFile(URL, fileName string) error {
	//Get the response bytes from the url
	response, err := http.Get(URL)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if response.StatusCode != 200 {
		return errors.New("Received non 200 response code")
	}
	//Create a empty file
	file, err := os.Create(fileName)
	if err != nil {
		return err
	}
	defer file.Close()

	//Write the bytes to the fiel
	_, err = io.Copy(file, response.Body)
	if err != nil {
		return err
	}

	return nil
}

func (s *Server) PlayerHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Println(":: PlayerHandler ::")

	cp := player.GetPlayer()
	song := player.GetPlaying()

	fullData := map[string]interface{}{
		"Player": cp,
		"Song":   song,
	}
	files := []string{
		"templates/player.html",
		"templates/layout.html",
	}
	tpl := template.Must(template.New("base").ParseFiles(files...))
	render(w, r, tpl, fullData)
}

func (s *Server) PlaySongHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Println(":: PlaySongHandler ::")
	vars := mux.Vars(r)
	key := vars["song_id"]

	var song *model.Song
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(SongBucket))
		v := b.Get([]byte(key))
		err := json.Unmarshal(v, &song)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		s.httpError(w, fmt.Errorf("PlaySongHandler|db.View|%w", err), http.StatusInternalServerError)
		return
	}
	err = player.Play(song)
	if err != nil {
		// TODO: check if err is user error or system error
		s.httpError(w, fmt.Errorf("PlaySongHandler|player.Play|%w", err), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/player", 301)
}

func (s *Server) StopSongHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Println(":: StopSongHandler ::")
	player.Stop()

	http.Redirect(w, r, "/player", 301)
}
