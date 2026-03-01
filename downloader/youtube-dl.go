package downloader

import (
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
type YoutubeDLDownloader struct{}

func (d *YoutubeDLDownloader) GetVideo(videoID string) (*youtube.Video, error) {
	return &youtube.Video{ID: videoID}, nil
}

func (d *YoutubeDLDownloader) DownloadVideo(videoID string, _ log.Logger) (string, *youtube.Video, error) {
	videoID = normalizeVideoID(videoID)
	logger := log.Get()
	logger.Info("DownloadVideo", log.Any("videoID", videoID))

	video := &youtube.Video{ID: videoID}
	var filename string
	var wg sync.WaitGroup

	// Fetch title and download audio in parallel.
	wg.Add(1)
	go func() {
		defer wg.Done()
		if info, err := getVideoInfo(videoID); err == nil {
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
		filename, err = downloadVideo(videoID)
		if err != nil {
			logger.Error("downloadVideo", log.Error(err))
		}
	}()

	wg.Wait()

	if filename == "" {
		if latest, err := getNewestFile(getSongRoot()); err == nil {
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

// normalizeVideoID rewrites music.youtube URLs to the standard youtube domain.
func normalizeVideoID(videoID string) string {
	return strings.Replace(videoID, "//music.", "//", 1)
}

var getVideoInfoArgs = []string{
	"--ignore-errors", "--no-call-home", "--no-cache-dir",
	"--skip-download", "--restrict-filenames", "-J",
}

func getVideoInfo(videoID string) (map[string]any, error) {
	cmd := NewDLCommandFromArgs(DefaultYtDlpBinary, getVideoInfoArgs)
	out, err := cmd.ExecB(videoID)
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

// GetVideoFilename returns the path where yt-dlp would save the audio for the given video.
// It runs yt-dlp with --get-filename and no actual download.
func GetVideoFilename(videoID string, _ log.Logger) (string, error) {
	dir := getSongRoot()
	cmd := NewDLCommandFromArgs(DefaultYtDlpBinary, []string{
		"--ignore-errors", "--no-call-home", "--no-cache-dir",
		"--skip-download", "--restrict-filenames",
		"-f", "bestaudio", "--get-filename",
		"-o", filepath.Join(dir, "%(title)s-%(id)s.%(ext)s"),
	})
	out, err := cmd.ExecB(videoID)
	if err != nil {
		return "", fmt.Errorf("GetVideoFilename: %w", err)
	}
	return strings.TrimSpace(strings.Trim(string(out), `"`)), nil
}
