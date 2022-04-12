package server

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/gorilla/mux"
	"github.com/jaredwarren/rpi_music/model"
	"github.com/jaredwarren/rpi_music/player"
	"github.com/kkdai/youtube/v2"
	bolt "go.etcd.io/bbolt"
)

const (
	SongBucket = "SongBucket"
)

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
	db *bolt.DB
}

func New(db *bolt.DB) *Server {
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
		db: db,
	}
}

func ArticlesCategoryHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Category: %v\n", vars["category"])
}

func (s *Server) EditSongFormHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Println(":: EditSongFormHandler ::")
	vars := mux.Vars(r)
	key := vars["song_id"]
	if key == "" {
		fmt.Println("no key")
		return
	}
	push(w, "/static/style.css")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	var song *model.Song
	s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(SongBucket))
		v := b.Get([]byte(key))
		err := json.Unmarshal(v, &song)
		if err != nil {
			return err
		}
		return nil
	})

	fullData := map[string]interface{}{
		"Song": song,
	}
	render(w, r, editSongFormTpl, fullData)
}

func (s *Server) ListSongHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Println(":: ListSongHandler ::")
	push(w, "/static/style.css")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	songs := []*model.Song{}

	s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(SongBucket))
		c := b.Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			var song *model.Song
			err := json.Unmarshal(v, &song)
			if err != nil {
				return err
			}
			songs = append(songs, song)
		}
		return nil
	})

	fullData := map[string]interface{}{
		"Songs": songs,
	}
	render(w, r, homepageTpl, fullData)
}

func (s *Server) NewSongFormHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Println(":: NewSongFormHandler ::")

	push(w, "/static/style.css")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	song := &model.Song{
		ID: "new",
	}

	fullData := map[string]interface{}{
		"Song": song,
	}
	render(w, r, newSongFormTpl, fullData)
}

func (s *Server) NewSongHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Println(":: NewSongHandler ::")
	// 0. Validate input
	err := r.ParseForm()
	if err != nil {
		httpError(w, fmt.Errorf("NewSongHandler|ParseForm|%w", err))
		return
	}

	url := r.PostForm.Get("url")
	if url == "" {
		httpError(w, fmt.Errorf("need url"))
		return
	}

	rfid := r.PostForm.Get("rfid")
	if rfid == "" {
		httpError(w, fmt.Errorf("need rfid"))
		return
	}

	fmt.Println(" - url:", url)
	fmt.Println(" - rfid:", rfid)
	rfid = strings.ReplaceAll(rfid, ":", "")

	overwrite := true // TODO: make param,
	if !overwrite {
		// check for duplicates
		err = s.db.View(func(tx *bolt.Tx) error {
			b := tx.Bucket([]byte(SongBucket))
			v := b.Get([]byte(rfid))
			if v != nil {
				return fmt.Errorf("already exists")
			}
			return nil
		})
		if err != nil {
			httpError(w, fmt.Errorf("NewSongHandler|db.View|%w", err))
			return
		}
	}

	song := &model.Song{
		ID:   rfid,
		URL:  url,
		RFID: rfid,
	}

	// 1. Download song
	file, video, err := downloadVideo(url)
	if err != nil {
		httpError(w, fmt.Errorf("NewSongHandler|downloadVideo|%w", err))
		return
	}
	song.FilePath = file
	song.Title = video.Title

	// 2. Store
	err = s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(SongBucket))

		buf, err := json.Marshal(song)
		if err != nil {
			return err
		}
		return b.Put([]byte(song.ID), buf)
	})
	if err != nil {
		httpError(w, fmt.Errorf("NewSongHandler|db.Update|%w", err))
		return
	}

	http.Redirect(w, r, "/songs", 301)
}

