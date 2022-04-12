package player

import (
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/jaredwarren/rpi_music/model"
)

var (
	p *Player
)

type Player struct {
	playing     bool
	currentSong *model.Song
	cmd         *exec.Cmd
}

func getPlayer() *Player {
	if p == nil {
		p = &Player{}
	}
	return p
}

func Play(song *model.Song) error {
	Stop()

	if song.FilePath == "" {
		return fmt.Errorf("invalid file:%+v", song)
	}

	cp := getPlayer()
	cmd := exec.Command("ffplay", "-nodisp", song.FilePath)
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
	cp.currentSong = song

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
	cp.currentSong = nil
}

func GetPlaying() *model.Song {
	cp := getPlayer()
	return cp.currentSong
}

func Beep() {
	// {
	// 	cmd := exec.Command("tput", "bel")
	// 	err := cmd.Run()
	// 	if err != nil {
	// 		fmt.Println("EEE:", err)
	// 	}
	// }
	// time.Sleep(1 * time.Second)

	// {
	// 	cmd := exec.Command("tput", "bel")
	// 	err := cmd.Run()
	// 	if err != nil {
	// 		fmt.Println("EEE:", err)
	// 	}
	// }
	// time.Sleep(1 * time.Second)

	// {
	// 	cmd := exec.Command("tput", "bel")
	// 	err := cmd.Run()
	// 	if err != nil {
	// 		fmt.Println("EEE:", err)
	// 	}
	// }
	// time.Sleep(1 * time.Second)

	{
		cmd := exec.Command("tput", "bel")
		err := cmd.Run()
		if err != nil {
			fmt.Println("EEE:", err)
		}
	}
	time.Sleep(1 * time.Second)
	runCmd("speaker-test -t sine -f 1000 -l 1")

}

func runCmd(cmds string) {
	command := strings.Split(cmds, " ")
	if len(command) < 2 {
		// TODO: handle error
	}
	cmd := exec.Command(command[0], command[1:]...)
	cmd.Start()
	time.Sleep(200 * time.Millisecond)
	cmd.Process.Kill()

	// stdoutStderr, err := cmd.CombinedOutput()
	// if err != nil {
	// 	// TODO: handle error more gracefully
	// 	fmt.Println("EEE:", err)
	// }
	// do something with output
	// fmt.Printf(":::::`%s`\n%s\n", cmds, stdoutStderr)
}
