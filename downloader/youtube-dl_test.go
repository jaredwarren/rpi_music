package downloader

import (
	"os"
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
	assert.Equal(t, "thumb_files/Could_Have_Been_Me-ZJocdnMvTYs.webp", thumb)
	_ = os.RemoveAll("./thumb_files")
}

func TestDownloadVideo(t *testing.T) {
	v := YoutubeDLDownloader{}
	f, vv, err := v.DownloadVideo("https://youtu.be/ZJocdnMvTYs", nil)

	assert.NoError(t, err)
	assert.Equal(t, "song_files/Could_Have_Been_Me-ZJocdnMvTYs.webm", f)
	assert.Equal(t, "Could Have Been Me", vv.Title)
	// TODO: clean up files
}
