package downloader

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
)

// mergerOutputRegex captures the output path from yt-dlp's "[Merger] Merging formats into \"path\"" line (stderr).
var mergerOutputRegex = regexp.MustCompile(`\[Merger\] Merging formats into "(.+?)"`)

func downloadVideo(ctx context.Context, videoID, songRoot, binary string) (string, error) {
	cmd := NewDLCommandFromArgs(binary, []string{
		"--no-call-home", "--no-cache-dir", "--restrict-filenames",
		"--audio-quality", "0",
		"-o", filepath.Join(songRoot, "%(title)s-%(id)s.%(ext)s"),
	})
	out, err := cmd.ExecCombinedContext(ctx, videoID)
	if err != nil {
		return "", err
	}

	matches := mergerOutputRegex.FindStringSubmatch(string(out))
	if len(matches) > 1 && matches[1] != "" {
		return matches[1], nil
	}
	return "", fmt.Errorf("could not parse output path from yt-dlp")
}
