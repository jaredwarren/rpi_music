package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/jaredwarren/rpi_music/log"
	"github.com/jaredwarren/rpi_music/model"
	"github.com/jaredwarren/rpi_music/player"
	"github.com/spf13/viper"
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
		"templates/new_song.html",
		"templates/layout.html",
	}
	// TODO:  maybe these would be better as objects
	tpl := template.Must(template.New("base").ParseFiles(files...))
	s.render(w, r, tpl, fullData)
}

func (s *Server) NewSongHandler(w http.ResponseWriter, r *http.Request) {
	var err error
	err = r.ParseMultipartForm(32 << 20)
	if err != nil {
		s.logger.Error(err.Error())
		s.httpError(w, fmt.Errorf("NewSongHandler|ParseForm|%w", err), http.StatusBadRequest)
		return
	}
	s.logger.Info("NewSongHandler", log.Any("form", r.PostForm))
	return

	var song *model.Song

	vtype := r.PostForm.Get("type")
	switch vtype {
	case "upload":
		song, err = s.uploadSongHandler(r)
		if err != nil {
			s.logger.Error(err.Error())
			s.httpError(w, err, http.StatusBadRequest)
			return
		}
	case "download":
		song, err = s.downloadSongHandler(r)
		if err != nil {
			s.logger.Error(err.Error())
			s.httpError(w, err, http.StatusBadRequest)
			return
		}
	default:
		s.httpError(w, fmt.Errorf("invalid type|%s", vtype), http.StatusBadRequest)
		return
	}

	// 2. Store
	err = s.db.UpdateSong(song)
	if err != nil {
		s.httpError(w, fmt.Errorf("NewSongHandler|db.Update|%w", err), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/songs", http.StatusFound)
}

func (s *Server) uploadSongHandler(r *http.Request) (*model.Song, error) {
	rfid := r.PostForm.Get("rfid")
	if rfid == "" {
		return nil, fmt.Errorf("need rfid")
	}
	rfid = strings.ReplaceAll(rfid, ":", "")

	title := r.PostForm.Get("title")

	song := &model.Song{
		ID:    rfid,
		Title: title,
		RFID:  rfid,
	}

	// download image
	thumb := r.PostForm.Get("thumb")
	if thumb != "" {
		thumbFile, err := downloadThumbFile(thumb)
		if err != nil {
			s.logger.Error(err.Error())
			return nil, fmt.Errorf("thumbCopy|%w", err)
		}
		song.Thumbnail = thumbFile
	}

	// Handle Upload
	file, handler, err := r.FormFile("upload")
	if err != nil {
		s.logger.Error(err.Error())
		return nil, fmt.Errorf("FormFile|%w", err)
	}
	defer file.Close()

	fileName := filepath.Join(viper.GetString("player.song_root"), handler.Filename)
	f, err := os.OpenFile(fileName, os.O_WRONLY|os.O_CREATE, 0666)
	if err != nil {
		s.logger.Error(err.Error())
		return nil, fmt.Errorf("OpenFile|%w", err)
	}
	defer f.Close()
	_, err = io.Copy(f, file)
	if err != nil {
		s.logger.Error(err.Error())
		return nil, fmt.Errorf("Copy|%w", err)
	}

	return song, nil
}

func (s *Server) downloadSongHandler(r *http.Request) (*model.Song, error) {
	rfid := r.PostForm.Get("rfid")
	if rfid == "" {
		return nil, fmt.Errorf("need rfid")
	}
	rfid = strings.ReplaceAll(rfid, ":", "")

	url := r.PostForm.Get("url")
	if url == "" {
		return nil, fmt.Errorf("missing url")
	}

	song := &model.Song{
		ID:   rfid,
		RFID: rfid,
		URL:  url,
	}

	// 1. Download song
	file, video, err := s.downloader.DownloadVideo(url, s.logger)
	if err != nil {
		s.logger.Error(err.Error())
		return nil, fmt.Errorf("DownloadVideo|%w", err)
	}

	thumb := r.PostForm.Get("thumb")
	if thumb != "" {
		thumbFile, err := downloadThumbFile(thumb)
		if err != nil {
			s.logger.Error(err.Error())
			return nil, fmt.Errorf("thumbCopy|%w", err)
		}
		song.Thumbnail = thumbFile
	} else {
		tmb, err := s.downloader.DownloadThumb(video)
		if err != nil {
			s.logger.Warn("NewSongHandler|downloadThumb", log.Error(err))
			// ignore err
		}
		song.Thumbnail = tmb
	}
	song.FilePath = file
	song.Title = video.Title

	return song, nil
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
	rfid = strings.ReplaceAll(rfid, ":", "") // added because js code is bad and sometimes sends rfid without ':'

	// Delete if rfid blank
	if rfid == "" {
		s.httpError(w, fmt.Errorf("need rfid"), http.StatusBadRequest)
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

	http.Redirect(w, r, "/songs", http.StatusFound)
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
	http.Redirect(w, r, "/songs", http.StatusFound)
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

func downloadThumbFile(URL string) (string, error) {
	//Get the response bytes from the url
	response, err := http.Get(URL)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()

	if response.StatusCode != 200 {
		return "", errors.New("Received non 200 response code")
	}
	//Create a empty file
	fileUrl, err := url.Parse(URL)
	if err != nil {
		return "", err
	}
	segments := strings.Split(fileUrl.Path, "/")
	fileName := segments[len(segments)-1]
	if fileName == "" {
		fileName = uuid.New().String()
	}
	fileName = filepath.Join(viper.GetString("player.thumb_root"), fileName)

	file, err := os.Create(fileName)
	if err != nil {
		return "", err
	}
	defer file.Close()

	//Write the bytes to the fiel
	_, err = io.Copy(file, response.Body)
	if err != nil {
		return "", err
	}

	return fileName, nil
}
