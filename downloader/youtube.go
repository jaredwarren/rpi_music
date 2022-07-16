package downloader

import (
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/jaredwarren/rpi_music/log"
	"github.com/kkdai/youtube/v2"
	"github.com/spf13/viper"
)

type Downloader interface {
	DownloadVideo(videoID string, logger log.Logger) (string, *youtube.Video, error)
	DownloadThumb(video *youtube.Video) (string, error)
}

type YoutubeDownloader struct{}

func (d *YoutubeDownloader) DownloadVideo(videoID string, logger log.Logger) (string, *youtube.Video, error) {
	client := youtube.Client{}
	video, err := client.GetVideo(videoID)
	if err != nil {
		return "", video, err
	}

	formats := video.Formats.WithAudioChannels() // only get videos with audio
	formats.Sort()                               // I think this sorts best > worst

	bestFormat := formats[0]
	ext := getExt(bestFormat.MimeType)
	sEnc := base64.StdEncoding.EncodeToString([]byte(video.Title))
	fileName := filepath.Join(viper.GetString("player.song_root"), fmt.Sprintf("%s%s", sEnc, ext))

	logger.Info("downloading video", log.Any("title", video.Title), log.Any("file", fileName))

	if _, err := os.Stat(fileName); errors.Is(err, os.ErrNotExist) {
		stream, _, err := client.GetStream(video, &bestFormat)
		if err != nil {
			return fileName, video, err
		}

		file, err := os.Create(fileName)
		if err != nil {
			return fileName, video, err
		}
		defer file.Close()

		_, err = io.Copy(file, stream)
		if err != nil {
			return fileName, video, err
		}
	} else if err != nil {
		return fileName, video, err
	} else {
		logger.Warn("file already exists", log.Any("title", video.Title), log.Any("file", fileName))
	}
	return fileName, video, nil
}

func getExt(mimeType string) string {
	ls := strings.ToLower(mimeType)
	if strings.Contains(ls, "video/mp4") {
		return ".mp4"
	}
	if strings.Contains(ls, "video/webm") {
		return ".webm"
	}
	return "" //
}

func (d *YoutubeDownloader) DownloadThumb(video *youtube.Video) (string, error) {
	if len(video.Thumbnails) == 0 {
		return "", fmt.Errorf("no thumbs for video")
	}

	// find biggest
	thumb := video.Thumbnails[0]
	for _, t := range video.Thumbnails {
		if t.Width > thumb.Width {
			thumb = t
		}
	}

	fileURL := thumb.URL

	// clean up `.../hqdefault.jpg?sqp=-oaymwEj...`
	ext := filepath.Ext(fileURL)
	ext = strings.Split(ext, "?")[0]
	sEnc := base64.StdEncoding.EncodeToString([]byte(video.Title))
	fileName := filepath.Join(viper.GetString("player.thumb_root"), fmt.Sprintf("%s%s", sEnc, ext))

	err := downloadFile(fileURL, fileName)
	if err != nil {
		return "", err
	}

	return fileName, nil
}

func downloadFile(URL, fileName string) error {
	//Get the response bytes from the url
	response, err := http.Get(URL)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if response.StatusCode != 200 {
		return errors.New("Received non 200 response code")
	}
	//Create a empty file
	file, err := os.Create(fileName)
	if err != nil {
		return err
	}
	defer file.Close()

	//Write the bytes to the fiel
	_, err = io.Copy(file, response.Body)
	if err != nil {
		return err
	}

	return nil
}
