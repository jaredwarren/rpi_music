package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/jaredwarren/rpi_music/downloader"
	"github.com/jaredwarren/rpi_music/log"
	"github.com/jaredwarren/rpi_music/model"
	"github.com/jaredwarren/rpi_music/player"
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

	fullData := map[string]any{
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

	sort.Slice(songs, func(i, j int) bool {
		return songs[i].CreatedAt.Before(songs[j].CreatedAt)
	})

	fullData := map[string]any{
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
	fullData := map[string]any{
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
	logger := log.NewStdLogger(log.Info)
	logger.Info("[DownloadSong] start")

	err := r.ParseForm()
	if err != nil {
		logger.Error(err.Error())
		s.httpError(w, fmt.Errorf("DownloadSong|ParseForm|%w", err), http.StatusBadRequest)
		return
	}
	logger.Info("[DownloadSong] form", log.Any("form", r.PostForm))

	url := r.PostForm.Get("url")
	force := r.PostForm.Get("force") != ""

	go func(url string, force bool) {
		logger := log.NewStdLogger(log.Info)
		song, err := s.downloadSong(url, force)
		if err != nil {
			logger.Error(err.Error())
			return
		}

		// 2. Store
		err = s.db.CreateSong(song)
		if err != nil {
			logger.Error(err.Error())
			return
		}

		// 3. Insert RFID if set
		rfid := r.PostForm.Get("rfid")
		rfid = strings.ReplaceAll(rfid, ":", "")
		if rfid != "" {
			// Make sure rfid doesn't exist yet.
			rfidSong, err := s.db.GetRFIDSong(rfid)
			if err != nil {
				logger.Error("[DownloadSong] RFIDExists error", log.Error(err))
				return
			} else if rfidSong != nil {
				logger.Error(fmt.Sprintf("[DownloadSong] rfid aready assigned! (%+v)", rfidSong))
				return
			} // else continue

			err = s.db.AddRFIDSong(rfid, song.ID)
			if err != nil {
				logger.Error("[DownloadSong] AddRFIDSong error", log.Error(err))
				return
			}
		}
	}(url, force)

	http.Redirect(w, r, "/songs", http.StatusFound)
}

func (s *Server) NewSongHandler(w http.ResponseWriter, r *http.Request) {
	err := r.ParseMultipartForm(32 << 20)
	if err != nil {
		s.logger.Error(err.Error())
		s.httpError(w, fmt.Errorf("NewSongHandler|ParseForm|%w", err), http.StatusBadRequest)
		return
	}
	s.logger.Info("NewSongHandler", log.Any("form", r.PostForm))

	url := r.PostForm.Get("url")
	force := r.PostForm.Get("force") != ""
	rfid := r.PostForm.Get("rfid")

	s.newSong(url, rfid, force)

	http.Redirect(w, r, "/songs", http.StatusFound)
}

func (s *Server) newSong(url, rfid string, force bool) {
	song, err := s.downloadSong(url, force)
	if err != nil {
		s.logger.Error(err.Error())
		return
	}

	// 2. Store
	err = s.db.UpdateSong(song)
	if err != nil {
		s.logger.Error("NewSongHandler|db.Update|%w", log.Error(err))
		return
	}

	// 3. Insert RFID if set
	rfid = strings.ReplaceAll(rfid, ":", "")
	if rfid != "" {
		// Make sure rfid doesn't exist yet.
		rfidSong, err := s.db.GetRFIDSong(rfid)
		if err != nil {
			s.logger.Error("RFIDExists error", log.Error(err))
			return
		} else if rfidSong != nil {
			s.logger.Error("rfid aready assigned!", log.Error(err))
			return
		} // else continue

		err = s.db.AddRFIDSong(rfid, song.ID)
		if err != nil {
			s.logger.Error("AddRFIDSong error", log.Error(err))
			return
		}
	}
}

var urlreg = regexp.MustCompile(`.+?(https?:)`)

func (s *Server) downloadSong(url string, force bool) (*model.Song, error) {
	logger := log.Get()
	if url == "" {
		return nil, fmt.Errorf("missing url")
	}
	url = urlreg.ReplaceAllString(url, "${1}")

	if !force {
		logger.Info("getting file", log.Any("url", url))
		// check if file exists
		filename, err := downloader.GetVideoFilename(url, logger)
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
	logger.Info("getting video", log.Any("url", url))
	file, video, err := s.downloader.DownloadVideo(url, logger)
	if err != nil {
		logger.Error(err.Error())
		return nil, fmt.Errorf("DownloadVideo|%w", err)
	}

	// 2. Download Thumb
	logger.Info("getting thumb", log.Any("url", url))
	tmb, err := s.downloader.DownloadThumb(video)
	if err != nil {
		logger.Warn("NewSongHandler|downloadThumb", log.Error(err))
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

	fullData := map[string]any{
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
