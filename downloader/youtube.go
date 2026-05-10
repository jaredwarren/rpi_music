package downloader

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/kkdai/youtube/v2"
)

const httpClientTimeout = 60 * time.Second

// Downloader is the interface for downloading YouTube audio and thumbnails.
type Downloader interface {
	GetVideo(videoID string) (*youtube.Video, error)
	DownloadVideo(ctx context.Context, videoID string, logger *slog.Logger) (string, *youtube.Video, error)
	DownloadThumb(video *youtube.Video) (string, error)
	GetVideoFilename(ctx context.Context, videoID string, logger *slog.Logger) (string, error)
}

// YoutubeDownloader downloads audio using the kkdai/youtube library (no external binary).
type YoutubeDownloader struct {
	SongRoot  string // directory for downloaded audio files
	ThumbRoot string // directory for downloaded thumbnails
}

func (d *YoutubeDownloader) songRoot() string {
	if d.SongRoot != "" {
		return d.SongRoot
	}
	return defaultSongDir
}

func (d *YoutubeDownloader) thumbRoot() string {
	if d.ThumbRoot != "" {
		return d.ThumbRoot
	}
	return defaultThumbDir
}

func (d *YoutubeDownloader) GetVideo(videoID string) (*youtube.Video, error) {
	client := youtube.Client{Debug: true}
	return client.GetVideo(videoID)
}

func (d *YoutubeDownloader) DownloadVideo(ctx context.Context, videoID string, logger *slog.Logger) (string, *youtube.Video, error) {
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
	fileName := filepath.Join(d.songRoot(), fmt.Sprintf("%s%s", sEnc, ext))

	logger.Info("downloading video", "title", video.Title, "file", fileName)

	if _, err := os.Stat(fileName); err != nil && errors.Is(err, os.ErrNotExist) {
		stream, _, err := client.GetStream(video, &bestFormat)
		if err != nil {
			return fileName, video, err
		}

		file, err := os.Create(fileName)
		if err != nil {
			return fileName, video, err
		}
		defer file.Close()

		if _, err = io.Copy(file, stream); err != nil {
			return fileName, video, err
		}
	} else if err != nil {
		return fileName, video, err
	}
	return fileName, video, nil
}

func (d *YoutubeDownloader) GetVideoFilename(_ context.Context, _ string, _ *slog.Logger) (string, error) {
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
	return ""
}

func (d *YoutubeDownloader) DownloadThumb(video *youtube.Video) (string, error) {
	if len(video.Thumbnails) == 0 {
		return "", fmt.Errorf("no thumbs for video")
	}

	thumb := video.Thumbnails[0]
	for _, t := range video.Thumbnails {
		if t.Width > thumb.Width {
			thumb = t
		}
	}

	fileURL := thumb.URL
	ext := strings.Split(filepath.Ext(fileURL), "?")[0]
	sEnc := base64.StdEncoding.EncodeToString([]byte(video.Title))
	fileName := filepath.Join(d.thumbRoot(), fmt.Sprintf("%s%s", sEnc, ext))

	return fileName, downloadFile(fileURL, fileName)
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
