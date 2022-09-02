package localtunnel

import (
	"fmt"
	"net/url"
	"os/exec"
	"strings"
	"sync"

	"github.com/spf13/viper"
)

type lt struct {
	cmd *exec.Cmd
	mu  sync.Mutex
}

var (
	cmd *exec.Cmd
)

func Init() error {
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
		return fmt.Errorf("local port required")
	}

	args := []string{
		"--port", port,
		"--subdomain", ltHost,
	}
	cmd = exec.Command("lt", args...)
	err = cmd.Start()
	if err != nil {
		return fmt.Errorf("[ERROR] start error: %w", err)
	}
	go func() {
		cmd.Wait()
	}()

	return nil
}

func Close() error {
	return cmd.Process.Kill()
}
