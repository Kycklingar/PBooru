package user

import "testing"

func isCorrectTitle(t *testing.T, count int, expected string) {
	got := title(count)
	if got != expected {
		t.Errorf("expected title '%s' but got '%s'", expected, got)
	}
}

func TestTitle(t *testing.T) {
	isCorrectTitle(t, 0, "")
	isCorrectTitle(t, 1, titles[0])
	isCorrectTitle(t, 295, titles[2])
	isCorrectTitle(t, 999999, titles[5])
	isCorrectTitle(t, 6666666666, titles[6])
	isCorrectTitle(t, -100, "")
}
