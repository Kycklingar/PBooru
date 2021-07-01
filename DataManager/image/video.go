package image

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

func FFmpegThumbnailer() FFmpeg {
	return FFmpeg{
		Accept: map[string]struct{}{
			"video/webm":       struct{}{},
			"video/mp4":        struct{}{},
			"video/x-matroska": struct{}{},
			"video/x-msvideo":  struct{}{},
			"video/quicktime":  struct{}{},
			"video/x-flv":      struct{}{},
		},
	}
}

type FFmpeg struct {
	Accept map[string]struct{}
}

func (ffm FFmpeg) Accepts(mime string) bool {
	_, ok := ffm.Accept[mime]
	return ok
}

func (ffm FFmpeg) Resize(input io.ReadSeeker, format Format) (io.ReadSeekCloser, error) {
	// Must use a tempfile for ffmpeg
	// Stdin fails for no reason
	tvideo, err := tempFile()
	if err != nil {
		return nil, err
	}
	defer tvideo.Close()

	_, err = io.Copy(tvideo, input)
	if err != nil {
		return nil, err
	}

	_, err = tvideo.Seek(0, 0)
	if err != nil {
		return nil, err
	}

	dur, err := probeForDuration(tvideo)
	if err != nil {
		log.Println(err)
	}

	// If no duration, try first second frame
	if dur <= 0 {
		dur = time.Second
	} else {
		dur = dur / 2
	}

	args := []string{
		"-hide_banner",
		"-loglevel",
		"8",
		"-ss",
		fmt.Sprint(dur.Seconds()),
		"-i",
		tvideo.Name(),
		"-f",
		"mjpeg",
		"-frames:v",
		"1",
		"-",
	}

	cmd := exec.Command("ffmpeg", args...)

	output, err := tempFile()
	if err != nil {
		return nil, err
	}
	defer output.Close()

	var stderr bytes.Buffer

	cmd.Stdout = output
	cmd.Stderr = &stderr

	if err = cmd.Run(); err != nil {
		return nil, fmt.Errorf("Error ffmpeg: %v\n%s\n", err, stderr.Bytes())
	}

	if _, err = output.Seek(0, 0); err != nil {
		return nil, err
	}

	return magickResize(output, format)
}

var durationProbeRegex = regexp.MustCompile("Duration: (\\d+:\\d+:\\d+.\\d+)")

func probeForDuration(input io.Reader) (time.Duration, error) {
	args := []string{
		"-hide_banner",
		"-",
	}

	cmd := exec.Command("ffprobe", args...)
	cmd.Stdin = input

	out, err := cmd.CombinedOutput()
	if err != nil {
		return time.Duration(0), fmt.Errorf("ffprobe error: %v\n%s", err, out)
	}

	match := durationProbeRegex.FindStringSubmatch(string(out))
	if len(match) != 2 {
		return time.Duration(0), fmt.Errorf("ffprobe error: could not locate valid duration")
	}

	duration := match[1]

	duration = strings.Replace(duration, ":", "h", 1)
	duration = strings.Replace(duration, ":", "m", 1)
	duration = strings.Replace(duration, ".", "s", 1)
	duration += "0ms"
	return time.ParseDuration(duration)
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
