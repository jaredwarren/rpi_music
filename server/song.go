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
	"github.com/jaredwarren/rpi_music/player"
)

// JSONHandler
func (s *Server) JSONHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	vars := mux.Vars(r)
	key := vars["song_id"]
	if key == "" {
		json.NewEncoder(w).Encode(map[string]string{
			"error": "song_id required",
		})
		return
	}

	song, err := s.db.GetSong(key)
	if err != nil {
		json.NewEncoder(w).Encode(map[string]string{
			"error": err.Error(),
		})
		return
	}
	if song == nil {
		json.NewEncoder(w).Encode(map[string]string{
			"error": "song not found",
		})
		return
	}
	json.NewEncoder(w).Encode(song)
}

func (s *Server) EditSongFormHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	key := vars["song_id"]
	if key == "" {
		s.httpError(w, fmt.Errorf("song_id required"), http.StatusBadRequest)
		return
	}

	song, err := s.db.GetSong(key)
	if err != nil {
		s.httpError(w, fmt.Errorf("EditSongFormHandler|GetSong|%w", err), http.StatusBadRequest)
		return
	}
	if song == nil {
		s.httpError(w, fmt.Errorf("EditSongFormHandler|GetSong|%w", err), http.StatusBadRequest)
		return
	}

	fullData := map[string]interface{}{
		"Song":      song,
		TemplateTag: s.GetToken(w, r),
	}

	files := []string{
		"templates/edit_song.html",
		"templates/layout.html",
	}
	editSongFormTpl := template.Must(template.ParseFiles(files...))
	s.render(w, r, editSongFormTpl, fullData)
}

func (s *Server) ListSongHandler(w http.ResponseWriter, r *http.Request) {
	cp := player.GetPlayer()
	song := player.GetPlaying()

	songs, err := s.db.ListSongs()
	if err != nil {
		s.httpError(w, fmt.Errorf("ListSongHandler|ListSongs|%w", err), http.StatusBadRequest)
		return
	}
	fullData := map[string]interface{}{
		"Songs":       songs,
		"CurrentSong": song,
		"Player":      cp,
		TemplateTag:   s.GetToken(w, r),
	}

	// for now
	files := []string{
		"templates/index.html",
		"templates/layout.html",
	}
	homepageTpl := template.Must(template.ParseFiles(files...))

	s.render(w, r, homepageTpl, fullData)
}

func (s *Server) NewSongFormHandler(w http.ResponseWriter, r *http.Request) {
	fullData := map[string]interface{}{
		"Song":      model.NewSong(),
		TemplateTag: s.GetToken(w, r),
	}
	files := []string{
		"templates/edit_song.html",
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

	overwrite := true // TODO: make param, or config
	if !overwrite {
		// check for duplicates
		exists, err := s.db.SongExists(rfid)
		if err != nil {
			s.httpError(w, fmt.Errorf("NewSongHandler|db.View|%w", err), http.StatusInternalServerError)
			return
		}
		if exists {
			// TODO: what to do here?
			return
		}
	}

	song := &model.Song{
		ID:   rfid,
		URL:  url,
		RFID: rfid,
	}

	// 1. Download song
	file, video, err := s.downloader.DownloadVideo(url, s.logger)
	if err != nil {
		s.httpError(w, fmt.Errorf("NewSongHandler|downloadVideo|%w", err), http.StatusInternalServerError)
		return
	}
	tmb, err := s.downloader.DownloadThumb(video)
	if err != nil {
		s.logger.Warn("NewSongHandler|downloadThumb", log.Error(err))
		// ignore err
	}

	song.Thumbnail = tmb
	song.FilePath = file
	song.Title = video.Title

	// 2. Store
	err = s.db.UpdateSong(song)
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
		err := s.db.DeleteSong(key)
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
	file, video, err := s.downloader.DownloadVideo(url, s.logger)
	if err != nil {
		s.httpError(w, fmt.Errorf("UpdateSongHandler|downloadVideo|%w", err), http.StatusInternalServerError)
		return
	}
	tmb, err := s.downloader.DownloadThumb(video)
	if err != nil {
		s.logger.Warn("UpdateSongHandler|downloadThumb", log.Error(err))
		// ignore err
	}

	song.Thumbnail = tmb
	song.FilePath = file
	song.Title = video.Title

	// delete old key if rfid id different then key
	if key != rfid {
		err := s.db.DeleteSong(key)
		if err != nil {
			s.httpError(w, fmt.Errorf("UpdateSongHandler|db.Update|%w", err), http.StatusInternalServerError)
			return
		}
	}

	// Update otherwise
	err = s.db.UpdateSong(song)
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
	err := s.db.DeleteSong(key)
	if err != nil {
		s.httpError(w, fmt.Errorf("DeleteSongHandler|db.Update|%w", err), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/songs", 301)
}

func (s *Server) PlayVideoHandler(w http.ResponseWriter, r *http.Request) {
	s.logger.Info("PlayVideoHandler")

	vars := mux.Vars(r)
	key := vars["song_id"]
	if key == "" {
		s.httpError(w, fmt.Errorf("song_id required"), http.StatusBadRequest)
		return
	}

	song, err := s.db.GetSong(key)
	if err != nil {
		s.httpError(w, fmt.Errorf("EditSongFormHandler|GetSong|%w", err), http.StatusBadRequest)
		return
	}
	if song == nil {
		s.httpError(w, fmt.Errorf("EditSongFormHandler|GetSong|%w", err), http.StatusBadRequest)
		return
	}

	fullData := map[string]interface{}{
		"Song":      song,
		TemplateTag: s.GetToken(w, r),
	}

	files := []string{
		"templates/play_video.html",
		"templates/layout.html",
	}
	tpl := template.Must(template.New("base").Funcs(template.FuncMap{}).ParseFiles(files...))
	s.render(w, r, tpl, fullData)
}
