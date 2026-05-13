package downloader

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// DefaultYtDlpBinary is the name of the yt-dlp executable used by YoutubeDLDownloader.
const DefaultYtDlpBinary = "yt-dlp"

// NewDLCommand builds a command from a single string (space-separated).
// Arguments containing spaces are not supported; use NewDLCommandFromArgs for those.
func NewDLCommand(in string) *DLCommand {
	c := &DLCommand{
		FullCommand: in,
	}
	c.parse()
	return c
}

// NewDLCommandFromArgs builds a command from a binary name and a slice of arguments.
// Use this when arguments may contain spaces or when building args programmatically.
func NewDLCommandFromArgs(binary string, args []string) *DLCommand {
	argsCopy := make([]string, len(args))
	copy(argsCopy, args)
	return &DLCommand{
		BaseCommand: binary,
		Args:        argsCopy,
		parsed:      true,
	}
}

type DLCommand struct {
	FullCommand string   // full command as it appears in command line (for NewDLCommand)
	BaseCommand string   // executable name
	Args        []string // arguments (including -o paths)
	WorkingDir  string
	parsed      bool
}

func (d *DLCommand) GetCommand() (string, []string) {
	if !d.parsed {
		d.parse()
	}
	return d.BaseCommand, d.Args
}

func (d *DLCommand) parse() {
	c, cp := commandToParts(d.FullCommand)
	d.BaseCommand = c
	d.Args = cp
	d.parsed = true
}

// ExecCombined runs the command and returns combined stdout and stderr.
// Use when the subprocess writes progress or paths to stderr (e.g. yt-dlp merge message).
func (d *DLCommand) ExecCombined(exArgs ...string) ([]byte, error) {
	return d.ExecCombinedContext(context.Background(), exArgs...)
}

func (d *DLCommand) ExecCombinedContext(ctx context.Context, exArgs ...string) ([]byte, error) {
	if !d.parsed {
		d.parse()
	}
	c, baseArgs := d.GetCommand()
	args := make([]string, len(baseArgs), len(baseArgs)+len(exArgs))
	copy(args, baseArgs)
	args = append(args, exArgs...)
	cmd := exec.CommandContext(ctx, c, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("cmd err:%w", err)
	}
	return out, nil
}

func (d *DLCommand) Exec(exArgs ...string) (string, error) {
	std, err := d.ExecB(exArgs...)
	return string(std), err
}

func (d *DLCommand) ExecB(exArgs ...string) ([]byte, error) {
	return d.ExecBContext(context.Background(), exArgs...)
}

func (d *DLCommand) ExecBContext(ctx context.Context, exArgs ...string) ([]byte, error) {
	if !d.parsed {
		d.parse()
	}
	c, baseArgs := d.GetCommand()
	args := make([]string, len(baseArgs), len(baseArgs)+len(exArgs))
	copy(args, baseArgs)
	args = append(args, exArgs...)
	cmd := exec.CommandContext(ctx, c, args...)
	std, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("cmd err:%w", err)
	}
	return std, nil
}

func commandToParts(raw string) (string, []string) {
	parts := strings.Split(raw, " ")
	return parts[0], parts[1:]
}
