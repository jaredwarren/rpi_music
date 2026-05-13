package downloader

import (
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	defaultSongDir  = "song_files"
	defaultThumbDir = "thumb_files"

	dockerImage    = "jauderho/yt-dlp"
	containerMount = "/downloads"
)

// YoutubeDLConfig configures the yt-dlp CLI downloader. All fields are optional;
// empty values fall back to defaultSongDir / defaultThumbDir / DefaultYtDlpBinary.
type YoutubeDLConfig struct {
	SongRoot  string // output dir for audio
	ThumbRoot string // output dir for thumbnails
	Binary    string // yt-dlp executable name
	useDocker bool   // set by EnsureYtDlpAvailable when native binary is absent but Docker is present
}

func (c *YoutubeDLConfig) songRoot() string {
	if c != nil && c.SongRoot != "" {
		return c.SongRoot
	}
	return defaultSongDir
}

func (c *YoutubeDLConfig) thumbRoot() string {
	if c != nil && c.ThumbRoot != "" {
		return c.ThumbRoot
	}
	return defaultThumbDir
}

func (c *YoutubeDLConfig) binary() string {
	if c != nil && c.Binary != "" {
		return c.Binary
	}
	return DefaultYtDlpBinary
}

// BackendDescription returns a human-readable string describing which backend will be used.
func (c *YoutubeDLConfig) BackendDescription() string {
	if c != nil && c.useDocker {
		return "docker (" + dockerImage + ")"
	}
	return c.binary()
}

// newDownloadCmd builds a DLCommand for operations that write files to disk.
// absRoot is the absolute path of the output directory on the host. In Docker
// mode it is mounted as /downloads inside the container and the args are
// rewritten accordingly; returned paths must be un-translated with translatePath.
func (c *YoutubeDLConfig) newDownloadCmd(ytDlpArgs []string, absRoot string) *DLCommand {
	if c == nil || !c.useDocker {
		return NewDLCommandFromArgs(c.binary(), ytDlpArgs)
	}
	translated := translateArg(ytDlpArgs, absRoot, containerMount)
	args := append([]string{"run", "--rm", "-v", absRoot + ":" + containerMount, dockerImage}, translated...)
	return NewDLCommandFromArgs("docker", args)
}

// newMetaCmd builds a DLCommand for metadata-only operations (no file output).
// No volume mount is needed because nothing is written to disk.
func (c *YoutubeDLConfig) newMetaCmd(ytDlpArgs []string) *DLCommand {
	if c == nil || !c.useDocker {
		return NewDLCommandFromArgs(c.binary(), ytDlpArgs)
	}
	args := append([]string{"run", "--rm", dockerImage}, ytDlpArgs...)
	return NewDLCommandFromArgs("docker", args)
}

// translatePath maps a container-internal path (e.g. /downloads/foo.mp4) back to the
// equivalent host path. It is a no-op when native yt-dlp is in use.
func (c *YoutubeDLConfig) translatePath(path, absRoot string) string {
	if c == nil || !c.useDocker {
		return path
	}
	return strings.Replace(path, containerMount, absRoot, 1)
}

// EnsureYtDlpAvailable checks whether yt-dlp is available natively, then falls
// back to Docker. Sets cfg.useDocker when the Docker fallback will be used.
// Returns ErrExecutableNotFound only if neither is available.
func EnsureYtDlpAvailable(cfg *YoutubeDLConfig) error {
	if cfg == nil {
		cfg = &YoutubeDLConfig{}
	}
	if _, err := exec.LookPath(cfg.binary()); err == nil {
		return nil
	}
	if _, err := exec.LookPath("docker"); err == nil {
		cfg.useDocker = true
		return nil
	}
	return ErrExecutableNotFound
}

// translateArg replaces all occurrences of oldRoot with newRoot in each element of args.
func translateArg(args []string, oldRoot, newRoot string) []string {
	out := make([]string, len(args))
	for i, a := range args {
		out[i] = strings.ReplaceAll(a, oldRoot, newRoot)
	}
	return out
}

// absPath returns the absolute version of dir, falling back to dir unchanged on error.
func absPath(dir string) string {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return dir
	}
	return abs
}
