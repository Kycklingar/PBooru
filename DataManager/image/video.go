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

func ffmpeg(file io.ReadSeeker, format string, size, quality int) (*bytes.Buffer, error) {
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

	// No duration, try using first second frame
	if t <= 0 {
		//return nil, fmt.Errorf("video has no duration %s", duration)
		t = time.Second
	}

	t = t / 2


	output, err := run(int(t.Seconds()), tmpFile.Name())
	if err != nil && !(len(output) > 0) {
		// Retry with the first frame
		output, err = run(0, tmpFile.Name())
		if err != nil && !(len(output) > 0) {
			log.Println(err)
			return nil, err
		}
	}

	br := bytes.NewReader(output)

	out, err := magickResize(br, format, size, quality)
	if err != nil {
		log.Println(err)
		return nil, err
	}

	return out, nil
}

func run(sec int, filename string) ([]byte, error) {
	args := []string{
		"-hide_banner",
		"-loglevel",
		"8",
		"-ss",
		fmt.Sprint(sec),
		"-i",
		filename,
		"-f",
		"mjpeg",
		"-frames",
		"1",
		"-",
	}

	cmd := exec.Command("ffmpeg", args...)

	return cmd.CombinedOutput()
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
