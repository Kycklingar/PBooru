package image

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/kycklingar/mimemagic"
)

func ThumbnailerInstalled() {
	fmt.Println("Checking if Image Magick is installed.. ")
	cmd := exec.Command("convert", "-version")
	if err := cmd.Run(); err != nil {
		fmt.Print("Not found in '$PATH'! Install instructions can be found https://www.imagemagick.org/\n")
	} else {
		fmt.Print("Found!\n")
	}

	fmt.Println("Checking if ffmpeg is installed.. ")
	cmd = exec.Command("ffmpeg", "-version")
	if err := cmd.Run(); err != nil {
		fmt.Print("Not found in '$PATH'! Install ffmpeg from https://ffmpeg.org/\n")
	} else {
		fmt.Print("Found!\n")
	}

	fmt.Println("Checking if ffprobe is installed.. ")
	cmd = exec.Command("ffprobe", "-version")
	if err := cmd.Run(); err != nil {
		fmt.Print("Not found in '$PATH'! Install ffprobe from https://ffmpeg.org/\n")
	} else {
		fmt.Print("Found!\n")
	}

	fmt.Println("Checking if mutool is installed..")
	cmd = exec.Command("mutool", "-v")
	if err := cmd.Run(); err != nil {
		fmt.Print("Not found in '$PATH'! Install instruction can be found at https://mupdf.com/\n")
	} else {
		fmt.Print("Found!\n")
	}

	fmt.Println("Checking if gnome-mobi-thumbnailer is installed.. ")
	cmd = exec.Command("gnome-mobi-thumbnailer", "-h")
	if err := cmd.Run(); err != nil {
		fmt.Print("Not found in '$PATH'! Source can be found at https://github.com/GNOME/gnome-epub-thumbnailer\n")
	} else {
		fmt.Print("Found!\n")
	}
}

func MakeThumbnail(file io.ReadSeeker, thumbnailFormat string, thumbnailSize, quality int) (*bytes.Buffer, error) {
	var err error

	buffer := make([]byte, 512)
	_, err = file.Read(buffer)
	if err != nil {
		log.Println(err)
		return nil, err
	}

	mime := mimemagic.MatchMagic(buffer)

	//fmt.Println(mime.MediaType())
	file.Seek(0, 0)

	var b *bytes.Buffer

	switch mime.MediaType() {
	case "application/pdf", "application/epub+zip":
		var m string
		if strings.Contains(mime.MediaType(), "pdf") {
			m = "pdf"
		} else if strings.Contains(mime.MediaType(), "epub") {
			m = "epub"
		}
		b, err = mupdf(file, m, thumbnailFormat, thumbnailSize, quality)
	case "application/x-mobipocket-ebook":
		b, err = gnomeMobi(file, thumbnailFormat, thumbnailSize, quality)
	default:
		if strings.Contains(mime.MediaType(), "image") {
			b, err = magickResize(file, thumbnailFormat, thumbnailSize, quality)
		} else if strings.Contains(mime.MediaType(), "video") {
			b, err = ffmpeg(file, thumbnailFormat, thumbnailSize, quality)
		} else {
			return nil, errors.New("unsupported mime")
		}
	}

	if err != nil {
		log.Println(err)
		return nil, err
	}

	return b, nil
}

func magickResize(file io.Reader, format string, size, quality int) (*bytes.Buffer, error) {
	tmpdir, err := ioutil.TempDir("", "pbooru-temp")
	if err != nil {
		log.Println(err)
		return nil, err
	}
	defer os.RemoveAll(tmpdir)

	args := []string{
		"-quiet",
		"-[0]",
		"-quality",
		fmt.Sprintf("%d", quality),
		"-strip",
		"-resize",
		fmt.Sprintf("%dx%d>", size, size),
		fmt.Sprintf("%s:%s", format, filepath.Join(tmpdir, "out")),
	}
	command := exec.Command("convert", args...)

	command.Stdin = file

	var b, er bytes.Buffer
	command.Stdout = &b
	command.Stderr = &er

	err = command.Run()
	if err != nil {
		log.Println(b.String(), er.String(), err)
		return nil, err
	}

	f, err := os.Open(filepath.Join(tmpdir, "out"))
	if err != nil {
		log.Println(err)
		return nil, err
	}
	defer f.Close()

	var buf bytes.Buffer
	buf.ReadFrom(f)

	return &buf, nil
}

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
