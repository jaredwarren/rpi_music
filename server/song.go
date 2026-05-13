package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"net/http"

	"github.com/jaredwarren/rpi_music/db"
	"github.com/jaredwarren/rpi_music/downloader"
	"github.com/jaredwarren/rpi_music/model"
)

func writeJSONError(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func (s *Server) JSONGetSongByRFID(w http.ResponseWriter, r *http.Request) {
	rfid := r.PathValue("rfid")
	if rfid == "" {
		writeJSONError(w, "rfid required")
		return
	}

	rfidSong, err := s.db.GetRFIDSong(rfid)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			writeJSONError(w, "rfid has no song")
			return
		}
		writeJSONError(w, err.Error())
		return
	}
	if len(rfidSong.Songs) == 0 {
		writeJSONError(w, "rfid has no song")
		return
	}

	song, err := s.db.GetSong(rfidSong.Songs[0])
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			writeJSONError(w, "song not found")
			return
		}
		writeJSONError(w, err.Error())
		return
	}
	writeJSON(w, song)
}

func (s *Server) JSONHandler(w http.ResponseWriter, r *http.Request) {
	songID := r.PathValue("song_id")
	if songID == "" {
		writeJSONError(w, "song_id required")
		return
	}

	song, err := s.db.GetSong(songID)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			writeJSONError(w, "song not found")
			return
		}
		writeJSONError(w, err.Error())
		return
	}
	writeJSON(w, song)
}

func (s *Server) EditSongFormHandler(w http.ResponseWriter, r *http.Request) {
	song, ok := s.getSongFromPath(w, r, "song_id")
	if !ok {
		return
	}
	s.render(w, r, s.templates["editSong"], map[string]any{
		"Song":      song,
		TemplateTag: template.HTML(""),
	})
}

func (s *Server) ListSongHandler(w http.ResponseWriter, r *http.Request) {
	s.logger.Info("current song", "song", s.player.GetPlaying())

	songs, err := s.listSongsWithRFID()
	if err != nil {
		s.httpError(w, fmt.Errorf("ListSongHandler|%w", err), http.StatusBadRequest)
		return
	}

	s.render(w, r, s.templates["index"], map[string]any{
		"Songs":       songs,
		"CurrentSong": s.player.GetPlaying(),
		"Player":      s.player,
		TemplateTag:   template.HTML(""),
	})
}

func (s *Server) NewSongFormHandler(w http.ResponseWriter, r *http.Request) {
	s.render(w, r, s.templates["newSong"], map[string]any{
		"Song":      model.NewSong(),
		TemplateTag: template.HTML(""),
	})
}

// DownloadSong starts a background download and immediately redirects to /songs.
func (s *Server) DownloadSong(w http.ResponseWriter, r *http.Request) {
	if err := s.DownloadSongE(w, r); err != nil {
		var httpErr *HTTPError
		if errors.As(err, &httpErr) {
			s.httpError(w, httpErr.Err, httpErr.Code)
			return
		}
		s.httpError(w, err, http.StatusInternalServerError)
	}
}

// DownloadSongE starts a background download and immediately redirects to /songs.
func (s *Server) DownloadSongE(w http.ResponseWriter, r *http.Request) error {
	if err := r.ParseForm(); err != nil {
		s.logger.Error("DownloadSong|ParseForm", "err", err)
		return asHTTPError(http.StatusBadRequest, fmt.Errorf("DownloadSong|ParseForm|%w", err))
	}

	rawURL := r.PostForm.Get("url")
	if rawURL == "" {
		return asHTTPError(http.StatusBadRequest, downloader.ErrMissingURL)
	}
	force := r.PostForm.Get("force") != ""
	rfid := normalizeRFID(r.PostForm.Get("rfid"))

	go func() {
		song, err := s.createDownloadedSong(s.ctx, rawURL, force, rfid)
		if err != nil {
			s.logger.Error("createDownloadedSong", "err", err)
			notifyDesktop("Download failed", err.Error())
			s.notifyBroadcast("Download failed", err.Error())
			return
		}
		notifyDesktop("Download complete", song.Title)
		s.notifyBroadcast("Download complete", song.Title)
	}()

	http.Redirect(w, r, "/songs", http.StatusFound)
	return nil
}

