package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"

	"github.com/google/uuid"
	"github.com/jaredwarren/rpi_music/db"
	"github.com/jaredwarren/rpi_music/downloader"
	"github.com/jaredwarren/rpi_music/model"
)

var videoURLRegex = regexp.MustCompile(`.+?(https?:)`)

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
		return songs[i].CreatedAt.After(songs[j].CreatedAt)
	})

	s.render(w, r, s.templates["index"], map[string]any{
		"Songs":       songs,
		"CurrentSong": s.player.GetPlaying(),
		"Player":      s.player,
		TemplateTag:   template.HTML(""),
	})
}

// enrichSongsWithRFID sets each song's RFID field in a single O(n) pass.
func enrichSongsWithRFID(songs []*model.Song, rfidList []*model.RFIDSong) {
	songIDToRFID := make(map[string]string, len(rfidList))
	for _, entry := range rfidList {
		for _, id := range entry.Songs {
			songIDToRFID[id] = entry.RFID
		}
	}
	for _, song := range songs {
		if rfid, ok := songIDToRFID[song.ID]; ok {
			song.RFID = rfid
		}
	}
}

func (s *Server) NewSongFormHandler(w http.ResponseWriter, r *http.Request) {
	s.render(w, r, s.templates["newSong"], map[string]any{
		"Song":      model.NewSong(),
		TemplateTag: template.HTML(""),
	})
}

// DownloadSong starts a background download and immediately redirects to /songs.
func (s *Server) DownloadSong(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		s.logger.Error("DownloadSong|ParseForm", "err", err)
		s.httpError(w, fmt.Errorf("DownloadSong|ParseForm|%w", err), http.StatusBadRequest)
		return
	}

	rawURL := r.PostForm.Get("url")
	if rawURL == "" {
		s.httpError(w, downloader.ErrMissingURL, http.StatusBadRequest)
		return
	}
	force := r.PostForm.Get("force") != ""
	rfid := normalizeRFID(r.PostForm.Get("rfid"))

	go func() {
		song, err := s.downloadSong(s.ctx, rawURL, force)
		if err != nil {
			s.logger.Error("downloadSong", "err", err)
			notifyDesktop("Download failed", err.Error())
			s.notifyBroadcast("Download failed", err.Error())
			return
		}
		if err := s.db.CreateSong(song); err != nil {
			s.logger.Error("CreateSong", "err", err)
			notifyDesktop("Download failed", err.Error())
			s.notifyBroadcast("Download failed", err.Error())
			return
		}
		s.tryAssignRFID(rfid, song.ID)
		notifyDesktop("Download complete", song.Title)
		s.notifyBroadcast("Download complete", song.Title)
	}()

	http.Redirect(w, r, "/songs", http.StatusFound)
}

func (s *Server) NewSongHandler(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		s.logger.Error("NewSongHandler|ParseForm", "err", err)
		s.httpError(w, fmt.Errorf("NewSongHandler|ParseForm|%w", err), http.StatusBadRequest)
		return
	}

	url := r.PostForm.Get("url")
	force := r.PostForm.Get("force") != ""
	rfid := normalizeRFID(r.PostForm.Get("rfid"))

	song, err := s.downloadSong(s.ctx, url, force)
	if err != nil {
		s.logger.Error("NewSongHandler|downloadSong", "err", err)
	} else if err := s.db.UpdateSong(song); err != nil {
		s.logger.Error("NewSongHandler|UpdateSong", "err", err)
	} else {
		s.tryAssignRFID(rfid, song.ID)
	}

	http.Redirect(w, r, "/songs", http.StatusFound)
}

func normalizeRFID(rfid string) string {
	return strings.ReplaceAll(rfid, ":", "")
}

