package DataManager

type strindex func(int) string

func sep(sep string, length int, f strindex) string {
	if length <= 0 {
		return ""
	}

	var retu string

	for i := 0; i < length-1; i++ {
		retu += f(i) + sep
	}

	return retu + f(length-1)
}
