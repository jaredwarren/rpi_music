package downloader

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

// mergerOutputRegex captures the output path from yt-dlp's "[Merger] Merging formats into \"path\"" line (stderr).
var mergerOutputRegex = regexp.MustCompile(`\[Merger\] Merging formats into "(.+?)"`)
var destinationOutputRegex = regexp.MustCompile(`(?:\[download\]|\[ExtractAudio\])\s+Destination:\s+(.+?)(\n|$)`)

func downloadVideo(ctx context.Context, videoID, songRoot string, cfg *YoutubeDLConfig) (string, error) {
	absRoot := absPath(songRoot)
	cmd := cfg.newDownloadCmd([]string{
		"--no-call-home", "--no-cache-dir", "--restrict-filenames",
		"--audio-quality", "0",
		"-o", filepath.Join(absRoot, "%(title)s-%(id)s.%(ext)s"),
	}, absRoot)

	out, err := cmd.ExecCombinedContext(ctx, videoID)
	if err != nil {
		return "", err
	}

	if path := parseDownloadOutputPath(string(out)); path != "" {
		return cfg.translatePath(path, absRoot), nil
	}
	return "", fmt.Errorf("could not parse output path from yt-dlp")
}

func parseDownloadOutputPath(out string) string {
	regexes := []*regexp.Regexp{
		mergerOutputRegex,
		destinationOutputRegex,
	}
	for _, re := range regexes {
		matches := re.FindAllStringSubmatch(out, -1)
		for i := len(matches) - 1; i >= 0; i-- {
			if len(matches[i]) > 1 {
				path := strings.TrimSpace(matches[i][1])
				path = strings.Trim(path, `"`)
				if path != "" {
					return path
				}
			}
		}
	}
	return ""
}
