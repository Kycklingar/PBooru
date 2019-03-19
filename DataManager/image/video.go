package image

import (
	"bytes"
	"strings"
	"fmt"
	"io"
	"log"
	"os/exec"
	"regexp"
	"time"
)

func ffmpeg(file io.ReadSeeker, format string, size int) (*bytes.Buffer, error) {
	args := []string{
		"-hide_banner",
		"-",
	}

	cmd := exec.Command("ffprobe", args...)
	cmd.Stdin = file

	b, err := cmd.CombinedOutput()
	if err != nil {
		log.Println(string(b), err)
	}

	r, err := regexp.Compile("(\\d+:\\d+:\\d+.\\d+)")
	if err != nil {
		log.Println(err)
		return nil, err
	}

	duration := r.FindString(string(b))
	duration = strings.Replace(duration, ":", "h", 1)
	duration = strings.Replace(duration, ":", "m", 1)
	duration = strings.Replace(duration, ".", "s", 1)
	duration += "0ms"
	t, err := time.ParseDuration(duration)
	if err != nil {
		log.Println(err)
		return nil, err
	}

	t = t / 2

	//mid := fmt.Sprintf("%d:%d:%d.%d", int(t.Hours()), int(t.Minutes()), int(t.Seconds()), t / time.Millisecond)

	args = []string{
		"-hide_banner",
		"-i",
		"-",
		"-f",
		"mjpeg",
		"-vframes",
		"1",
		"-ss",
		fmt.Sprint(int(t.Seconds())),
		"-",
	}

	file.Seek(0, 0)
	cmd = exec.Command("ffmpeg", args...)
	cmd.Stdin = file

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Println(err)
		return nil, err
	}

	defer stdout.Close()

	err = cmd.Start()
	//b, err = cmd.CombinedOutput()
	if err != nil {
		log.Println(err)
		return nil, err
	}

	out, err := magickResize(stdout, format, size)
	if err != nil {
		log.Println(err)
		return nil, err
	}

	if err = cmd.Wait(); err != nil {
		log.Println(err)
		return nil, err
	}
	return out, nil
}

//func vidDim() {
//	args := []string{
//		"-hide_banner",
//		"-",
//	}
//
//	cmd := exec.Command("ffprobe", args...)
//	cmd.Stdin = file
//
//	b, err := cmd.CombinedOutput()
//	if err != nil {
//		log.Println(string(b), err)
//	}
//
//	r, err := regexp.Compile("(\\d+x\\d+)")
//	if err != nil {
//		log.Println(err)
//		return nil, err
//	}
//
//	fmt.Println(r.FindString(string(b)))
//
//	return nil, errors.New("not ready")
//}
