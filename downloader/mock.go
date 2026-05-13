package downloader

import (
	"context"
	"log/slog"

	"github.com/kkdai/youtube/v2"
)

type MockDownloader struct {
	Response map[string]*youtube.Video
}

func (d *MockDownloader) GetVideo(videoID string) (*youtube.Video, error) {
	v, ok := d.Response[videoID]
	if !ok {
		return nil, ErrNotFound
	}
	return v, nil
}

func (d *MockDownloader) DownloadVideo(ctx context.Context, videoID string, _ *slog.Logger) (string, *youtube.Video, error) {
	v, ok := d.Response[videoID]
	if !ok {
		return "", nil, ErrNotFound
	}
	return v.Title, v, nil
}

func (d *MockDownloader) DownloadThumb(video *youtube.Video) (string, error) {
	if len(video.Thumbnails) == 0 {
		return "", ErrNotFound
	}
	return video.Thumbnails[0].URL, nil
}

func (d *MockDownloader) GetVideoFilename(ctx context.Context, _ string, _ *slog.Logger) (string, error) {
	return "", nil
}
