package image

import (
	"fmt"
	"os/exec"
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
