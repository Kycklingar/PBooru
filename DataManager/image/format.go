package image

import "fmt"

type Format struct {
	Width, Height uint
	Mime          string
	Quality       int
	ResizeFunc    ResizeFunc
}

type ResizeFunc func(width, height uint) []string

func ShrinkKeepAspect(width, height uint) []string {
	return []string{
		"-resize",
		fmt.Sprintf("%dx%d>", width, height),
	}
}
