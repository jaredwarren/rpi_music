package downloader

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/jaredwarren/rpi_music/log"
	"github.com/kkdai/youtube/v2"
)

type YoutubeDLDownloader struct{}

func (d *YoutubeDLDownloader) GetVideo(videoID string) (*youtube.Video, error) {
	return &youtube.Video{
		ID: videoID,
	}, nil
}

func (d *YoutubeDLDownloader) DownloadVideo(videoID string, logger log.Logger) (string, *youtube.Video, error) {
	var filename string
	resp := &youtube.Video{
		ID: videoID,
	}

	// https://music.youtube.com/watch?v=4qwIhKfv_Dc&si=ST0cFoZIDwj5DKDI
	videoID = strings.Replace(videoID, "//music.", "//", 1)
	fmt.Printf("~~~~~~~~~~~~~~~\n clean url:%+v\n\n", videoID)

	var wg sync.WaitGroup

	// g := new(errgroup.Group)

	// get title
	wg.Add(1)
	go func() error {
		defer wg.Done()
		info, err := getVideoInfo(videoID)
		if err == nil {
			resp.Title = info["title"].(string)
		}
		fmt.Printf("~~~~~~~~~~~~~~~\n getVideoInfo err::\n%+v\n\n", err)
		return err
	}()

	// TODO: see if I can download both video and tumb

	// // get filename
	// wg.Add(1)
	// go func() error {
	// 	defer wg.Done()
	// 	var err error
	// 	filename, err = GetVideoFilename(videoID)
	// 	fmt.Printf("~~~~~~~~~~~~~~~\n GetVideoFilename err:\n%+v\n\n", err)
	// 	return err
	// }()

	// download video
	wg.Add(1)
	go func() error {
		defer wg.Done()
		var err error
		filename, err = downloadVideo(videoID)
		fmt.Printf("~~~~~~~~~~~~~~~\n downloadVideo err::\n%+v\n\n", err)
		return err
	}()

	wg.Wait()

	// validate that file exists
	if filename == "" {
		newestFile, _ := getNewestFile("song_files/")

		fmt.Printf("~~~~~~~~~~~~~~~\n newestFile:%+v\n\n", newestFile)

		return "", nil, fmt.Errorf("could not get filename")
	}
	if _, err := os.Stat(filename); err != nil {
		return "", nil, err
	}

	return filename, resp, nil
}

func getNewestFile(dir string) (string, error) {
	files, _ := ioutil.ReadDir(dir)
	var newestFile string
	var newestTime int64 = 0
	for _, f := range files {
		fi, err := os.Stat(dir + f.Name())
		if err != nil {
			return "", fmt.Errorf("[getNewestFile] os.Stat")
		}
		currTime := fi.ModTime().Unix()
		if currTime > newestTime {
			newestTime = currTime
			newestFile = f.Name()
		}
	}
	if newestFile == "" {
		return "", fmt.Errorf("[getNewestFile] no file")
	}
	return newestFile, nil
}

var (
	getVideoInfoCmd = NewDLCommand("yt-dlp --ignore-errors --no-call-home --no-cache-dir --skip-download --restrict-filenames -J")
)

func getVideoInfo(videoID string) (map[string]any, error) {
	std, err := getVideoInfoCmd.ExecB(videoID)
	if err != nil {
		fmt.Printf("~~~~~~~~~~~~~~~\n getvideoInfo err:\n%+v\n\n", err)
		fmt.Printf("~~~~~~~~~~~~~~~\n getvideoInfo out:\n%+v\n\n", std)
		return nil, fmt.Errorf("cmd err:%w", err)
	}
	out := map[string]any{}
	err = json.Unmarshal(std, &out)
	if err != nil {
		fmt.Printf("~~~~~~~~~~~~~~~\n getvideoInfo out:\n%+v\n\n", std)
		return nil, fmt.Errorf("json err:%w", err)
	}
	return out, nil
}

func GetVideoFilename(videoID string) (string, error) {
	// TODO: fix this command, figure out how to make it work with `yt-dlp`
	args := []string{
		"--ignore-errors",
		"--no-call-home",
		"--no-cache-dir",
		"--skip-download",
		"--restrict-filenames",
		"-f", "bestaudio",
		"--get-filename",
		"-o", `song_files/%(title)s-%(id)s.%(ext)s`,
	}
	args = append(args, videoID)
	cmd := exec.Command("yt-dlp", args...)
	std, err := cmd.Output()

	fmt.Printf("~~~~~~~~~~~~~~~\n GetVideoFilename out:\n%+v\n\n", string(std))

	// clean output
	outStr := string(std)
	outStr = strings.Trim(outStr, `"`)
	outStr = strings.TrimSpace(outStr)
	return outStr, err
}
