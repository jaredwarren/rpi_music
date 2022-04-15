package player

import (
	"fmt"
	"os/exec"
	"strings"
	"sync"
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
	mu          sync.Mutex
}

func getPlayer() *Player {
	if p == nil {
		p = &Player{}
	}
	return p
}

func Play(song *model.Song) error {
	fmt.Printf(">>>>%+v\n", song)
	if song.FilePath == "" {
		fmt.Println("  no file path")
		return fmt.Errorf("invalid file:%+v", song)
	}

	cp := getPlayer()
	if cp.playing {
		fmt.Println("  something is playing")
		restart := viper.GetBool("restart")
		if cp.currentSong.FilePath == song.FilePath && !restart {
			fmt.Println("    no restart")
			fmt.Println("song already playing:", song)
			return nil
		}
		if !viper.GetBool("allow_override") {
			fmt.Println("    no override")
			fmt.Println("song already playing something else:", song)
			return nil
		}
		Stop("pp")
	}

	args := []string{
		"-nodisp",
		"-autoexit",
	}
	if viper.GetBool("loop") {
		args = append(args, "-loop", "0")
	}
	args = append(args, "-volume", fmt.Sprintf("%d", viper.GetInt("volume")))
	args = append(args, song.FilePath)

	cp.mu.Lock()
	defer cp.mu.Unlock()
	fmt.Printf("  args:%+v	\n", args)

	cmd := exec.Command("ffplay", args...)
	cp.cmd = cmd
	err := cmd.Start()
	if err != nil {
		fmt.Println("  start err:", err)
		return fmt.Errorf("[ERROR] start error: %w", err)
	}
	go func() {
		cp.mu.Lock()
		defer cp.mu.Unlock()
		fmt.Println("  waiting...")
		err := cmd.Wait()
		if err != nil {
			fmt.Println("  wait err:", err)
			fmt.Println("[ERROR] play wait error: %w", err)
		}
		cp.playing = false
		fmt.Println("  done...")
	}()

	fmt.Println("  playing...")
	cp.playing = true
	cp.currentSong = song

	return nil
}

func Stop(f string) {
	fmt.Println("xxx stopping...", f)
	cp := getPlayer()

	if cp.cmd != nil {
		if cp.cmd.Process != nil {
			cp.cmd.Process.Kill()
		}
	}
	fmt.Println("xxx stopped!")
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
