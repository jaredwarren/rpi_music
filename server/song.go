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

const (
	templateLayout    = "templates/layout.html"
	templateEditSong  = "templates/edit_song.html"
	templateIndex     = "templates/index.html"
	templateNewSong   = "templates/new_song.html"
	templatePlayVideo = "templates/play_video.html"
)

var videoURLRegex = regexp.MustCompile(`.+?(https?:)`)

// writeJSONError writes a JSON object with a single "error" key to w.
func writeJSONError(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}

// writeJSON writes v as JSON to w and sets Content-Type.
func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func (s *Server) JSONGetSongByRFID(w http.ResponseWriter, r *http.Request) {
	s.logger.Info("JSONGetSongByRFID")
	vars := mux.Vars(r)
	rfid := vars["rfid"]
	if rfid == "" {
		writeJSONError(w, "rfid required")
		return
	}

	rfidSong, err := s.db.GetRFIDSong(rfid)
	if err != nil {
		writeJSONError(w, err.Error())
		return
	}
	if rfidSong == nil || len(rfidSong.Songs) == 0 {
		writeJSONError(w, "rfid has no song")
		return
	}

	song, err := s.db.GetSong(rfidSong.Songs[0])
	if err != nil {
		writeJSONError(w, err.Error())
		return
	}
	if song == nil {
		writeJSONError(w, "song not found")
		return
	}

	writeJSON(w, song)
}

func (s *Server) JSONHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	songID := vars["song_id"]
	if songID == "" {
		writeJSONError(w, "song_id required")
		return
	}

	song, err := s.db.GetSong(songID)
	if err != nil {
		writeJSONError(w, err.Error())
		return
	}
	if song == nil {
		writeJSONError(w, "song not found")
		return
	}

	writeJSON(w, song)
}

func (s *Server) EditSongFormHandler(w http.ResponseWriter, r *http.Request) {
	song, ok := s.getSongFromVars(w, r, "song_id")
	if !ok {
		return
	}

	fullData := map[string]any{
		"Song":      song,
		TemplateTag: s.GetToken(w, r),
	}
	tpl := template.Must(template.ParseFiles(templateEditSong, templateLayout))
	s.render(w, r, tpl, fullData)
}

func (s *Server) ListSongHandler(w http.ResponseWriter, r *http.Request) {
	s.logger.Info("current song", log.Any("song", player.GetPlaying()))

	songs, err := s.db.ListSongs()
	if err != nil {
		s.httpError(w, fmt.Errorf("ListSongHandler|ListSongs|%w", err), http.StatusBadRequest)
		return
	}

	rfidList, err := s.db.ListRFIDSongs()
	if err != nil {
		s.httpError(w, fmt.Errorf("ListSongHandler|ListRFIDSongs|%w", err), http.StatusBadRequest)
		return
	}

	enrichSongsWithRFID(songs, rfidList)
	sort.Slice(songs, func(i, j int) bool {
		return songs[i].CreatedAt.Before(songs[j].CreatedAt)
	})

	fullData := map[string]any{
		"Songs":       songs,
		"CurrentSong": player.GetPlaying(),
		"Player":      player.GetPlayer(),
		TemplateTag:   s.GetToken(w, r),
	}
	tpl := template.Must(template.ParseFiles(templateIndex, templateLayout))
	s.render(w, r, tpl, fullData)
}

// enrichSongsWithRFID sets each song's RFID field when it appears in the RFID list.
func enrichSongsWithRFID(songs []*model.Song, rfidList []*model.RFIDSong) {
	for _, song := range songs {
		for _, rfidEntry := range rfidList {
			for _, linkedID := range rfidEntry.Songs {
				if linkedID == song.ID {
					song.RFID = rfidEntry.RFID
					break
				}
			}
		}
	}
}

func (s *Server) NewSongFormHandler(w http.ResponseWriter, r *http.Request) {
	fullData := map[string]any{
		"Song":      model.NewSong(),
		TemplateTag: s.GetToken(w, r),
	}
	tpl := template.Must(template.New("base").ParseFiles(templateNewSong, templateLayout))
	s.render(w, r, tpl, fullData)
}

// DownloadSong starts a background download and immediately redirects to /songs.
// Form: url (required), force (optional), rfid (optional).
func (s *Server) DownloadSong(w http.ResponseWriter, r *http.Request) {
	logger := log.NewStdLogger(log.Info)
	logger.Info("[DownloadSong] start")

	if err := r.ParseForm(); err != nil {
		logger.Error(err.Error())
		s.httpError(w, fmt.Errorf("DownloadSong|ParseForm|%w", err), http.StatusBadRequest)
		return
	}
	logger.Info("[DownloadSong] form", log.Any("form", r.PostForm))

	url := r.PostForm.Get("url")
	force := r.PostForm.Get("force") != ""
	rfid := normalizeRFID(r.PostForm.Get("rfid"))

	go func() {
		song, err := s.downloadSong(url, force)
		if err != nil {
			logger.Error(err.Error())
			return
		}
		if err := s.db.CreateSong(song); err != nil {
			logger.Error(err.Error())
			return
		}
		s.tryAssignRFID(rfid, song.ID, logger)
	}()

	http.Redirect(w, r, "/songs", http.StatusFound)
}

