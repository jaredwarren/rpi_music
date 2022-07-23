package downloader

import (
	"fmt"

	"github.com/jaredwarren/rpi_music/log"
	"github.com/kkdai/youtube/v2"
)

type MockDownloader struct {
	Response map[string]*youtube.Video
}

func (d *MockDownloader) GetVideo(videoID string) (*youtube.Video, error) {
	v, ok := d.Response[videoID]
	if !ok {
		return nil, fmt.Errorf("not found")
	}
	return v, nil
}

func (d *MockDownloader) DownloadVideo(videoID string, logger log.Logger) (string, *youtube.Video, error) {
	v, ok := d.Response[videoID]
	if !ok {
		return "", nil, fmt.Errorf("not found")
	}
	return v.Title, v, nil
}

func (d *MockDownloader) DownloadThumb(video *youtube.Video) (string, error) {
	if len(video.Thumbnails) == 0 {
		return "", fmt.Errorf("thumb not found")
	}
	return video.Thumbnails[0].URL, nil
}
