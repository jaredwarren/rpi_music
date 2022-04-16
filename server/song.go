package server

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	"github.com/jaredwarren/rpi_music/log"
	"github.com/jaredwarren/rpi_music/model"
	bolt "go.etcd.io/bbolt"
)

func (s *Server) EditSongFormHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	key := vars["song_id"]
	if key == "" {
		s.httpError(w, fmt.Errorf("song_id required"), http.StatusBadRequest)
		return
	}

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
	s.render(w, r, editSongFormTpl, fullData)
}

func (s *Server) ListSongHandler(w http.ResponseWriter, r *http.Request) {
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

	s.render(w, r, homepageTpl, fullData)
}

func (s *Server) NewSongFormHandler(w http.ResponseWriter, r *http.Request) {
	fullData := map[string]interface{}{
		"Song": &model.Song{
			ID: "new",
		},
	}
	files := []string{
		"templates/new_song.html",
		"templates/layout.html",
	}
	// TODO:  maybe these would be better as objects
	tpl := template.Must(template.New("base").ParseFiles(files...))
	s.render(w, r, tpl, fullData)
}

func (s *Server) NewSongHandler(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		s.httpError(w, fmt.Errorf("NewSongHandler|ParseForm|%w", err), http.StatusBadRequest)
		return
	}
	s.logger.Info("NewSongHandler", log.Any("form", r.PostForm))

	url := r.PostForm.Get("url")
	if url == "" {
		s.httpError(w, fmt.Errorf("need url"), http.StatusBadRequest)
		return
	}

	rfid := r.PostForm.Get("rfid")
	if rfid == "" {
		s.httpError(w, fmt.Errorf("need rfid"), http.StatusBadRequest)
		return
	}

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
			s.httpError(w, fmt.Errorf("NewSongHandler|db.View|%w", err), http.StatusInternalServerError)
			return
		}
	}

	song := &model.Song{
		ID:   rfid,
		URL:  url,
		RFID: rfid,
	}

	// 1. Download song
	file, video, err := downloadVideo(url, s.logger)
	if err != nil {
		s.httpError(w, fmt.Errorf("NewSongHandler|downloadVideo|%w", err), http.StatusInternalServerError)
		return
	}
	tmb, err := downloadThumb(video)
	if err != nil {
		s.logger.Warn("NewSongHandler|downloadThumb", log.Error(err))
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
		s.httpError(w, fmt.Errorf("NewSongHandler|db.Update|%w", err), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/songs", 301)
}

func (s *Server) UpdateSongHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	key := vars["song_id"]
	if key == "" {
		s.httpError(w, fmt.Errorf("no key"), http.StatusBadRequest)
		return
	}
	s.logger.Info("UpdateSongHandler", log.Any("form", r.PostForm))

	err := r.ParseForm()
	if err != nil {
		s.httpError(w, fmt.Errorf("UpdateSongHandler|ParseForm|%w", err), http.StatusBadRequest)
		return
	}

	url := r.PostForm.Get("url")
	rfid := r.PostForm.Get("rfid")
	rfid = strings.ReplaceAll(rfid, ":", "")

	// Delete if blank
	if rfid == "" || url == "" {
		err := s.db.Update(func(tx *bolt.Tx) error {
			b := tx.Bucket([]byte(SongBucket))
			return b.Delete([]byte(key)) // note: needs to "key"
		})
		if err != nil {
			s.httpError(w, fmt.Errorf("UpdateSongHandler|db.Update|%w", err), http.StatusInternalServerError)
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
	file, video, err := downloadVideo(url, s.logger)
	if err != nil {
		s.httpError(w, fmt.Errorf("UpdateSongHandler|downloadVideo|%w", err), http.StatusInternalServerError)
		return
	}
	tmb, err := downloadThumb(video)
	if err != nil {
		s.logger.Warn("UpdateSongHandler|downloadThumb", log.Error(err))
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
			s.httpError(w, fmt.Errorf("UpdateSongHandler|db.Update|%w", err), http.StatusInternalServerError)
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
		s.httpError(w, fmt.Errorf("UpdateSongHandler|db.Update|%w", err), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/songs", 301)
}

func (s *Server) DeleteSongHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	key := vars["song_id"]
	if key == "" {
		s.httpError(w, fmt.Errorf("no key"), http.StatusBadRequest)
		return
	}
	err := s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(SongBucket))
		return b.Delete([]byte(key))
	})
	if err != nil {
		s.httpError(w, fmt.Errorf("DeleteSongHandler|db.Update|%w", err), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/songs", 301)
}
