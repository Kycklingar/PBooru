package DataManager

// strindex returns the string represantation of the type at supplied index
type strindex func(int) string

// Separates a slice type using sep and the types strindex function
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