func (s *Server) NewSongHandler(w http.ResponseWriter, r *http.Request) {
	if err := s.NewSongHandlerE(w, r); err != nil {
		var httpErr *HTTPError
		if errors.As(err, &httpErr) {
			s.httpError(w, httpErr.Err, httpErr.Code)
			return
		}
		s.httpError(w, err, http.StatusInternalServerError)
	}
}

func (s *Server) NewSongHandlerE(w http.ResponseWriter, r *http.Request) error {
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		s.logger.Error("NewSongHandler|ParseForm", "err", err)
		return asHTTPError(http.StatusBadRequest, fmt.Errorf("NewSongHandler|ParseForm|%w", err))
	}

	url := r.PostForm.Get("url")
	force := r.PostForm.Get("force") != ""
	rfid := normalizeRFID(r.PostForm.Get("rfid"))

	if _, err := s.updateDownloadedSong(s.ctx, url, force, rfid); err != nil {
		s.logger.Error("NewSongHandler|updateDownloadedSong", "err", err)
	}

	http.Redirect(w, r, "/songs", http.StatusFound)
	return nil
}


// getSongFromPath reads param from path values, fetches the song, and writes an error on failure.
func (s *Server) getSongFromPath(w http.ResponseWriter, r *http.Request, param string) (*model.Song, bool) {
	key := r.PathValue(param)
	if key == "" {
		s.httpError(w, fmt.Errorf("%s required", param), http.StatusBadRequest)
		return nil, false
	}
	song, err := s.db.GetSong(key)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			s.httpError(w, fmt.Errorf("song not found"), http.StatusNotFound)
			return nil, false
		}
		s.httpError(w, fmt.Errorf("GetSong|%w", err), http.StatusInternalServerError)
		return nil, false
	}
	return song, true
}

func (s *Server) DeleteSongHandler(w http.ResponseWriter, r *http.Request) {
	if err := s.DeleteSongHandlerE(w, r); err != nil {
		var httpErr *HTTPError
		if errors.As(err, &httpErr) {
			s.httpError(w, httpErr.Err, httpErr.Code)
			return
		}
		s.httpError(w, err, http.StatusInternalServerError)
	}
}

func (s *Server) DeleteSongHandlerE(w http.ResponseWriter, r *http.Request) error {
	songID := r.PathValue("song_id")
	if songID == "" {
		return asHTTPError(http.StatusBadRequest, fmt.Errorf("song_id required"))
	}
	if err := s.db.DeleteSong(songID); err != nil {
		return asHTTPError(http.StatusInternalServerError, fmt.Errorf("DeleteSongHandler|DeleteSong|%w", err))
	}
	http.Redirect(w, r, "/songs", http.StatusFound)
	return nil
}

// RedownloadSongAssetsHandler repairs missing song assets in place.
func (s *Server) RedownloadSongAssetsHandler(w http.ResponseWriter, r *http.Request) {
	song, ok := s.getSongFromPath(w, r, "song_id")
	if !ok {
		return
	}

	if err := s.redownloadMissingAssets(song); err != nil {
		s.httpError(w, fmt.Errorf("RedownloadSongAssetsHandler|%w", err), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/songs", http.StatusFound)
}

func (s *Server) PlayVideoHandler(w http.ResponseWriter, r *http.Request) {
	song, ok := s.getSongFromPath(w, r, "song_id")
	if !ok {
		return
	}
	s.render(w, r, s.templates["playVideo"], map[string]any{
		"Song":      song,
		TemplateTag: template.HTML(""),
	})
}
