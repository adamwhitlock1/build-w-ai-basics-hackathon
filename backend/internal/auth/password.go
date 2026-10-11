// Package auth handles who is making a request: password hashing, the
// session cookie, and middleware that loads the logged-in user (or turns
// the request away).
package auth

import (
	"fmt"
	"sync"

	"golang.org/x/crypto/bcrypt"
)

// MaxPasswordBytes is the longest password bcrypt accepts. It ignores
// anything after 72 bytes, so longer passwords are rejected rather than
// silently cut short.
const MaxPasswordBytes = 72

// HashPassword returns a bcrypt hash of plain, salted and slow to brute
// force. It fails for passwords longer than MaxPasswordBytes.
func HashPassword(plain string) ([]byte, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}
	return hash, nil
}

// CheckPassword reports whether plain matches hash.
//
// Pass a nil hash when there is no such user: CheckPassword still does the
// work of a real check (against a dummy hash) and returns false. That way a
// login for an unknown email takes as long as one with a wrong password, and
// response times don't reveal which emails have accounts.
func CheckPassword(plain string, hash []byte) bool {
	if hash == nil {
		_ = bcrypt.CompareHashAndPassword(dummyHash(), []byte(plain))
		return false
	}
	return bcrypt.CompareHashAndPassword(hash, []byte(plain)) == nil
}

// dummyHash is computed the first time it's needed and reused after that:
// sync.OnceValue wraps a function so it runs at most once. New calls it at
// startup, so no login has to wait for it.
var dummyHash = sync.OnceValue(func() []byte {
	hash, err := bcrypt.GenerateFromPassword([]byte("porchlight-dummy-password"), bcrypt.DefaultCost)
	if err != nil {
		panic(err) // only possible for passwords over 72 bytes
	}
	return hash
})
