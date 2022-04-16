package server

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	"github.com/jaredwarren/rpi_music/model"
	bolt "go.etcd.io/bbolt"
)

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

	// for now
	files := []string{
		"templates/index.html",
		"templates/layout.html",
	}
	homepageTpl = template.Must(template.ParseFiles(files...))

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
	files := []string{
		"templates/new_song.html",
		"templates/layout.html",
	}
	// TODO:  maybe these would be better as objects
	tpl := template.Must(template.New("base").ParseFiles(files...))
	render(w, r, tpl, fullData)
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
	tmb, err := downloadThumb(video)
	if err != nil {
		fmt.Println("NewSongHandler|downloadThumb|", err)
		// ignore err
	}

	song.Thumbnail = tmb
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
	tmb, err := downloadThumb(video)
	if err != nil {
		fmt.Println("NewSongHandler|downloadThumb|", err)
		// ignore err
	}

	song.Thumbnail = tmb
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
