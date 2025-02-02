package downloader

import (
	"fmt"
	"os/exec"
	"strings"
)

func NewDLCommand(in string) *DLCommand {
	c := &DLCommand{
		FullCommand: in,
	}
	c.parse()
	return c
}

type DLCommand struct {
	FullCommand string // full command as it appears in command line
	BaseCommand string // main executable
	Args        []string
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

func (d *DLCommand) Exec(exArgs ...string) (string, error) {
	std, err := d.ExecB(exArgs...)
	return string(std), err
}

func (d *DLCommand) ExecB(exArgs ...string) ([]byte, error) {
	if !d.parsed {
		d.parse()
	}

	c, args := d.GetCommand()
	args = append(args, exArgs...)
	cmd := exec.Command(c, args...)
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
