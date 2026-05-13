package server

import (
	"context"
	"errors"
	"fmt"
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

func (s *Server) listSongsWithRFID() ([]*model.Song, error) {
	songs, err := s.db.ListSongs()
	if err != nil {
		return nil, fmt.Errorf("ListSongs|%w", err)
	}
	rfidList, err := s.db.ListRFIDSongs()
	if err != nil {
		return nil, fmt.Errorf("ListRFIDSongs|%w", err)
	}
	enrichSongsWithRFID(songs, rfidList)
	sort.Slice(songs, func(i, j int) bool {
		return songs[i].CreatedAt.After(songs[j].CreatedAt)
	})
	return songs, nil
}

func (s *Server) createDownloadedSong(ctx context.Context, rawURL string, force bool, rfid string) (*model.Song, error) {
	song, err := s.downloadSong(ctx, rawURL, force)
	if err != nil {
		return nil, fmt.Errorf("downloadSong|%w", err)
	}
	if err := s.db.CreateSong(song); err != nil {
		return nil, fmt.Errorf("CreateSong|%w", err)
	}
	s.tryAssignRFID(rfid, song.ID)
	return song, nil
}

func (s *Server) updateDownloadedSong(ctx context.Context, rawURL string, force bool, rfid string) (*model.Song, error) {
	song, err := s.downloadSong(ctx, rawURL, force)
	if err != nil {
		return nil, fmt.Errorf("downloadSong|%w", err)
	}
	if err := s.db.UpdateSong(song); err != nil {
		return nil, fmt.Errorf("UpdateSong|%w", err)
	}
	s.tryAssignRFID(rfid, song.ID)
	return song, nil
}

func (s *Server) redownloadMissingAssets(song *model.Song) error {
	videoMissing := pathMissing(song.FilePath)
	thumbMissing := pathMissing(song.Thumbnail)
	if !videoMissing && !thumbMissing {
		return nil
	}

	if videoMissing {
		filePath, video, err := s.downloader.DownloadVideo(s.ctx, song.URL, s.logger)
		if err != nil {
			return fmt.Errorf("DownloadVideo|%w", err)
		}
		song.FilePath = normalizeAssetPath(filePath, s.songAssetRoot())
		if video != nil && video.Title != "" {
			song.Title = video.Title
		}
		if thumbMissing {
			thumb, err := s.downloader.DownloadThumb(video)
			if err != nil {
				return fmt.Errorf("DownloadThumb|%w", err)
			}
			song.Thumbnail = normalizeAssetPath(thumb, s.thumbAssetRoot())
			thumbMissing = false
		}
	}

	if thumbMissing {
		video, err := s.downloader.GetVideo(song.URL)
		if err != nil {
			return fmt.Errorf("GetVideo|%w", err)
		}
		thumb, err := s.downloader.DownloadThumb(video)
		if err != nil {
			return fmt.Errorf("DownloadThumb|%w", err)
		}
		song.Thumbnail = normalizeAssetPath(thumb, s.thumbAssetRoot())
	}

	if err := s.db.UpdateSong(song); err != nil {
		return fmt.Errorf("UpdateSong|%w", err)
	}
	return nil
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
