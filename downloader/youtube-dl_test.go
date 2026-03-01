package downloader

import (
	"os"
	"os/exec"
	"testing"

	"github.com/kkdai/youtube/v2"
	"github.com/stretchr/testify/require"
)

// skipIfNoYtDlp skips the test if the yt-dlp executable is not in PATH.
// These tests are integration tests that require yt-dlp to be installed.
func skipIfNoYtDlp(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath(DefaultYtDlpBinary); err != nil {
		t.Skipf("skipping: %s not found in PATH: %v", DefaultYtDlpBinary, err)
	}
}

func TestDownloadThumb(t *testing.T) {
	skipIfNoYtDlp(t)

	v := YoutubeDLDownloader{}
	thumb, err := v.DownloadThumb(&youtube.Video{
		ID: "https://youtu.be/ZJocdnMvTYs",
	})

	require.NoError(t, err)
	require.Equal(t, "thumb_files/Could_Have_Been_Me-ZJocdnMvTYs.webp", thumb)
	_ = os.RemoveAll("./thumb_files")
}

func TestDownloadVideo(t *testing.T) {
	skipIfNoYtDlp(t)

	v := YoutubeDLDownloader{}
	f, vv, err := v.DownloadVideo("https://youtu.be/ZJocdnMvTYs", nil)

	require.NoError(t, err)
	require.NotNil(t, vv, "DownloadVideo returned nil video without error")
	require.Equal(t, "song_files/Could_Have_Been_Me-ZJocdnMvTYs.webm", f)
	require.Equal(t, "Could Have Been Me", vv.Title)
}
