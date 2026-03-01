package downloader

import (
	"fmt"
	"regexp"
)

var (
	downloadVideoCmd = NewDLCommand("yt-dlp --no-call-home --no-cache-dir --restrict-filenames --audio-quality 0 -o song_files/%(title)s-%(id)s.%(ext)s")
	matchRegex       = regexp.MustCompile(`\[Merger\] Merging formats into "song_files/(.+?)"`)
)

func downloadVideo(videoID string) (string, error) {
	std, err := downloadVideoCmd.Exec(videoID)
	if err != nil {
		return "", err
	}

	// Try to match filename from raw output, because GetVideoFilename is currently broken
	// [Merger] Merging formats into "song_files/The_Bare_Necessities_from_The_Jungle_Book-08NlhjpVFsU.mp4"
	matches := matchRegex.FindStringSubmatch(std)
	if len(matches) > 1 {
		filename := matches[1]
		if filename != "" {
			return "song_files/" + filename, nil
		}
	}

	return "", fmt.Errorf("couldn't find file")
}
