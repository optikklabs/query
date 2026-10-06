package token

import (
	"errors"
	"slices"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/optikklabs/query/internal/config"
)

const (
	typAccess        = "access"
	typPasswordReset = "pwd_reset"
	passwordResetTTL = 30 * time.Minute
)

type AuthState struct {
	UserID          int64
	Email           string
	Role            string
	DefaultTenantID int64
	TenantIDs       []int64
}

type accessClaims struct {
	Typ             string  `json:"typ"`
	Email           string  `json:"email"`
	Role            string  `json:"role"`
	DefaultTenantID int64   `json:"dtid"`
	TenantIDs       []int64 `json:"tids"`
	jwt.RegisteredClaims
}

type resetClaims struct {
	Typ string `json:"typ"`
	jwt.RegisteredClaims
}

type Service struct {
	secret     []byte
	accessTTL  time.Duration
	refreshTTL time.Duration
	cookie     cookieOpts
}

func NewService(cfg config.Config) *Service {
	return &Service{
		secret:     []byte(cfg.Auth.JWTSecret),
		accessTTL:  cfg.AccessTokenTTL(),
		refreshTTL: cfg.RefreshTokenTTL(),
		cookie: cookieOpts{
			name:     cfg.Auth.RefreshCookieName,
			domain:   cfg.Auth.CookieDomain,
			secure:   cfg.Auth.CookieSecure,
			sameSite: parseSameSite(cfg.Auth.CookieSameSite),
		},
	}
}

func (s *Service) SignAccess(state AuthState) (string, error) {
	now := time.Now()
	claims := accessClaims{
		Typ:             typAccess,
		Email:           state.Email,
		Role:            state.Role,
		DefaultTenantID: state.DefaultTenantID,
		TenantIDs:       state.TenantIDs,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   strconv.FormatInt(state.UserID, 10),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(s.accessTTL)),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.secret)
}

func (s *Service) ParseAccess(raw string) (AuthState, error) {
	var claims accessClaims
	if err := parseClaims(raw, &claims, s.secret); err != nil {
		return AuthState{}, err
	}
	if claims.Typ != typAccess {
		return AuthState{}, errors.New("token is not an access token")
	}
	userID, err := parseSubject(claims.Subject)
	if err != nil {
		return AuthState{}, err
	}
	return AuthState{
		UserID:          userID,
		Email:           claims.Email,
		Role:            claims.Role,
		DefaultTenantID: claims.DefaultTenantID,
		TenantIDs:       claims.TenantIDs,
	}, nil
}

func (s *Service) SignPasswordReset(userID int64, passwordHash string) (string, error) {
	now := time.Now()
	claims := resetClaims{
		Typ: typPasswordReset,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   strconv.FormatInt(userID, 10),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(passwordResetTTL)),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.resetSecret(passwordHash))
}

func (s *Service) ParsePasswordReset(raw string, passwordHash string) (int64, error) {
	var claims resetClaims
	if err := parseClaims(raw, &claims, s.resetSecret(passwordHash)); err != nil {
		return 0, err
	}
	if claims.Typ != typPasswordReset {
		return 0, errors.New("token is not a password reset token")
	}
	return parseSubject(claims.Subject)
}

// ResetTokenSubject reads the user ID from a reset token without verifying
// it, to look up the password hash the token must then verify against.
func ResetTokenSubject(raw string) (int64, error) {
	var claims resetClaims
	if _, _, err := jwt.NewParser().ParseUnverified(raw, &claims); err != nil {
		return 0, err
	}
	return parseSubject(claims.Subject)
}

// resetSecret keys reset tokens to the current password hash, so a token stops
// verifying once the password changes. slices.Concat always allocates, so
// the shared secret's backing array is never written.
func (s *Service) resetSecret(passwordHash string) []byte {
	return slices.Concat(s.secret, []byte(passwordHash))
}

func parseClaims(raw string, claims jwt.Claims, secret []byte) error {
	parser := jwt.NewParser(
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithLeeway(30*time.Second),
		jwt.WithExpirationRequired(),
	)
	_, err := parser.ParseWithClaims(raw, claims, func(*jwt.Token) (any, error) {
		return secret, nil
	})
	return err
}

func parseSubject(subject string) (int64, error) {
	userID, err := strconv.ParseInt(subject, 10, 64)
	if err != nil || userID <= 0 {
		return 0, errors.New("invalid token subject")
	}
	return userID, nil
}
