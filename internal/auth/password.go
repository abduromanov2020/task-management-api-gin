package auth

import "golang.org/x/crypto/bcrypt"

// BcryptCost is the work factor used for password hashing. 12 is a common
// production-leaning choice for 2026 hardware; it can be tuned via the
// PasswordHasher constructor if cost-vs-latency needs to shift.
const BcryptCost = 12

// DummyHash is a precomputed bcrypt hash used in the login flow to keep
// constant-time behavior when the email is not found, defeating timing-based
// email enumeration. The plaintext is "this-password-will-never-match".
const DummyHash = "$2a$12$0qg.QY4N3aT3wEZqQpqXq.aBkPgFqK6BkV3wQwL6mYxq3Q2tEpQS6"

type PasswordHasher struct{ cost int }

func NewPasswordHasher() *PasswordHasher { return &PasswordHasher{cost: BcryptCost} }

func (p *PasswordHasher) Hash(plain string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(plain), p.cost)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// Compare returns nil on a match. On mismatch returns bcrypt.ErrMismatchedHashAndPassword.
func (p *PasswordHasher) Compare(hash, plain string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain))
}
