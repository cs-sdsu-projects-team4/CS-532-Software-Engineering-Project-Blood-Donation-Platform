package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Role is a user's access role. The string values match the Postgres user_role
// enum and the role claim carried in JWTs.
type Role string

const (
	RoleDonor          Role = "donor"
	RoleClinicEmployee Role = "clinic_employee"
	RoleClinicAdmin    Role = "clinic_admin"
	RoleSystemAdmin    Role = "system_admin"
)

// IsValidRole reports whether r is one of the four known roles.
func IsValidRole(r Role) bool {
	switch r {
	case RoleDonor, RoleClinicEmployee, RoleClinicAdmin, RoleSystemAdmin:
		return true
	default:
		return false
	}
}

// TokenType distinguishes short-lived access tokens from long-lived refresh
// tokens so a refresh token can never be replayed as an access token.
type TokenType string

const (
	AccessToken  TokenType = "access"
	RefreshToken TokenType = "refresh"
)

// ErrInvalidToken is returned for any token that fails to parse, is expired, has
// the wrong signature, or is the wrong token type.
var ErrInvalidToken = errors.New("invalid or expired token")

// Claims is the JWT payload. We embed RegisteredClaims for standard fields
// (expiry, issued-at, subject, issuer) and add our role + token type.
type Claims struct {
	Role Role      `json:"role"`
	Typ  TokenType `json:"typ"`
	jwt.RegisteredClaims
}

// TokenManager issues and verifies JWTs with a shared HMAC secret. In the MVP
// refresh tokens are stateless (signed, longer-lived) to avoid extra schema;
// token-type separation prevents cross-use.
type TokenManager struct {
	secret     []byte
	issuer     string
	accessTTL  time.Duration
	refreshTTL time.Duration
}

// NewTokenManager builds a TokenManager. secret must be non-empty.
func NewTokenManager(secret, issuer string, accessTTL, refreshTTL time.Duration) (*TokenManager, error) {
	if secret == "" {
		return nil, errors.New("jwt secret must not be empty")
	}
	return &TokenManager{
		secret:     []byte(secret),
		issuer:     issuer,
		accessTTL:  accessTTL,
		refreshTTL: refreshTTL,
	}, nil
}

// TokenPair is what login/refresh hand back to the client.
type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"` // always "Bearer"
	ExpiresIn    int    `json:"expires_in"` // access-token lifetime in seconds
}

// IssuePair mints a fresh access+refresh pair for a user.
func (m *TokenManager) IssuePair(userID int64, role Role) (TokenPair, error) {
	access, err := m.sign(userID, role, AccessToken, m.accessTTL)
	if err != nil {
		return TokenPair{}, err
	}
	refresh, err := m.sign(userID, role, RefreshToken, m.refreshTTL)
	if err != nil {
		return TokenPair{}, err
	}
	return TokenPair{
		AccessToken:  access,
		RefreshToken: refresh,
		TokenType:    "Bearer",
		ExpiresIn:    int(m.accessTTL.Seconds()),
	}, nil
}

func (m *TokenManager) sign(userID int64, role Role, typ TokenType, ttl time.Duration) (string, error) {
	now := time.Now()
	claims := Claims{
		Role: role,
		Typ:  typ,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   fmt.Sprintf("%d", userID),
			Issuer:    m.issuer,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(m.secret)
	if err != nil {
		return "", fmt.Errorf("sign token: %w", err)
	}
	return signed, nil
}

// Parse verifies a token's signature, expiry, and type, returning the user id
// and role. want selects which token type is acceptable at this call site (an
// access token on protected routes, a refresh token at the refresh endpoint).
func (m *TokenManager) Parse(tokenString string, want TokenType) (userID int64, role Role, err error) {
	claims := &Claims{}
	token, perr := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return m.secret, nil
	}, jwt.WithValidMethods([]string{"HS256"}), jwt.WithIssuer(m.issuer))
	if perr != nil || !token.Valid {
		return 0, "", ErrInvalidToken
	}
	if claims.Typ != want {
		return 0, "", ErrInvalidToken
	}

	var id int64
	if _, serr := fmt.Sscanf(claims.Subject, "%d", &id); serr != nil {
		return 0, "", ErrInvalidToken
	}
	if !IsValidRole(claims.Role) {
		return 0, "", ErrInvalidToken
	}
	return id, claims.Role, nil
}
