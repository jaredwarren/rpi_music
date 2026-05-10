package downloader

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jaredwarren/rpi_music/log"
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

	thumbDir := t.TempDir()
	v := NewYoutubeDLDownloader(&YoutubeDLConfig{ThumbRoot: thumbDir})
	thumb, err := v.DownloadThumb(&youtube.Video{
		ID: "https://youtu.be/ZJocdnMvTYs",
	})

	require.NoError(t, err)
	require.True(t, strings.HasPrefix(thumb, thumbDir), "thumb path should be under temp dir")
	require.Contains(t, thumb, "Could_Have_Been_Me")
	require.Contains(t, thumb, ".webp")
}

func TestDownloadVideo(t *testing.T) {
	skipIfNoYtDlp(t)

	songDir := t.TempDir()
	v := NewYoutubeDLDownloader(&YoutubeDLConfig{SongRoot: songDir})
	ctx := context.Background()

	f, vv, err := v.DownloadVideo(ctx, "https://youtu.be/ZJocdnMvTYs", log.NewNoOpLogger())

	require.NoError(t, err)
	require.NotNil(t, vv, "DownloadVideo returned nil video without error")
	require.True(t, strings.HasPrefix(f, songDir), "path should be under temp dir")
	require.Contains(t, filepath.Base(f), "Could_Have_Been_Me")
	require.Equal(t, "Could Have Been Me", vv.Title)
}
