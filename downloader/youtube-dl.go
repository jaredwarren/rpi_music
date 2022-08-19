package downloader

import (
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/jaredwarren/rpi_music/log"
	"github.com/kkdai/youtube/v2"
	"github.com/spf13/viper"
)

type YoutubeDLDownloader struct{}

func (d *YoutubeDLDownloader) GetVideo(videoID string) (*youtube.Video, error) {
	return &youtube.Video{
		ID: videoID,
	}, nil
}

func (d *YoutubeDLDownloader) DownloadVideo(videoID string, logger log.Logger) (string, *youtube.Video, error) {
	// TODO: set output in cmd!!!!!, but how do I get .ext?

	args := []string{
		"-f",
		"best",
	}
	args = append(args, videoID)
	cmd := exec.Command("youtube-dl", args...)
	std, err := cmd.Output()
	if err != nil {
		return "", nil, err
	}
	rawOutput := string(std)

	var thumbRegex = regexp.MustCompile(`Destination: (.+?)\n`)
	result := thumbRegex.FindStringSubmatch(rawOutput)
	if len(result) == 0 {
		return "", nil, fmt.Errorf("invalid results from download:%s", rawOutput)
	}
	oldLocation := result[1]

	ext := filepath.Ext(oldLocation)
	sEnc := base64.StdEncoding.EncodeToString([]byte(videoID))
	fileName := filepath.Join(viper.GetString("player.thumb_root"), fmt.Sprintf("%s%s", sEnc, ext))

	err = os.Rename(oldLocation, fileName)
	if err != nil {
		return "", nil, err
	}

	title := oldLocation
	{
		args := []string{
			"-e",
		}
		args = append(args, videoID)
		cmd := exec.Command("youtube-dl", args...)
		std, _ := cmd.Output()
		rawOutput := strings.TrimSpace(string(std))
		if rawOutput != "" {
			title = rawOutput
		}
	}

	return fileName, &youtube.Video{
		ID:    videoID,
		Title: title,
	}, nil
}

func (d *YoutubeDLDownloader) DownloadThumb(video *youtube.Video) (string, error) {
	args := []string{
		"--write-thumbnail",
		"--skip-download",
	}
	args = append(args, video.ID)
	cmd := exec.Command("youtube-dl", args...)
	std, err := cmd.Output()
	if err != nil {
		return "", err
	}
	rawOutput := string(std)

	var thumbRegex = regexp.MustCompile(`Writing thumbnail to: (.+?)\n`)
	result := thumbRegex.FindStringSubmatch(rawOutput)
	if len(result) == 0 {
		return "", fmt.Errorf("invalid results from download:%s", rawOutput)
	}
	oldLocation := result[1]

	ext := filepath.Ext(oldLocation)
	sEnc := base64.StdEncoding.EncodeToString([]byte(video.ID))
	fileName := filepath.Join(viper.GetString("player.thumb_root"), fmt.Sprintf("%s%s", sEnc, ext))

	err = os.Rename(oldLocation, fileName)
	if err != nil {
		return "", err
	}

	return fileName, nil
}
