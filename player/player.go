package player

import (
	"fmt"
	"os"
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

func InitPlayer() {
	if _, err := os.Stat("./song_files"); os.IsNotExist(err) {
		err := os.Mkdir("./song_files", os.ModeDir)
		if err != nil {
			panic(err.Error())
		}
	}

	if _, err := os.Stat("./thumb_files"); os.IsNotExist(err) {
		err := os.Mkdir("./thumb_files", os.ModeDir)
		if err != nil {
			panic(err.Error())
		}
	}

	// TODO: check if ffplay is setup
}

type Player struct {
	Playing     bool
	currentSong *model.Song
	cmd         *exec.Cmd
	mu          sync.Mutex
}

func GetPlayer() *Player {
	if p == nil {
		p = &Player{}
	}
	return p
}

func Play(song *model.Song) error {
	if song.FilePath == "" {
		return fmt.Errorf("invalid file:%+v", song)
	}

	cp := GetPlayer()
	if cp.Playing {
		restart := viper.GetBool("restart")
		if cp.currentSong.FilePath == song.FilePath && !restart {
			fmt.Println("song already playing:", song)
			return nil
		}
		if !viper.GetBool("allow_override") {
			fmt.Println("song already playing something else:", song)
			return nil
		}
		Stop()
	}

	args := []string{
		"-nodisp",
		"-autoexit", // exit after song finishes, otherwise command won't stop
	}
	if viper.GetBool("player.loop") {
		// args = append(args, "-loop", "0")
		// TODO: fix this so if config changes loop stops
		// alos need override for things like startup sound
	}
	args = append(args, "-volume", fmt.Sprintf("%d", viper.GetInt("player.volume")))
	args = append(args, song.FilePath)

	cp.mu.Lock()
	defer cp.mu.Unlock()

	fmt.Printf("Playing: ffplay %+v\n", args)
	cmd := exec.Command("ffplay", args...)
	cp.cmd = cmd
	err := cmd.Start()
	if err != nil {
		return fmt.Errorf("[ERROR] start error: %w", err)
	}
	go func() {
		cp.mu.Lock()
		defer cp.mu.Unlock()
		cmd.Wait()
		cp.Playing = false
	}()

	cp.Playing = true
	cp.currentSong = song

	return nil
}

func Stop() {
	cp := GetPlayer()
	if cp.cmd != nil {
		if cp.cmd.Process != nil {
			cp.cmd.Process.Kill()
		}
	}
}

func GetPlaying() *model.Song {
	cp := GetPlayer()
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
