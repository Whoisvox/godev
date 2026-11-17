package codegen

import (
	"crypto/rand"
	"errors"
	"math/big"
)

const alphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

// New() create n bit base62 random string
func New(n int) (string, error) {
	if n <= 0 {
		return "", errors.New("n must be positive")
	}

	b := make([]byte, n)
	max := big.NewInt(int64(len(alphabet)))

	for i := 0; i < n; i++ {
		v, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", err
		}
		b[i] = alphabet[v.Int64()]
	}
	return string(b), nil
}
