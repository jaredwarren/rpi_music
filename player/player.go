package player

import (
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/jaredwarren/rpi_music/model"
	"github.com/spf13/viper"
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
	// TODO: add config check
	Stop()

	// TODO: con't overrite if already playing

	if song.FilePath == "" {
		return fmt.Errorf("invalid file:%+v", song)
	}

	// TODO: ffplay config
	// -loop number ;0 inf
	// -volume 0-100

	args := []string{
		"-nodisp",
	}

	if viper.GetBool("loop") {
		args = append(args, "-loop", "0")
	}

	args = append(args, "-volume", fmt.Sprintf("%d", viper.GetInt("volume")))

	args = append(args, song.FilePath)

	cp := getPlayer()
	cmd := exec.Command("ffplay", args...)
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
	if !viper.GetBool("beep") {
		return
	}
	cmds := "speaker-test -t sine -f 1000 -l 1"
	command := strings.Split(cmds, " ")
	cmd := exec.Command(command[0], command[1:]...)
	err := cmd.Start()
	if err != nil {
		fmt.Println("Beep Error:", err)
	}
	time.Sleep(200 * time.Millisecond)
	if cmd != nil && cmd.Process != nil {
		err := cmd.Process.Kill()
		if err != nil {
			fmt.Println("Beep kill Error:", err)
		}
	}
}

func runCmd(cmds string) ([]byte, error) {
	command := strings.Split(cmds, " ")
	if len(command) < 2 {
		return nil, fmt.Errorf("command too short:%s", cmds)
	}
	cmd := exec.Command(command[0], command[1:]...)
	cmd.Start()
	time.Sleep(200 * time.Millisecond)
	cmd.Process.Kill()

	return cmd.CombinedOutput()
}
