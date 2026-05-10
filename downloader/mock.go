package downloader

import (
	"context"

	"github.com/kkdai/youtube/v2"
	"github.com/rs/zerolog"
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

func (d *MockDownloader) DownloadVideo(ctx context.Context, videoID string, _ zerolog.Logger) (string, *youtube.Video, error) {
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

func (d *MockDownloader) GetVideoFilename(ctx context.Context, _ string, _ zerolog.Logger) (string, error) {
	return "", nil
}
