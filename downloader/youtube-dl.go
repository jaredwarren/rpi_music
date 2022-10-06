package downloader

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/jaredwarren/rpi_music/log"
	"github.com/kkdai/youtube/v2"
	"golang.org/x/sync/errgroup"
)

var (
	logger = log.NewStdLogger(log.Debug)
)

type YoutubeDLDownloader struct{}

func (d *YoutubeDLDownloader) GetVideo(videoID string) (*youtube.Video, error) {
	return &youtube.Video{
		ID: videoID,
	}, nil
}

func (d *YoutubeDLDownloader) DownloadVideo(videoID string, logger log.Logger) (string, *youtube.Video, error) {
	var filename string
	resp := &youtube.Video{
		ID: videoID,
	}

	g := new(errgroup.Group)

	// get title
	g.Go(func() error {
		info, err := getVideoInfo(videoID)
		if err == nil {
			resp.Title = info["title"].(string)
		}
		return err
	})

	// get filename
	g.Go(func() error {
		var err error
		filename, err = getVideoFilename(videoID)
		return err
	})

	// download video
	g.Go(func() error {
		return downloadVideo(videoID)
	})

	if err := g.Wait(); err != nil {
		logger.Error("error downloading video", log.Error(err), log.Any("id", videoID))
		return "", nil, err
	}

	// validate that file exists
	if filename == "" {
		return "", nil, fmt.Errorf("could not get filename")
	}
	if _, err := os.Stat(filename); err != nil {
		return "", nil, err
	}

	return filename, resp, nil

}

func getVideoInfo(videoID string) (map[string]interface{}, error) {
	args := []string{
		"--ignore-errors",
		"--no-call-home",
		"--no-cache-dir",
		"--skip-download",
		"--restrict-filenames",
		"-J",
	}
	args = append(args, videoID)
	cmd := exec.Command("youtube-dl", args...)
	std, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	out := map[string]interface{}{}
	json.Unmarshal(std, &out)
	return out, nil
}

func getVideoFilename(videoID string) (string, error) {
	args := []string{
		"--ignore-errors",
		"--no-call-home",
		"--no-cache-dir",
		"--skip-download",
		"--restrict-filenames",
		"-f", "bestaudio",
		"--get-filename",
		"-o", `song_files/%(title)s-%(id)s.%(ext)s`,
	}
	args = append(args, videoID)
	cmd := exec.Command("youtube-dl", args...)
	std, err := cmd.Output()

	// clean output
	outStr := string(std)
	outStr = strings.Trim(outStr, `"`)
	outStr = strings.TrimSpace(outStr)
	return outStr, err
}

func downloadVideo(videoID string) error {
	args := []string{
		"--ignore-errors",
		"--no-call-home",
		"--no-cache-dir",
		"--restrict-filenames",
		"-f", "bestaudio",
		"-o", `song_files/%(title)s-%(id)s.%(ext)s`,
	}
	args = append(args, videoID)
	cmd := exec.Command("youtube-dl", args...)
	_, err := cmd.Output()
	return err
}

func (d *YoutubeDLDownloader) DownloadThumb(video *youtube.Video) (string, error) {
	// download video
	filename, err := downloadVideoThumb(video.ID)
	if err != nil {
		return "", err
	}

	// validate that file exists
	if filename == "" {
		return "", fmt.Errorf("could not get thumb filename")
	}
	if _, err := os.Stat(filename); err != nil {
		return "", err
	}

	return filename, nil
}

func downloadVideoThumb(videoID string) (string, error) {
	args := []string{
		"--write-thumbnail",
		"--ignore-errors",
		"--no-call-home",
		"--no-cache-dir",
		"--skip-download",
		"--restrict-filenames",
		"-o", `thumb_files/%(title)s-%(id)s`,
	}
	args = append(args, videoID)
	cmd := exec.Command("youtube-dl", args...)
	std, err := cmd.Output()

	// parse output because I can't find a better way to get thumb name
	outStr := string(std)
	var thumbRegex = regexp.MustCompile(`Writing thumbnail to: (.+?)\n`)
	result := thumbRegex.FindStringSubmatch(outStr)
	if len(result) == 0 {
		return "", fmt.Errorf("invalid results from download:%s", outStr)
	}
	return result[1], err
}