func notifyDesktop(title, body string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "linux":
		cmd = exec.Command("notify-send", title, body)
	case "darwin":
		bodyEsc := strings.ReplaceAll(strings.ReplaceAll(body, `\`, `\\`), `"`, `\"`)
		titleEsc := strings.ReplaceAll(strings.ReplaceAll(title, `\`, `\\`), `"`, `\"`)
		script := fmt.Sprintf("display notification \"%s\" with title \"%s\"", bodyEsc, titleEsc)
		cmd = exec.Command("osascript", "-e", script)
	default:
		return
	}
	_ = cmd.Run()
}

func (s *Server) tryAssignRFID(rfid, songID string) {
	if rfid == "" {
		return
	}
	existing, err := s.db.GetRFIDSong(rfid)
	if err != nil && !errors.Is(err, db.ErrNotFound) {
		s.logger.Error("tryAssignRFID|GetRFIDSong", "err", err)
		return
	}
	if existing != nil {
		s.logger.Error("tryAssignRFID|rfid already assigned", "rfidSong", existing)
		return
	}
	if err := s.db.AddRFIDSong(rfid, songID); err != nil {
		s.logger.Error("tryAssignRFID|AddRFIDSong", "err", err)
	}
}

func (s *Server) downloadSong(ctx context.Context, rawURL string, force bool) (*model.Song, error) {
	if rawURL == "" {
		return nil, downloader.ErrMissingURL
	}
	url := normalizeVideoURL(rawURL)

	if !force {
		if err := s.checkAlreadyDownloaded(ctx, url); err != nil {
			return nil, err
		}
	}

	filePath, video, err := s.downloader.DownloadVideo(ctx, url, s.logger)
	if err != nil {
		return nil, fmt.Errorf("DownloadVideo|%w", err)
	}

	thumb, _ := s.downloader.DownloadThumb(video)

	return &model.Song{
		ID:        uuid.New().String(),
		URL:       url,
		Thumbnail: thumb,
		FilePath:  filePath,
		Title:     video.Title,
	}, nil
}

func normalizeVideoURL(url string) string {
	return videoURLRegex.ReplaceAllString(url, "${1}")
}

func (s *Server) checkAlreadyDownloaded(ctx context.Context, url string) error {
	filename, err := s.downloader.GetVideoFilename(ctx, url, s.logger)
	if err != nil {
		return err
	}
	if filename == "" {
		return nil
	}
	_, err = os.Stat(filename)
	if err == nil {
		return downloader.ErrAlreadyExists
	}
	if !errors.Is(err, os.ErrNotExist) {
		return err
	}
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
	songID := r.PathValue("song_id")
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

// RedownloadSongAssetsHandler repairs missing song assets in place.
func (s *Server) RedownloadSongAssetsHandler(w http.ResponseWriter, r *http.Request) {
	song, ok := s.getSongFromPath(w, r, "song_id")
	if !ok {
		return
	}

	videoMissing := pathMissing(song.FilePath)
	thumbMissing := pathMissing(song.Thumbnail)
	if !videoMissing && !thumbMissing {
		http.Redirect(w, r, "/songs", http.StatusFound)
		return
	}

	if videoMissing {
		filePath, video, err := s.downloader.DownloadVideo(s.ctx, song.URL, s.logger)
		if err != nil {
			s.httpError(w, fmt.Errorf("RedownloadSongAssetsHandler|DownloadVideo|%w", err), http.StatusInternalServerError)
			return
		}
		song.FilePath = normalizeAssetPath(filePath, s.songAssetRoot())
		if video != nil && video.Title != "" {
			song.Title = video.Title
		}
		if thumbMissing {
			thumb, err := s.downloader.DownloadThumb(video)
			if err != nil {
				s.httpError(w, fmt.Errorf("RedownloadSongAssetsHandler|DownloadThumb|%w", err), http.StatusInternalServerError)
				return
			}
			song.Thumbnail = normalizeAssetPath(thumb, s.thumbAssetRoot())
			thumbMissing = false
		}
	}

	if thumbMissing {
		video, err := s.downloader.GetVideo(song.URL)
		if err != nil {
			s.httpError(w, fmt.Errorf("RedownloadSongAssetsHandler|GetVideo|%w", err), http.StatusInternalServerError)
			return
		}
		thumb, err := s.downloader.DownloadThumb(video)
		if err != nil {
			s.httpError(w, fmt.Errorf("RedownloadSongAssetsHandler|DownloadThumb|%w", err), http.StatusInternalServerError)
			return
		}
		song.Thumbnail = normalizeAssetPath(thumb, s.thumbAssetRoot())
	}

	if err := s.db.UpdateSong(song); err != nil {
		s.httpError(w, fmt.Errorf("RedownloadSongAssetsHandler|UpdateSong|%w", err), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/songs", http.StatusFound)
}

func pathMissing(path string) bool {
	if path == "" {
		return true
	}
	_, err := os.Stat(path)
	return errors.Is(err, os.ErrNotExist)
}

func (s *Server) songAssetRoot() string {
	if s.cfg != nil && s.cfg.Player.SongRoot != "" {
		return s.cfg.Player.SongRoot
	}
	return "song_files"
}

func (s *Server) thumbAssetRoot() string {
	if s.cfg != nil && s.cfg.Player.ThumbRoot != "" {
		return s.cfg.Player.ThumbRoot
	}
	return "thumb_files"
}

func normalizeAssetPath(path, root string) string {
	if path == "" {
		return ""
	}
	normalizedRoot := filepath.Clean(root)
	cleanPath := filepath.Clean(path)
	normalizedRootSlash := filepath.ToSlash(normalizedRoot)

	if filepath.IsAbs(cleanPath) {
		if absRoot, err := filepath.Abs(normalizedRoot); err == nil {
			if rel, relErr := filepath.Rel(absRoot, cleanPath); relErr == nil && rel != "." && !strings.HasPrefix(rel, "..") {
				return filepath.ToSlash(filepath.Join(normalizedRoot, rel))
			}
		}
	}

	pathSlash := filepath.ToSlash(cleanPath)
	if pathSlash == normalizedRootSlash || strings.HasPrefix(pathSlash, normalizedRootSlash+"/") {
		return pathSlash
	}
	return filepath.ToSlash(filepath.Join(normalizedRoot, filepath.Base(cleanPath)))
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
