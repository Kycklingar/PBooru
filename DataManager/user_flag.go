package DataManager

type flag int

const (
	flagTagging = 1
	flagUpload  = 2
	flagComics  = 4
	flagBanning = 8
	flagDelete  = 16
	flagTags    = 32
	flagSpecial = 64

	flagAll = 0xff
)

func (f flag) Tagging() bool {
	return f&flagTagging != 0
}

func (f flag) Upload() bool {
	return f&flagUpload != 0
}

func (f flag) Comics() bool {
	return f&flagComics != 0
}

func (f flag) Banning() bool {
	return f&flagBanning != 0
}

func (f flag) Delete() bool {
	return f&flagDelete != 0
}

func (f flag) Tags() bool {
	return f&flagTags != 0
}

func (f flag) Special() bool {
	return f&flagSpecial != 0
}
