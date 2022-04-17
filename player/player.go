package player

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/jaredwarren/rpi_music/log"
	"github.com/jaredwarren/rpi_music/model"
	"github.com/spf13/viper"
)

var (
	p      *Player
	logger log.Logger
)

func InitPlayer(l log.Logger) {
	logger = l
	if _, err := os.Stat(viper.GetString("player.song_root")); os.IsNotExist(err) {
		err := os.Mkdir(viper.GetString("player.song_root"), os.ModeDir)
		if err != nil {
			logger.Panic("error creating song_file dir", log.Error(err))
		}
	}

	if _, err := os.Stat(viper.GetString("player.thumb_root")); os.IsNotExist(err) {
		err := os.Mkdir(viper.GetString("player.thumb_root"), os.ModeDir)
		if err != nil {
			logger.Panic("error creating thumb_file dir", log.Error(err))
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
			logger.Info("selected song already playing", log.Any("song", song))
			return nil
		}
		if !viper.GetBool("allow_override") {
			logger.Info("another song already playing", log.Any("song", song))
			// TODO: if queue enabled add sont to queue, and don't error
			Error()
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

	logger.Info("Play song", log.Any("song", song), log.Any("args", args))
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
	fmt.Println("~~~~~~~~~ BEEP!!!")
	if !viper.GetBool("beep") {
		fmt.Println("~~~~~~~~~ NO BEEP!!!")
		return
	}
	args := []string{
		"-nodisp",
		"-autoexit", // exit after song finishes, otherwise command won't stop
	}
	args = append(args, "-volume", fmt.Sprintf("%d", viper.GetInt("player.volume")))
	args = append(args, "sounds/success.wav")
	fmt.Println(args)
	logger.Error("ffplay", log.Any("args", args))
	cmd := exec.Command("ffplay", args...)
	cmd.Run()
}

func Error() {
	if !viper.GetBool("beep") {
		return
	}
	args := []string{
		"-nodisp",
		"-autoexit", // exit after song finishes, otherwise command won't stop
	}
	args = append(args, "-volume", fmt.Sprintf("%d", viper.GetInt("player.volume")))
	args = append(args, "sounds/error.wav")
	cmd := exec.Command("ffplay", args...)
	cmd.Run()
}

// runCmd, used to debug commands
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
