package downloader

import (
	"testing"

	"github.com/kkdai/youtube/v2"
	"github.com/stretchr/testify/assert"
)

func TestDownloadThumb(t *testing.T) {
	v := YoutubeDLDownloader{}
	thumb, err := v.DownloadThumb(&youtube.Video{
		ID: "https://youtu.be/ZJocdnMvTYs",
	})

	assert.NoError(t, err)
	assert.Equal(t, "aHR0cHM6Ly95b3V0dS5iZS9aSm9jZG5NdlRZcw==.webp", thumb)
	// TODO: clean up files
}

func TestDownloadVideo(t *testing.T) {
	v := YoutubeDLDownloader{}
	f, vv, err := v.DownloadVideo("https://youtu.be/ZJocdnMvTYs", nil)

	assert.NoError(t, err)
	assert.Equal(t, "aHR0cHM6Ly95b3V0dS5iZS9aSm9jZG5NdlRZcw==.mp4", f)
	assert.Equal(t, "Could Have Been Me", vv.Title)
	// TODO: clean up files
}
