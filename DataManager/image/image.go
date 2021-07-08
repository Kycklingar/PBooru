package image

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"os/exec"
	"strconv"
	"strings"
)

func ImageMagickThumbnailer() ImageMagick {
	return ImageMagick{
		Accept: map[string]struct{}{
			"image/png":  struct{}{},
			"image/jpeg": struct{}{},
			"image/gif":  struct{}{},
			"image/webp": struct{}{},
		},
	}
}

type ImageMagick struct {
	Accept map[string]struct{}
}

func (i ImageMagick) Accepts(mime string) bool {
	_, ok := i.Accept[mime]
	return ok
}

func (i ImageMagick) Resize(input io.ReadSeeker, format Format) (io.ReadSeekCloser, error) {
	return magickResize(input, format)
}

func magickResize(input io.Reader, format Format) (io.ReadSeekCloser, error) {
	args := []string{
		"-quiet",
		"-[0]",
		"-quality",
		fmt.Sprint(format.Quality),
		"-strip",
	}
	args = append(args, format.ResizeFunc(format.Width, format.Height)...)
	args = append(args, fmt.Sprintf("%s:-", format.Mime))

	cmd := exec.Command("convert", args...)

	output, err := tempFile()
	if err != nil {
		return nil, err
	}

	var stderr bytes.Buffer

	cmd.Stdin = input
	cmd.Stdout = output
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		output.Close()
		return nil, fmt.Errorf("Error magickResize: %v\n%s", err, stderr.Bytes())
	}

	if _, err = output.Seek(0, 0); err != nil {
		output.Close()
		return nil, err
	}

	return output, nil
}

//func magickResize(file io.Reader, format string, size, quality int) (*bytes.Buffer, error) {
//	tmpdir, err := ioutil.TempDir("", "pbooru-temp")
//	if err != nil {
//		log.Println(err)
//		return nil, err
//	}
//	defer os.RemoveAll(tmpdir)
//
//	args := []string{
//		"-quiet",
//		"-[0]",
//		"-quality",
//		fmt.Sprintf("%d", quality),
//		"-strip",
//		"-resize",
//		fmt.Sprintf("%dx%d>", size, size),
//		fmt.Sprintf("%s:%s", format, filepath.Join(tmpdir, "out")),
//	}
//	command := exec.Command("convert", args...)
//
//	command.Stdin = file
//
//	var b, er bytes.Buffer
//	command.Stdout = &b
//	command.Stderr = &er
//
//	err = command.Run()
//	if err != nil {
//		log.Println(b.String(), er.String(), err)
//		return nil, err
//	}
//
//	f, err := os.Open(filepath.Join(tmpdir, "out"))
//	if err != nil {
//		log.Println(err)
//		return nil, err
//	}
//	defer f.Close()
//
//	var buf bytes.Buffer
//	buf.ReadFrom(f)
//
//	return &buf, nil
//}

func GetDimensions(file io.Reader) (int, int, error) {
	args := []string{
		"-ping",
		"-quiet",
		"-format",
		"%wx%h",
		"-[0]",
	}

	cmd := exec.Command("identify", args...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		log.Println(err)
		return 0, 0, err
	}

	go func() {
		defer stdin.Close()
		_, err = io.Copy(stdin, file)
		if err != nil {
			log.Println(err)
		}
	}()

	out, err := cmd.CombinedOutput()
	if err != nil {
		log.Println(string(out), err)
		return 0, 0, err
	}

	wh := strings.Split(string(out), "x")
	if len(wh) != 2 {
		return 0, 0, errors.New("No width/height " + string(out))
	}

	width, err := strconv.Atoi(wh[0])
	if err != nil {
		log.Println(err)
		return 0, 0, err
	}
	height, err := strconv.Atoi(wh[1])
	if err != nil {
		log.Println(err)
		return 0, 0, err
	}

	return width, height, nil
}
