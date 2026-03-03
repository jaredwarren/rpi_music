package downloader

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/jaredwarren/rpi_music/log"
	"github.com/kkdai/youtube/v2"
)

// YoutubeDLDownloader downloads audio and thumbnails using the yt-dlp CLI.
// Configure via YoutubeDLConfig; use NewYoutubeDLDownloader to construct.
type YoutubeDLDownloader struct {
	cfg *YoutubeDLConfig
}

// NewYoutubeDLDownloader returns a new yt-dlp-based downloader with the given config.
// cfg may be nil to use viper defaults (player.song_root, player.thumb_root) and DefaultYtDlpBinary.
func NewYoutubeDLDownloader(cfg *YoutubeDLConfig) *YoutubeDLDownloader {
	return &YoutubeDLDownloader{cfg: cfg}
}

func (d *YoutubeDLDownloader) config() *YoutubeDLConfig {
	if d.cfg == nil {
		return &YoutubeDLConfig{}
	}
	return d.cfg
}

// EnsureAvailable returns an error if the yt-dlp binary is not in PATH.
// Call at startup to fail fast instead of on first download.
func (d *YoutubeDLDownloader) EnsureAvailable() error {
	return EnsureYtDlpAvailable(d.config())
}

func (d *YoutubeDLDownloader) GetVideo(videoID string) (*youtube.Video, error) {
	return &youtube.Video{ID: videoID}, nil
}

func (d *YoutubeDLDownloader) DownloadVideo(ctx context.Context, videoID string, logger log.Logger) (string, *youtube.Video, error) {
	videoID = normalizeVideoID(videoID)
	logger.Info("DownloadVideo", log.Any("videoID", videoID))

	songRoot := d.config().songRoot()
	binary := d.config().binary()

	video := &youtube.Video{ID: videoID}
	var filename string
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		if info, err := getVideoInfo(ctx, videoID, binary); err == nil {
			if t, ok := info["title"].(string); ok {
				video.Title = t
			}
		} else {
			logger.Error("getVideoInfo", log.Error(err))
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		var err error
		filename, err = downloadVideo(ctx, videoID, songRoot, binary)
		if err != nil {
			logger.Error("downloadVideo", log.Error(err))
		}
	}()

	wg.Wait()

	if filename == "" {
		if latest, err := getNewestFile(songRoot); err == nil {
			logger.Info("getNewestFile fallback", log.Any("file", latest))
		}
		return "", nil, fmt.Errorf("could not get filename")
	}
	if _, err := os.Stat(filename); err != nil {
		logger.Error("os.Stat", log.Any("filename", filename), log.Error(err))
		return "", nil, err
	}

	return filename, video, nil
}

func (d *YoutubeDLDownloader) GetVideoFilename(ctx context.Context, videoID string, _ log.Logger) (string, error) {
	return getVideoFilename(ctx, videoID, d.config().songRoot(), d.config().binary())
}

// normalizeVideoID rewrites music.youtube URLs to the standard youtube domain.
func normalizeVideoID(videoID string) string {
	return strings.Replace(videoID, "//music.", "//", 1)
}

var getVideoInfoArgs = []string{
	"--ignore-errors", "--no-call-home", "--no-cache-dir",
	"--skip-download", "--restrict-filenames", "-J",
}

func getVideoInfo(ctx context.Context, videoID string, binary string) (map[string]any, error) {
	cmd := NewDLCommandFromArgs(binary, getVideoInfoArgs)
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

func getVideoFilename(ctx context.Context, videoID string, songRoot, binary string) (string, error) {
	cmd := NewDLCommandFromArgs(binary, []string{
		"--ignore-errors", "--no-call-home", "--no-cache-dir",
		"--skip-download", "--restrict-filenames",
		"-f", "bestaudio", "--get-filename",
		"-o", filepath.Join(songRoot, "%(title)s-%(id)s.%(ext)s"),
	})
	out, err := cmd.ExecBContext(ctx, videoID)
	if err != nil {
		return "", fmt.Errorf("GetVideoFilename: %w", err)
	}
	return strings.TrimSpace(strings.Trim(string(out), `"`)), nil
}
