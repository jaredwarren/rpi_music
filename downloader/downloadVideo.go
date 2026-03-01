package downloader

import (
	"fmt"
	"path/filepath"
	"regexp"
)

// mergerOutputRegex captures the output path from yt-dlp's "[Merger] Merging formats into \"path\"" line (stderr).
var mergerOutputRegex = regexp.MustCompile(`\[Merger\] Merging formats into "(.+?)"`)

func downloadVideo(videoID string) (string, error) {
	dir := getSongRoot()
	cmd := NewDLCommandFromArgs(DefaultYtDlpBinary, []string{
		"--no-call-home", "--no-cache-dir", "--restrict-filenames",
		"--audio-quality", "0",
		"-o", filepath.Join(dir, "%(title)s-%(id)s.%(ext)s"),
	})
	out, err := cmd.ExecCombined(videoID)
	if err != nil {
		return "", err
	}

	matches := mergerOutputRegex.FindStringSubmatch(string(out))
	if len(matches) > 1 && matches[1] != "" {
		return matches[1], nil
	}
	return "", fmt.Errorf("could not parse output path from yt-dlp")
}
