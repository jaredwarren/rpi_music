package downloader

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/kkdai/youtube/v2"
	"github.com/rs/zerolog"
)

// YoutubeDLDownloader downloads audio and thumbnails using the yt-dlp CLI (or Docker fallback).
type YoutubeDLDownloader struct {
	cfg *YoutubeDLConfig
}

// NewYoutubeDLDownloader returns a new yt-dlp-based downloader with the given config.
func NewYoutubeDLDownloader(cfg *YoutubeDLConfig) *YoutubeDLDownloader {
	return &YoutubeDLDownloader{cfg: cfg}
}

func (d *YoutubeDLDownloader) config() *YoutubeDLConfig {
	if d.cfg == nil {
		return &YoutubeDLConfig{}
	}
	return d.cfg
}

// EnsureAvailable returns an error if neither yt-dlp nor Docker is available.
// Sets the Docker fallback flag on the config if Docker will be used.
func (d *YoutubeDLDownloader) EnsureAvailable() error {
	return EnsureYtDlpAvailable(d.config())
}

// BackendDescription returns a string describing the active download backend.
func (d *YoutubeDLDownloader) BackendDescription() string {
	return d.config().BackendDescription()
}

func (d *YoutubeDLDownloader) GetVideo(videoID string) (*youtube.Video, error) {
	return &youtube.Video{ID: videoID}, nil
}

func (d *YoutubeDLDownloader) DownloadVideo(ctx context.Context, videoID string, logger zerolog.Logger) (string, *youtube.Video, error) {
	videoID = normalizeVideoID(videoID)
	logger.Info().Str("videoID", videoID).Msg("DownloadVideo")

	cfg := d.config()
	songRoot := cfg.songRoot()

	video := &youtube.Video{ID: videoID}
	var filename string
	var downloadErr error
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		if info, err := getVideoInfo(ctx, videoID, cfg); err == nil {
			if t, ok := info["title"].(string); ok {
				video.Title = t
			}
		} else {
			logger.Error().Err(err).Msg("getVideoInfo")
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		filename, downloadErr = downloadVideo(ctx, videoID, songRoot, cfg)
	}()

	wg.Wait()

	if filename == "" {
		if latest, err := getNewestFile(songRoot); err == nil {
			logger.Info().Str("file", latest).Msg("getNewestFile fallback")
			filename = latest
		} else {
			if downloadErr != nil {
				logger.Error().Err(downloadErr).Msg("downloadVideo")
			}
			return "", nil, fmt.Errorf("could not get filename")
		}
		if downloadErr != nil {
			logger.Warn().Err(downloadErr).Str("file", filename).Msg("downloadVideo parse failed; fallback file used")
		}
	}
	if _, err := os.Stat(filename); err != nil {
		logger.Error().Str("filename", filename).Err(err).Msg("os.Stat")
		return "", nil, err
	}

	return filename, video, nil
}

func (d *YoutubeDLDownloader) GetVideoFilename(ctx context.Context, videoID string, _ zerolog.Logger) (string, error) {
	cfg := d.config()
	absRoot := absPath(cfg.songRoot())
	cmd := cfg.newDownloadCmd([]string{
		"--ignore-errors", "--no-call-home", "--no-cache-dir",
		"--skip-download", "--restrict-filenames",
		"-f", "bestaudio", "--get-filename",
		"-o", filepath.Join(absRoot, "%(title)s-%(id)s.%(ext)s"),
	}, absRoot)

	out, err := cmd.ExecBContext(ctx, videoID)
	if err != nil {
		return "", fmt.Errorf("GetVideoFilename: %w", err)
	}
	path := strings.TrimSpace(strings.Trim(string(out), `"`))
	return cfg.translatePath(path, absRoot), nil
}

// normalizeVideoID rewrites music.youtube URLs to the standard youtube domain.
func normalizeVideoID(videoID string) string {
	return strings.Replace(videoID, "//music.", "//", 1)
}

var getVideoInfoArgs = []string{
	"--ignore-errors", "--no-call-home", "--no-cache-dir",
	"--skip-download", "--restrict-filenames", "-J",
}

func getVideoInfo(ctx context.Context, videoID string, cfg *YoutubeDLConfig) (map[string]any, error) {
	cmd := cfg.newMetaCmd(getVideoInfoArgs)
	out, err := cmd.ExecBContext(ctx, videoID)
	if err != nil {
		return nil, fmt.Errorf("getVideoInfo: %w", err)
	}
	var info map[string]any
	if err := json.Unmarshal(out, &info); err != nil {
		return nil, fmt.Errorf("getVideoInfo json: %w", err)
	}
	return info, nil
}

func getNewestFile(dir string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	var newestName string
	var newestTime int64
	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			continue
		}
		if t := info.ModTime().Unix(); t > newestTime {
			newestTime = t
			newestName = info.Name()
		}
	}
	if newestName == "" {
		return "", fmt.Errorf("no files in %s", dir)
	}
	return filepath.Join(dir, newestName), nil
}
