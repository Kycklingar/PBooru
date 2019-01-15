package DataManager

import (
	"bytes"
	"errors"
	"fmt"
	"image/png"
	"io"
	"log"
	"os/exec"
	"os"
	"io/ioutil"

	"github.com/Nr90/imgsim"
	"github.com/zRedShift/mimemagic"
)

func makeThumbnail(file io.ReadSeeker, thumbnailSize int) (string, error) {
	var err error

	buffer := make([]byte, 512)
	_, err = file.Read(buffer)
	if err != nil {
		log.Println(err)
		return "", err
	}

	//mime := http.DetectContentType(buffer)

	mime := mimemagic.MatchMagic(buffer)

	fmt.Println(mime.MediaType())
	file.Seek(0, 0)

	var b *bytes.Buffer

	switch mime.MediaType(){
		case "image/png", "image/jpeg", "image/gif", "image/webp":
			b, err = magickResize(file, CFG.ThumbnailFormat, thumbnailSize)
		case "application/pdf", "application/epub+zip":
			b, err = mupdf(file, CFG.ThumbnailFormat, thumbnailSize)
		default:
			return "", nil
	}

	if err != nil{
		log.Println(err)
		return "", err
	}

	thumbHash, err := ipfsAdd(b)
	if err != nil {
		log.Println(err)
		return "", err
	}

	err = mfsCP(fmt.Sprint(CFG.MFSRootDir, "thumbnails/", thumbnailSize, "/"), thumbHash, true)

	return thumbHash, err
}

func magickResize(file io.Reader, format string, size int) (*bytes.Buffer, error) {
	args := []string{
		"-[0]",
		"-quality",
		"75",
		"-strip",
		"-resize",
		fmt.Sprintf("%dx%d\\>", size, size),
		fmt.Sprintf("%s:-", format),
	}
	command := exec.Command("magick", args...)

	command.Stdin = file

	var b, er bytes.Buffer
	var err error
	command.Stdout = &b
	command.Stderr = &er

	err = command.Run()
	if err != nil {
		log.Println(b.String(), er.String(), err)
		return nil, err
	}

	if len(b.Bytes()) <= 0 {
		return nil, errors.New("nolength buffer")
	}

	return &b, nil
}

func mupdf(file io.Reader, format string, size int)(*bytes.Buffer, error){
	tmp, err := ioutil.TempFile("", "pbooru-tmp")
	if err != nil{
		log.Println(err)
		return nil, err
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()

	var tmpbuf bytes.Buffer
	tmpbuf.ReadFrom(file)

	_, err = tmp.Write(tmpbuf.Bytes())
	if err != nil{
		log.Println(err)
		return nil, err
	}

	fmt.Println(tmp.Name())

	args := []string{
		"-dNOPAUSE",
		"-q",
		"-dBATCH",
		"-r200",
		"-sDEVICE=png256",
		"-dLastPage=1",
		"-sOutputFile=-",
		tmp.Name(),
	}

	cmd := exec.Command("gs", args...)

	//res, err := cmd.CombinedOutput()
	//if err != nil{
	//	log.Println(string(res), err)
	//	return nil, err
	//}

	var b, er bytes.Buffer
	cmd.Stdout = &b
	cmd.Stderr = &er

	err = cmd.Run()
	if err != nil{
		log.Println(b.String(), er.String(), err)
		return nil, err
	}

	f := bytes.NewReader(b.Bytes())
	return magickResize(f, format, size)
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
