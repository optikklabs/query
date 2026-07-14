package token

import (
	"fmt"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/optikklabs/query/internal/config"
)

const typAccess = "access"
const typPasswordReset = "pwd_reset"

// AuthState carries the authenticated user identity embedded in tokens.
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
	if err := s.parse(raw, &claims); err != nil {
		return AuthState{}, err
	}
	if claims.Typ != typAccess {
		return AuthState{}, fmt.Errorf("token is not an access token")
	}
	userID, err := strconv.ParseInt(claims.Subject, 10, 64)
	if err != nil || userID == 0 {
		return AuthState{}, fmt.Errorf("invalid token subject")
	}
	return AuthState{
		UserID:          userID,
		Email:           claims.Email,
		Role:            claims.Role,
		DefaultTenantID: claims.DefaultTenantID,
		TenantIDs:       claims.TenantIDs,
	}, nil
}

func (s *Service) parse(raw string, claims jwt.Claims) error {
	parser := jwt.NewParser(
		jwt.WithValidMethods([]string{"HS256"}),
		jwt.WithLeeway(30*time.Second),
		jwt.WithExpirationRequired(),
	)
	_, err := parser.ParseWithClaims(raw, claims, func(t *jwt.Token) (any, error) {
		return s.secret, nil
	})
	return err
}

// SignPasswordReset signs a short-lived token using a dynamic secret that includes the user's current password hash.
func (s *Service) SignPasswordReset(userID int64, passwordHash string) (string, error) {
	now := time.Now()
	// Short TTL for password resets (e.g. 30 mins)
	ttl := 30 * time.Minute
	claims := resetClaims{
		Typ: typPasswordReset,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   strconv.FormatInt(userID, 10),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
	}
	secret := append(s.secret, []byte(passwordHash)...)
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(secret)
}

// ParsePasswordReset parses and validates a password reset token using the dynamic secret.
func (s *Service) ParsePasswordReset(raw string, passwordHash string) (int64, error) {
	var claims resetClaims
	parser := jwt.NewParser(
		jwt.WithValidMethods([]string{"HS256"}),
		jwt.WithLeeway(30*time.Second),
		jwt.WithExpirationRequired(),
	)
	
	secret := append(s.secret, []byte(passwordHash)...)
	_, err := parser.ParseWithClaims(raw, &claims, func(t *jwt.Token) (any, error) {
		return secret, nil
	})
	if err != nil {
		return 0, err
	}
	
	if claims.Typ != typPasswordReset {
		return 0, fmt.Errorf("token is not a password reset token")
	}
	userID, err := strconv.ParseInt(claims.Subject, 10, 64)
	if err != nil || userID == 0 {
		return 0, fmt.Errorf("invalid token subject")
	}
	return userID, nil
}

// ExtractSubjectWithoutVerify extracts the subject from a JWT without verifying the signature.
func (s *Service) ExtractSubjectWithoutVerify(raw string) (int64, error) {
	parser := jwt.NewParser()
	var claims resetClaims
	_, _, err := parser.ParseUnverified(raw, &claims)
	if err != nil {
		return 0, err
	}
	userID, err := strconv.ParseInt(claims.Subject, 10, 64)
	if err != nil || userID == 0 {
		return 0, fmt.Errorf("invalid token subject")
	}
	return userID, nil
}
