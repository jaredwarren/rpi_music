package localtunnel

import (
	"bufio"
	"fmt"
	"net/url"
	"os/exec"
	"strings"
	"sync"

	"github.com/jaredwarren/rpi_music/log"
	"github.com/spf13/viper"
)

type lt struct {
	cmd    *exec.Cmd
	mu     sync.Mutex
	logger log.Logger
}

var (
	cmd *exec.Cmd
)

func Init(logger log.Logger) error {
	logger.Info("[localtunnel] localtunnel")
	// TODO: check if localtunnel is installed!!!
	ltHost := viper.GetString("localtunnel.host") // LOCALTUNNEL_HOST
	if ltHost == "" {
		return fmt.Errorf("localtunnel host required")
	}

	h := viper.GetString("host")
	if h == "" {
		return fmt.Errorf("local host required")
	}

	var port string
	var err error
	if strings.HasPrefix(h, ":") {
		port = strings.TrimLeft(h, ":")
	} else {
		u, err := url.Parse(h)
		if err != nil {
			return err
		}
		port = u.Port()
	}

	if port == "" {
		// assume 80 or 443 based on https in config???
		return fmt.Errorf("local port required")
	}

	logger.Info("[localtunnel] starting localtunnel", log.Any("subdomain", ltHost), log.Any("local_host", h), log.Any("port", port))
	args := []string{
		"--port", port,
		"--subdomain", ltHost,
	}
	cmd = exec.Command("lt", args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("[ERROR] StdoutPipe error: %w", err)
	}
	err = cmd.Start()
	if err != nil {
		return fmt.Errorf("[ERROR] start error: %w", err)
	}
	go func() {
		in := bufio.NewScanner(stdout)

		for in.Scan() {
			logger.Info(in.Text())
		}
		if err := in.Err(); err != nil {
			logger.Error(err.Error())
		}
	}()
	go func() {
		cmd.Wait()
	}()

	return nil
}

func Close() error {
	return cmd.Process.Kill()
}
