package DataManager

import (
	"bytes"
	"fmt"
	"image/png"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"encoding/binary"

	"github.com/Nr90/imgsim"
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

func makeThumbnail(file io.ReadSeeker, thumbnailSize int) (string, error) {
	var err error

	buffer := make([]byte, 512)
	_, err = file.Read(buffer)
	if err != nil {
		log.Println(err)
		return "", err
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
		b, err = mupdf(file, m, CFG.ThumbnailFormat, thumbnailSize)
	case "application/x-mobipocket-ebook":
		b, err = gnomeMobi(file, CFG.ThumbnailFormat, thumbnailSize)
	default:
		if strings.Contains(mime.MediaType(), "image") {
			b, err = magickResize(file, CFG.ThumbnailFormat, thumbnailSize)
		} else {
			return "", nil
		}
	}

	if err != nil {
		log.Println(err)
		return "", err
	}

	thumbHash, err := ipfsAdd(b)
	if err != nil {
		log.Println(err)
		return "", err
	}

	if CFG.UseMFS {
		err = mfsCP(fmt.Sprint(CFG.MFSRootDir, "thumbnails/", thumbnailSize, "/"), thumbHash, true)
	}

	return thumbHash, err
}

func magickResize(file io.Reader, format string, size int) (*bytes.Buffer, error) {
	tmpdir, err := ioutil.TempDir("", "pbooru-temp")
	if err != nil {
		log.Println(err)
		return nil, err
	}
	defer os.RemoveAll(tmpdir)

	args := []string{
		"-[0]",
		"-quality",
		"75",
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

func mupdf(file io.Reader, mime, format string, size int) (*bytes.Buffer, error) {
	tmpdir, err := ioutil.TempDir("", "pbooru-tmp")
	if err != nil {
		log.Println(err)
		return nil, err

	}
	defer os.RemoveAll(tmpdir)

	var tmpbuf bytes.Buffer
	tmpbuf.ReadFrom(file)

	err = ioutil.WriteFile(fmt.Sprintf("%s/file.%s", tmpdir, mime), tmpbuf.Bytes(), 0660)

	if err != nil {
		log.Println(err)
		return nil, err
	}

	args := []string{
		"draw",
		"-o",
		"",
		"-F",
		"png",
		fmt.Sprintf("%s/file.%s", tmpdir, mime),
		"1",
	}

	cmd := exec.Command("mutool", args...)

	var b, er bytes.Buffer
	cmd.Stdout = &b
	cmd.Stderr = &er

	err = cmd.Run()
	if err != nil {
		log.Println(b.String(), er.String(), err)
		return nil, err
	}

	f := bytes.NewReader(b.Bytes())
	return magickResize(f, format, size)
}

func gnomeMobi(file io.Reader, format string, size int) (*bytes.Buffer, error) {
	tmpdir, err := ioutil.TempDir("", "pbooru-tmp")
	if err != nil {
		log.Println(err)
		return nil, err
	}
	defer os.RemoveAll(tmpdir)

	var tmpbuf bytes.Buffer
	tmpbuf.ReadFrom(file)

	err = ioutil.WriteFile(fmt.Sprintf("%s/file.%s", tmpdir, "mobi"), tmpbuf.Bytes(), 0660)
	if err != nil {
		log.Println(err)
		return nil, err
	}

	args := []string{
		"-s",
		strconv.Itoa(2048),
		fmt.Sprintf("%s/file.mobi", tmpdir),
		fmt.Sprintf("%s/out.png", tmpdir),
	}

	cmd := exec.Command("gnome-mobi-thumbnailer", args...)

	var b, er bytes.Buffer
	cmd.Stdout = &b
	cmd.Stderr = &er

	err = cmd.Run()
	if err != nil {
		log.Println(b.String(), er.String(), err)
		return nil, err
	}

	f, err := os.Open(filepath.Join(tmpdir, "out.png"))

	if err != nil {
		log.Println(err)
		return nil, err
	}

	defer f.Close()

	return magickResize(f, format, size)
}

func ImageLookup(file io.Reader, distance int) ([]*Post, error) {
	hash := dHash(file)
	if hash == 0{
		return nil, nil
	}

	type phash struct {
		post_id int
		h1      uint16
		h2      uint16
		h3      uint16
		h4      uint16
	}

	var ph phash
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, hash)
	ph.h1 = uint16(b[1]) | uint16(b[0])<<8
	ph.h2 = uint16(b[3]) | uint16(b[2])<<8
	ph.h3 = uint16(b[5]) | uint16(b[4])<<8
	ph.h4 = uint16(b[7]) | uint16(b[6])<<8

	rows, err := DB.Query("SELECT * FROM phash WHERE h1=$1 OR h2=$2 OR h3=$3 OR h4=$4 ORDER BY post_id DESC", ph.h1, ph.h2, ph.h3, ph.h4)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var phs []phash

	for rows.Next() {
		var phn phash
		if err = rows.Scan(&phn.post_id, &phn.h1, &phn.h2, &phn.h3, &phn.h4); err != nil {
			return nil, err
		}
		phs = append(phs, phn)
	}
	f := func(a phash) imgsim.Hash {
		return imgsim.Hash(uint64(a.h1)<<16 | uint64(a.h2)<<32 | uint64(a.h3)<<48 | uint64(a.h4)<<64)
	}

	var posts []*Post
	hasha := f(ph)

	for _, h := range phs {
		hashb := f(h)
		if imgsim.Distance(hasha, hashb) < distance{
			pst := NewPost()
			pst.ID = h.post_id
			posts = append(posts, pst)
		}
	}

	return posts, nil
}

func dHash(file io.Reader) uint64 {
	b, err := magickResize(file, "png", 1024)
	if err != nil {
		log.Println(err)
		return 0
	}

	img, err := png.Decode(b)
	if err != nil {
		log.Println(err)
		return 0
	}
	hash := imgsim.DifferenceHash(img)
	return uint64(hash)
}
