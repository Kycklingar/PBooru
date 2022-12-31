package flag

type Flag int

const (
	Tagging Flag = 0x1
	Upload       = 0x2
	Comics       = 0x4
	Banning      = 0x8
	Delete       = 0x10
	Tags         = 0x20
	Special      = 0x40

	All = 0xff
)

type flagstr struct {
	f   Flag
	str string
}

func (f flagstr) String() string {
	return f.str
}

var flagstrs = []flagstr{
	flagstr{Special, "Admin"},
	flagstr{Banning, "Banning"},
	flagstr{Delete, "Delete"},
	flagstr{Tags, "Tags"},
	flagstr{Comics, "Comics"},
	flagstr{Upload, "Upload"},
	flagstr{Tagging, "Tagging"},
}

func (f Flag) Tagging() bool {
	return f&Tagging != 0
}

func (f Flag) Upload() bool {
	return f&Upload != 0
}

func (f Flag) Comics() bool {
	return f&Comics != 0
}

func (f Flag) Banning() bool {
	return f&Banning != 0
}

func (f Flag) Delete() bool {
	return f&Delete != 0
}

func (f Flag) Tags() bool {
	return f&Tags != 0
}

func (f Flag) Special() bool {
	return f&Special != 0
}

func (f Flag) Flags() []flagstr {
	var flags []flagstr
	for _, flag := range flagstrs {
		if f&flag.f != 0 {
			flags = append(flags, flag)
		}
	}

	return flags
}
