package downloader

import (
	"fmt"
	"os"
	"regexp"

	"github.com/kkdai/youtube/v2"
)

func (d *YoutubeDLDownloader) DownloadThumb(video *youtube.Video) (string, error) {
	// download video
	filename, err := downloadVideoThumb(video.ID)
	if err != nil {
		return "", fmt.Errorf("download video thumb error: %w", err)
	}

	// validate that file exists
	if filename == "" {
		return "", fmt.Errorf("could not get thumb filename")
	}
	if _, err := os.Stat(filename); err != nil {
		return "", fmt.Errorf("os.stat error: %w", err)
	}

	return filename, nil
}

var (
	downloadVideoThumbCmd = NewDLCommand("yt-dlp --write-thumbnail --ignore-errors --no-call-home --no-cache-dir --restrict-filenames --skip-download -o thumb_files/%(title)s-%(id)s")
	thumbRegex            = regexp.MustCompile(`Writing .+? to: (.+?)(\n|$)`)
)

func downloadVideoThumb(videoID string) (string, error) {
	outStr, err := downloadVideoThumbCmd.Exec(videoID)
	if err != nil {
		return "", err
	}

	// parse output because I can't find a better way to get thumb name
	result := thumbRegex.FindStringSubmatch(outStr)
	if len(result) > 1 {
		filename := result[1]
		if filename != "" {
			return filename, nil
		}
	}

	return "", fmt.Errorf("couldn't find thumb file")
}
