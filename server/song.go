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
	"regexp"
	"strings"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/jaredwarren/rpi_music/downloader"
	"github.com/jaredwarren/rpi_music/log"
	"github.com/jaredwarren/rpi_music/model"
	"github.com/jaredwarren/rpi_music/player"
	"github.com/spf13/viper"
)

// JSONHandler
func (s *Server) JSONGetSongByRFID(w http.ResponseWriter, r *http.Request) {
	s.logger.Info("JSONGetSongByRFID")
	w.Header().Set("Content-Type", "application/json")
	vars := mux.Vars(r)
	rfid := vars["rfid"]
	if rfid == "" {
		json.NewEncoder(w).Encode(map[string]string{
			"error": "rfid required",
		})
		return
	}
	rs, err := s.db.GetRFIDSong(rfid)
	if err != nil {
		json.NewEncoder(w).Encode(map[string]string{
			"error": err.Error(),
		})
		return
	}
	if rs == nil || len(rs.Songs) == 0 {
		json.NewEncoder(w).Encode(map[string]string{
			"error": "rfid has now song",
		})
		return
	}

	song, err := s.db.GetSong(rs.Songs[0])
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
func (s *Server) JSONHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	vars := mux.Vars(r)
	songID := vars["song_id"]
	if songID == "" {
		json.NewEncoder(w).Encode(map[string]string{
			"error": "song_id required",
		})
		return
	}

	song, err := s.db.GetSong(songID)
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

	s.logger.Info("current song", log.Any("song", song))

	songs, err := s.db.ListSongs()
	if err != nil {
		s.httpError(w, fmt.Errorf("ListSongHandler|ListSongs|%w", err), http.StatusBadRequest)
		return
	}

	rfids, err := s.db.ListRFIDSongs()
	if err != nil {
		s.httpError(w, fmt.Errorf("ListSongHandler|ListRFIDSongs|%w", err), http.StatusBadRequest)
		return
	}
	for _, s := range songs {
		for _, r := range rfids {
			for _, rs := range r.Songs {
				if rs == s.ID {
					s.RFID = r.RFID
				}
			}
		}
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

// DownloadSong raw download song, same as new song, but easier url
func (s *Server) DownloadSong(w http.ResponseWriter, r *http.Request) {
	s.NewSongFormHandler(w, r)
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

	fmt.Printf("~~~~~~~~~~~~~~~\n %+v\n\n", r.PostForm)

	song, err := s.downloadSongHandler(r)
	if err != nil {
		s.logger.Error(err.Error())
		s.httpError(w, err, http.StatusBadRequest)
		return
	}

	// 2. Store
	err = s.db.UpdateSong(song)
	if err != nil {
		s.httpError(w, fmt.Errorf("NewSongHandler|db.Update|%w", err), http.StatusInternalServerError)
		return
	}

	// 3. Insert RFID if set
	rfid := r.PostForm.Get("rfid")
	rfid = strings.ReplaceAll(rfid, ":", "")
	if rfid != "" {
		// Make sure rfid doesn't exist yet.
		rfidSong, err := s.db.GetRFIDSong(rfid)
		if err != nil {
			s.logger.Error("RFIDExists error", log.Error(err))
			s.httpError(w, fmt.Errorf("RFIDExists error %w", err), http.StatusInternalServerError)
			return
		} else if rfidSong != nil {
			s.httpError(w, fmt.Errorf("rfid aready assigned! (%+v)", rfidSong), http.StatusInternalServerError)
			return
		} // else continue

		err = s.db.AddRFIDSong(rfid, song.ID)
		if err != nil {
			s.logger.Error("AddRFIDSong error", log.Error(err))
			s.httpError(w, fmt.Errorf("AddRFIDSong error %w", err), http.StatusInternalServerError)
			return
		}
	}

	http.Redirect(w, r, "/songs", http.StatusFound)
}

var urlreg = regexp.MustCompile(`.+?(https?:)`)

func (s *Server) downloadSongHandler(r *http.Request) (*model.Song, error) {
	url := r.PostForm.Get("url")
	if url == "" {
		return nil, fmt.Errorf("missing url")
	}
	url = urlreg.ReplaceAllString(url, "${1}")

	force := r.PostForm.Get("force") != ""
	if !force {
		// check if file exists
		filename, err := downloader.GetVideoFilename(url)
		if err != nil {
			return nil, err
		}
		if _, err := os.Stat(filename); err == nil {
			return nil, fmt.Errorf("file already downloaded")
		} else if !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
	}

	// 1. Download song
	file, video, err := s.downloader.DownloadVideo(url, s.logger)
	if err != nil {
		s.logger.Error(err.Error())
		return nil, fmt.Errorf("DownloadVideo|%w", err)
	}

	// 2. Download Thumb
	tmb, err := s.downloader.DownloadThumb(video)
	if err != nil {
		s.logger.Warn("NewSongHandler|downloadThumb", log.Error(err))
		// ignore err
	}
	song := &model.Song{
		ID:        uuid.New().String(),
		URL:       url,
		Thumbnail: tmb,
		FilePath:  file,
		Title:     video.Title,
	}

	return song, nil
}

func (s *Server) UpdateSongHandler(w http.ResponseWriter, r *http.Request) {
	s.logger.Warn("UpdateSongHandler broken!!!")
	return
	vars := mux.Vars(r)
	songID := vars["song_id"]
	if songID == "" {
		s.httpError(w, fmt.Errorf("no song_id"), http.StatusBadRequest)
		return
	}
	song, err := s.db.GetSong(songID)
	if err != nil {
		s.httpError(w, fmt.Errorf("UpdateSongHandler|GetSong|%w", err), http.StatusBadRequest)
		return
	}
	if song == nil {
		s.httpError(w, fmt.Errorf("UpdateSongHandler|GetSong|%w", err), http.StatusBadRequest)
		return
	}

	err = r.ParseForm()
	if err != nil {
		s.httpError(w, fmt.Errorf("UpdateSongHandler|ParseForm|%w", err), http.StatusBadRequest)
		return
	}

	url := r.PostForm.Get("url")

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
	songID := vars["song_id"]
	if songID == "" {
		s.httpError(w, fmt.Errorf("no key"), http.StatusBadRequest)
		return
	}
	err := s.db.DeleteSong(songID)
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
