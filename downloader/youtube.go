package downloader

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/jaredwarren/rpi_music/log"
	"github.com/kkdai/youtube/v2"
	"github.com/spf13/viper"
)

const httpClientTimeout = 60 * time.Second

// Downloader is the interface for downloading YouTube audio and thumbnails.
type Downloader interface {
	GetVideo(videoID string) (*youtube.Video, error)
	DownloadVideo(ctx context.Context, videoID string, logger log.Logger) (string, *youtube.Video, error)
	DownloadThumb(video *youtube.Video) (string, error)
	GetVideoFilename(ctx context.Context, videoID string, logger log.Logger) (string, error)
}

type YoutubeDownloader struct{}

func (d *YoutubeDownloader) GetVideo(videoID string) (*youtube.Video, error) {
	client := youtube.Client{
		Debug: true,
	}
	return client.GetVideo(videoID)
}

func (d *YoutubeDownloader) DownloadVideo(ctx context.Context, videoID string, logger log.Logger) (string, *youtube.Video, error) {
	client := youtube.Client{}
	video, err := client.GetVideo(videoID)
	if err != nil {
		return "", video, err
	}

	formats := video.Formats.WithAudioChannels()
	if len(formats) == 0 {
		return "", video, ErrNoAudioFormats
	}
	sort.Slice(formats, func(i, j int) bool {
		return formats[i].AverageBitrate > formats[j].AverageBitrate
	})
	bestFormat := formats[0]
	ext := getExt(bestFormat.MimeType)
	sEnc := base64.StdEncoding.EncodeToString([]byte(video.Title))
	fileName := filepath.Join(viper.GetString("player.song_root"), fmt.Sprintf("%s%s", sEnc, ext))

	logger.Info("downloading video", log.Any("title", video.Title), log.Any("file", fileName))

	if _, err := os.Stat(fileName); err != nil && errors.Is(err, os.ErrNotExist) {
		stream, _, err := client.GetStream(video, &bestFormat)
		if err != nil {
			logger.Error("GetStream error", log.Any("title", video.Title), log.Error(err), log.Any("format", bestFormat))
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

func (d *YoutubeDownloader) GetVideoFilename(ctx context.Context, _ string, _ log.Logger) (string, error) {
	// Path is determined at download time from video title; no pre-check available.
	return "", nil
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
	ctx, cancel := context.WithTimeout(context.Background(), httpClientTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, URL, nil)
	if err != nil {
		return err
	}
	client := &http.Client{Timeout: httpClientTimeout}
	response, err := client.Do(req)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %d", response.StatusCode)
	}
	file, err := os.Create(fileName)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = io.Copy(file, response.Body)
	return err
}
