// Package downloader provides backends for downloading YouTube audio and thumbnails.
//
// Two implementations are available:
//   - YoutubeDownloader: uses the go-youtube library (no external binary).
//   - YoutubeDLDownloader: uses the yt-dlp CLI for downloading; supports more sites and formats.
//
// The implementation is chosen at server startup via config (e.g. downloader: "ytdl" for yt-dlp).
// All implementations satisfy the Downloader interface.
package downloader
