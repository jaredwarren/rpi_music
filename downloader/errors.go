package downloader

import "errors"

// Sentinel errors for caller branching. Use errors.Is to check.
var (
	ErrNotFound           = errors.New("downloader: not found")
	ErrAlreadyExists      = errors.New("downloader: file already exists")
	ErrExecutableNotFound = errors.New("downloader: yt-dlp executable not found in PATH")
	ErrNoAudioFormats     = errors.New("downloader: no audio formats found")
	ErrMissingURL         = errors.New("downloader: missing url")
)
