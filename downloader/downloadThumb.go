package downloader

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/kkdai/youtube/v2"
)

// thumbOutputRegex captures the path from yt-dlp's "Writing ... to: path" line.
var thumbOutputRegex = regexp.MustCompile(`Writing .+? to: (.+?)(\n|$)`)

func (d *YoutubeDLDownloader) DownloadThumb(video *youtube.Video) (string, error) {
	cfg := d.config()
	thumbRoot := cfg.thumbRoot()

	filename, err := downloadVideoThumb(video.ID, thumbRoot, cfg)
	if err != nil {
		return "", fmt.Errorf("download thumb: %w", err)
	}
	if filename == "" {
		return "", fmt.Errorf("could not get thumb filename")
	}
	if _, err := os.Stat(filename); err != nil {
		return "", fmt.Errorf("thumb file stat: %w", err)
	}
	return filename, nil
}

func downloadVideoThumb(videoID, thumbRoot string, cfg *YoutubeDLConfig) (string, error) {
	absRoot := absPath(thumbRoot)
	cmd := cfg.newDownloadCmd([]string{
		"--write-thumbnail", "--ignore-errors", "--no-call-home", "--no-cache-dir",
		"--restrict-filenames", "--skip-download",
		"-o", filepath.Join(absRoot, "%(title)s-%(id)s"),
	}, absRoot)
	out, err := cmd.Exec(videoID)
	if err != nil {
		return "", err
	}
	matches := thumbOutputRegex.FindStringSubmatch(out)
	if len(matches) > 1 && matches[1] != "" {
		thumbPath := strings.TrimSpace(matches[1])
		return cfg.translatePath(thumbPath, absRoot), nil
	}
	return "", fmt.Errorf("could not parse thumb path from yt-dlp output")
}
