package toolkit

import "crypto/rand"

const randomSourceString = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ01923456789_+#@%^&*"

/**
* Tool is the type used to instantiate this module. Any variable of this type will
* have access to all the methods with the receiver *Tools
 */
type Tools struct{}

// RandomString return a random string of length n
func (t *Tools) RandomString(n int) string {
	s, r := make([]rune, n), []rune(randomSourceString)

	for i := range s {
		p, _ := rand.Prime(rand.Reader, len(r))
		x, y := p.Uint64(), uint64(len(r))
		s[i] = r[x%y]
	}

	return string(s)
}
