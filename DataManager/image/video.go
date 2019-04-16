package image

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

func ffmpeg(file io.ReadSeeker, format string, size int) (*bytes.Buffer, error) {
	tmpFile, err := ioutil.TempFile("", "pbooru-temp")
	if err != nil {
		log.Println(err)
		return nil, err
	}
	defer os.Remove(tmpFile.Name())

	if _, err = io.Copy(tmpFile, file); err != nil {
		log.Println(err)
		tmpFile.Close()
		return nil, err
	}
	tmpFile.Close()

	args := []string{
		"-hide_banner",
		tmpFile.Name(),
	}

	cmd := exec.Command("ffprobe", args...)
	cmd.Stdin = file

	b, err := cmd.CombinedOutput()
	if err != nil {
		log.Println(string(b), err)
	}

	r, err := regexp.Compile("(Duration: \\d+:\\d+:\\d+.\\d+)")
	if err != nil {
		log.Println(err)
		return nil, err
	}

	duration := strings.Replace(r.FindString(string(b)), "Duration: ", "", 1)
	duration = strings.Replace(duration, ":", "h", 1)
	duration = strings.Replace(duration, ":", "m", 1)
	duration = strings.Replace(duration, ".", "s", 1)
	duration += "0ms"
	t, err := time.ParseDuration(duration)
	if err != nil {
		log.Println(err)
		return nil, err
	}

	if t <= 0 {
		return nil, fmt.Errorf("video has no duration %s", duration)
	}

	t = t / 2

	args = []string{
		"-hide_banner",
		"-i",
		tmpFile.Name(),
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
