package session

import "crypto/rand"

type Key string

var alph = []rune("0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

func generateKey(length int) Key {
	var b = make([]byte, length)
	var k = make([]rune, length)

	_, err := rand.Read(b)
	if err != nil {
		panic(err)
	}

	for i := 0; i < length; i++ {
		k[i] = alph[int(b[i])%len(alph)]
	}

	return Key(k)
}
