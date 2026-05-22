package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// Claims is the JWT payload. Subject = user id; TeamID is embedded so the
// auth middleware can inject a fully-formed Actor into the request context
// without a DB round-trip on every authenticated call.
type Claims struct {
	TeamID uuid.UUID `json:"team_id"`
	jwt.RegisteredClaims
}

type Issuer struct {
	secret   []byte
	ttl      time.Duration
	issuer   string
	audience string
}

func NewIssuer(secret string, ttl time.Duration, iss, aud string) *Issuer {
	return &Issuer{secret: []byte(secret), ttl: ttl, issuer: iss, audience: aud}
}

func (i *Issuer) Issue(userID, teamID uuid.UUID) (string, error) {
	now := time.Now()
	claims := Claims{
		TeamID: teamID,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID.String(),
			Issuer:    i.issuer,
			Audience:  jwt.ClaimStrings{i.audience},
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(i.ttl)),
			ID:        uuid.NewString(),
		},
	}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return t.SignedString(i.secret)
}

var ErrInvalidToken = errors.New("invalid token")

func (i *Issuer) Parse(raw string) (Claims, error) {
	tok, err := jwt.ParseWithClaims(raw, &Claims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return i.secret, nil
	},
		jwt.WithIssuer(i.issuer),
		jwt.WithAudience(i.audience),
		jwt.WithValidMethods([]string{"HS256"}),
	)
	if err != nil {
		return Claims{}, err
	}
	claims, ok := tok.Claims.(*Claims)
	if !ok || !tok.Valid {
		return Claims{}, ErrInvalidToken
	}
	return *claims, nil
}