func (s *Server) NewSongHandler(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		s.logger.Error(err.Error())
		s.httpError(w, fmt.Errorf("NewSongHandler|ParseForm|%w", err), http.StatusBadRequest)
		return
	}
	s.logger.Info("NewSongHandler", log.Any("form", r.PostForm))

	url := r.PostForm.Get("url")
	force := r.PostForm.Get("force") != ""
	rfid := normalizeRFID(r.PostForm.Get("rfid"))

	s.createAndStoreSong(url, rfid, force)
	http.Redirect(w, r, "/songs", http.StatusFound)
}

// createAndStoreSong downloads a song, stores it via UpdateSong, and optionally assigns an RFID.
func (s *Server) createAndStoreSong(url, rfid string, force bool) {
	song, err := s.downloadSong(url, force)
	if err != nil {
		s.logger.Error(err.Error())
		return
	}
	if err := s.db.UpdateSong(song); err != nil {
		s.logger.Error("createAndStoreSong|UpdateSong", log.Error(err))
		return
	}
	s.tryAssignRFID(rfid, song.ID, s.logger)
}

// normalizeRFID removes colons from an RFID string (e.g. "AB:CD:EF" -> "ABCDEF").
func normalizeRFID(rfid string) string {
	return strings.ReplaceAll(rfid, ":", "")
}

// tryAssignRFID assigns the given RFID to the song if rfid is non-empty and not already assigned.
// Errors are logged; the caller can ignore the return value.
func (s *Server) tryAssignRFID(rfid, songID string, logger log.Logger) {
	if rfid == "" {
		return
	}
	existing, err := s.db.GetRFIDSong(rfid)
	if err != nil {
		logger.Error("tryAssignRFID|GetRFIDSong", log.Error(err))
		return
	}
	if existing != nil {
		logger.Error("tryAssignRFID|rfid already assigned", log.Any("rfidSong", existing))
		return
	}
	if err := s.db.AddRFIDSong(rfid, songID); err != nil {
		logger.Error("tryAssignRFID|AddRFIDSong", log.Error(err))
	}
}

func (s *Server) downloadSong(rawURL string, force bool) (*model.Song, error) {
	logger := log.Get()
	if rawURL == "" {
		return nil, fmt.Errorf("missing url")
	}
	url := normalizeVideoURL(rawURL)

	if !force {
		if err := s.checkAlreadyDownloaded(url, logger); err != nil {
			return nil, err
		}
	}

	filePath, video, err := s.downloader.DownloadVideo(url, logger)
	if err != nil {
		logger.Error(err.Error())
		return nil, fmt.Errorf("DownloadVideo|%w", err)
	}

	thumb, _ := s.downloader.DownloadThumb(video) // best-effort; empty string on error

	return &model.Song{
		ID:        uuid.New().String(),
		URL:       url,
		Thumbnail: thumb,
		FilePath:  filePath,
		Title:     video.Title,
	}, nil
}

// normalizeVideoURL extracts the scheme (http: or https:) from a URL string.
func normalizeVideoURL(url string) string {
	return videoURLRegex.ReplaceAllString(url, "${1}")
}

// checkAlreadyDownloaded returns an error if the video for url is already on disk.
func (s *Server) checkAlreadyDownloaded(url string, logger log.Logger) error {
	logger.Info("getting file", log.Any("url", url))
	filename, err := downloader.GetVideoFilename(url, logger)
	if err != nil {
		return err
	}
	_, err = os.Stat(filename)
	if err == nil {
		return fmt.Errorf("file already downloaded")
	}
	if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// getSongFromVars reads "song_id" from route vars, loads the song, and writes an error response if missing or not found.
// It returns (song, true) on success and (nil, false) after writing an error.
func (s *Server) getSongFromVars(w http.ResponseWriter, r *http.Request, param string) (*model.Song, bool) {
	key := mux.Vars(r)[param]
	if key == "" {
		s.httpError(w, fmt.Errorf("%s required", param), http.StatusBadRequest)
		return nil, false
	}
	song, err := s.db.GetSong(key)
	if err != nil {
		s.httpError(w, fmt.Errorf("GetSong|%w", err), http.StatusBadRequest)
		return nil, false
	}
	if song == nil {
		s.httpError(w, fmt.Errorf("song not found"), http.StatusBadRequest)
		return nil, false
	}
	return song, true
}

func (s *Server) DeleteSongHandler(w http.ResponseWriter, r *http.Request) {
	songID := mux.Vars(r)["song_id"]
	if songID == "" {
		s.httpError(w, fmt.Errorf("song_id required"), http.StatusBadRequest)
		return
	}
	if err := s.db.DeleteSong(songID); err != nil {
		s.httpError(w, fmt.Errorf("DeleteSongHandler|DeleteSong|%w", err), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/songs", http.StatusFound)
}

func (s *Server) PlayVideoHandler(w http.ResponseWriter, r *http.Request) {
	s.logger.Info("PlayVideoHandler")
	song, ok := s.getSongFromVars(w, r, "song_id")
	if !ok {
		return
	}
	fullData := map[string]any{
		"Song":      song,
		TemplateTag: s.GetToken(w, r),
	}
	tpl := template.Must(template.New("base").Funcs(template.FuncMap{}).ParseFiles(templatePlayVideo, templateLayout))
	s.render(w, r, tpl, fullData)
}