func (s *Server) UpdateSongHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Println(":: UpdateSongHandler ::")
	vars := mux.Vars(r)
	key := vars["song_id"]
	if key == "" {
		httpError(w, fmt.Errorf("no key"))
		return
	}

	err := r.ParseForm()
	if err != nil {
		httpError(w, fmt.Errorf("UpdateSongHandler|ParseForm|%w", err))
		return
	}

	url := r.PostForm.Get("url")
	rfid := r.PostForm.Get("rfid")
	rfid = strings.ReplaceAll(rfid, ":", "")

	fmt.Println(" - key:", key)
	fmt.Println(" - url:", url)
	fmt.Println(" - rfid:", rfid)

	// Delete if blank
	if rfid == "" || url == "" {
		err := s.db.Update(func(tx *bolt.Tx) error {
			b := tx.Bucket([]byte(SongBucket))
			return b.Delete([]byte(key)) // note: needs to "key"
		})
		if err != nil {
			httpError(w, fmt.Errorf("UpdateSongHandler|db.Update|%w", err))
			return
		}
		return
	}

	song := &model.Song{
		ID:   rfid,
		URL:  url,
		RFID: rfid,
	}

	// try to download file again
	file, video, err := downloadVideo(url)
	if err != nil {
		httpError(w, fmt.Errorf("UpdateSongHandler|downloadVideo|%w", err))
		return
	}
	song.FilePath = file
	song.Title = video.Title

	// delete old key if rfid id different then key
	if key != rfid {
		err := s.db.Update(func(tx *bolt.Tx) error {
			b := tx.Bucket([]byte(SongBucket))
			return b.Delete([]byte(key)) // note: needs to "key"
		})
		if err != nil {
			httpError(w, fmt.Errorf("UpdateSongHandler|db.Update|%w", err))
			return
		}
	}

	// Update otherwise
	err = s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(SongBucket))
		v := b.Get([]byte(key)) // note: needs to "key"
		if v == nil {
			return fmt.Errorf("missing id")
		}

		buf, err := json.Marshal(song)
		if err != nil {
			return err
		}
		return b.Put([]byte(song.ID), buf)
	})
	if err != nil {
		httpError(w, fmt.Errorf("UpdateSongHandler|db.Update|%w", err))
		return
	}

	http.Redirect(w, r, "/songs", 301)
}

func (s *Server) DeleteSongHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	key := vars["song_id"]
	if key == "" {
		httpError(w, fmt.Errorf("no key"))
		return
	}
	err := s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(SongBucket))
		return b.Delete([]byte(key))
	})
	if err != nil {
		httpError(w, fmt.Errorf("DeleteSongHandler|db.Update|%w", err))
		return
	}
	http.Redirect(w, r, "/songs", 301)
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
		return "", video, err
	}

	formats := video.Formats.WithAudioChannels() // only get videos with audio
	formats.Sort()                               // I think this sorts best > worst

	bestFormat := formats[0]
	ext := getExt(bestFormat.MimeType)

	sEnc := base64.StdEncoding.EncodeToString([]byte(video.Title))

	fileName := fmt.Sprintf("song_files/%s.%s", sEnc, ext)

	if _, err := os.Stat(fileName); errors.Is(err, os.ErrNotExist) {
		// path/to/whatever does not exist

		stream, _, err := client.GetStream(video, &bestFormat)
		if err != nil {
			return "", video, err
		}

		file, err := os.Create(fileName)
		if err != nil {
			return "", video, err
		}
		defer file.Close()

		_, err = io.Copy(file, stream)
		if err != nil {
			return "", video, err
		}
	}
	return fileName, video, nil
}

func getExt(mimeType string) string {
	ls := strings.ToLower(mimeType)
	if strings.Contains(ls, "video/mp4") {
		return "mp4"
	}
	if strings.Contains(ls, "video/webm") {
		return "webm"
	}
	fmt.Println("~~~~ unknown::", mimeType)
	return "" //
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
		httpError(w, fmt.Errorf("PlaySongHandler|db.View|%w", err))
		return
	}
	player.Beep()
	err = player.Play(song)
	if err != nil {
		httpError(w, fmt.Errorf("PlaySongHandler|player.Play|%w", err))
		return
	}

	fullData := map[string]interface{}{
		"Song": song,
	}
	render(w, r, playerTpl, fullData)
}

func (s *Server) StopSongHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Println(":: StopSongHandler ::")
	player.Stop()

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
		httpError(w, fmt.Errorf("PlaySongHandler|db.View|%w", err))
		return
	}
	fullData := map[string]interface{}{
		"Song": song,
	}
	render(w, r, playerTpl, fullData)
}
