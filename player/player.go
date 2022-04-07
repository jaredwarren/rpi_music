package player

import (
	"fmt"
	"os/exec"
)

var (
	p *Player
)

type Player struct {
	playing bool
	cmd     *exec.Cmd
}

func getPlayer() *Player {
	if p == nil {
		p = &Player{}
	}
	return p
}

func Play(file string) error {
	Stop()

	cp := getPlayer()
	cmd := exec.Command("ffplay", "-nodisp", file)
	cp.cmd = cmd
	err := cmd.Start()
	if err != nil {
		return fmt.Errorf("[ERROR] start error: %w", err)
	}
	go func() {
		err := cmd.Wait()
		if err != nil {
			fmt.Println("[ERROR] play error: %w", err)
		}
	}()
	cp.playing = true

	return nil
}

func Stop() {
	cp := getPlayer()

	if cp.cmd != nil {
		err := cp.cmd.Process.Kill()
		if err != nil {
			fmt.Println("[ERROR] kill error: %w", err)
		}
	}
	cp.playing = false
}
