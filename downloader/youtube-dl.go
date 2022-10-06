package downloader

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"

	"github.com/jaredwarren/rpi_music/log"
	"github.com/kkdai/youtube/v2"
	"golang.org/x/sync/errgroup"
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

	// args := []string{
	// 	"--ignore-errors",
	// 	"--no-call-home",
	// 	"--no-cache-dir",
	// 	"--skip-download",
	// 	"--restrict-filenames",
	// 	"-J",
	// 	"-f",
	// 	"best",
	// }
	// args = append(args, videoID)
	// cmd := exec.Command("youtube-dl", args...)
	// std, err := cmd.Output()
	// if err != nil {
	// 	return "", nil, err
	// }
	// rawOutput := string(std)

	// var thumbRegex = regexp.MustCompile(`Destination: (.+?)\n`)
	// result := thumbRegex.FindStringSubmatch(rawOutput)
	// if len(result) == 0 {
	// 	return "", nil, fmt.Errorf("invalid results from download:%s", rawOutput)
	// }
	// oldLocation := result[1]

	// ext := filepath.Ext(oldLocation)
	// sEnc := base64.StdEncoding.EncodeToString([]byte(videoID))
	// fileName := filepath.Join(viper.GetString("player.thumb_root"), fmt.Sprintf("%s%s", sEnc, ext))

	// err = os.Rename(oldLocation, fileName)
	// if err != nil {
	// 	return "", nil, err
	// }

	// title := oldLocation
	// {
	// 	args := []string{
	// 		"-e",
	// 	}
	// 	args = append(args, videoID)
	// 	cmd := exec.Command("youtube-dl", args...)
	// 	std, _ := cmd.Output()
	// 	rawOutput := strings.TrimSpace(string(std))
	// 	if rawOutput != "" {
	// 		title = rawOutput
	// 	}
	// }

	// return fileName, &youtube.Video{
	// 	ID:    videoID,
	// 	Title: title,
	// }, nil
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
		"-o", `"./song_files/%(title)s-%(id)s.%(ext)s"`,
	}
	args = append(args, videoID)
	cmd := exec.Command("youtube-dl", args...)
	std, err := cmd.Output()
	return string(std), err
}

func downloadVideo(videoID string) error {
	args := []string{
		"--ignore-errors",
		"--no-call-home",
		"--no-cache-dir",
		"--restrict-filenames",
		"-f", "bestaudio",
		"-o", `"./song_files/%(title)s-%(id)s.%(ext)s"`,
	}
	args = append(args, videoID)
	cmd := exec.Command("youtube-dl", args...)
	_, err := cmd.Output()
	return err
}

func (d *YoutubeDLDownloader) DownloadThumb(video *youtube.Video) (string, error) {

	var filename string

	g := new(errgroup.Group)

	// get filename
	g.Go(func() error {
		var err error
		filename, err = getVideoThumb(video.ID)
		return err
	})

	// download video
	g.Go(func() error {
		return downloadVideoThumb(video.ID)
	})

	if err := g.Wait(); err != nil {
		// logger.Error("error downloading video", log.Error(err), log.Any("id", video.ID))
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

	// args := []string{
	// 	"--write-thumbnail",
	// 	"--skip-download",
	// }
	// args = append(args, video.ID)
	// cmd := exec.Command("youtube-dl", args...)
	// std, err := cmd.Output()
	// if err != nil {
	// 	return "", err
	// }
	// rawOutput := string(std)

	// var thumbRegex = regexp.MustCompile(`Writing thumbnail to: (.+?)\n`)
	// result := thumbRegex.FindStringSubmatch(rawOutput)
	// if len(result) == 0 {
	// 	return "", fmt.Errorf("invalid results from download:%s", rawOutput)
	// }
	// oldLocation := result[1]

	// ext := filepath.Ext(oldLocation)
	// sEnc := base64.StdEncoding.EncodeToString([]byte(video.ID))
	// fileName := filepath.Join(viper.GetString("player.thumb_root"), fmt.Sprintf("%s%s", sEnc, ext))

	// err = os.Rename(oldLocation, fileName)
	// if err != nil {
	// 	return "", err
	// }

	// return fileName, nil
}

func getVideoThumb(videoID string) (string, error) {
	args := []string{
		"--write-thumbnail",
		"--ignore-errors",
		"--no-call-home",
		"--no-cache-dir",
		"--restrict-filenames",
		"--get-filename",
		"-o", `"./thumb_files/%(title)s-%(id)s.%(ext)s"`,
	}
	args = append(args, videoID)
	cmd := exec.Command("youtube-dl", args...)
	std, err := cmd.Output()
	return string(std), err
}

func downloadVideoThumb(videoID string) error {
	args := []string{
		"--write-thumbnail",
		"--ignore-errors",
		"--no-call-home",
		"--no-cache-dir",
		"--restrict-filenames",
		"-o", `"./thumb_files/%(title)s-%(id)s.%(ext)s"`,
	}
	args = append(args, videoID)
	cmd := exec.Command("youtube-dl", args...)
	_, err := cmd.Output()
	return err
}
