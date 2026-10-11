package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	ErrInvalidToken = errors.New("jeton d'authentification invalide")
	ErrExpiredToken = errors.New("le jeton d'authentification a expiré")
)

// UserClaims représente la charge utile signée du jeton JWT
type UserClaims struct {
	UserID     string `json:"sub"`
	TenantID   string `json:"tid"`
	TenantName string `json:"tname"`
	Email      string `json:"email"`
	FullName   string `json:"name"`
	Role       string `json:"role"`
	ExpiresAt  int64  `json:"exp"`
}

// JWTManager gère la signature et la vérification des jetons
type JWTManager struct {
	secretKey     []byte
	tokenDuration time.Duration
}

// NewJWTManager initialise le gestionnaire JWT avec un secret sécurisé
func NewJWTManager(secret string, duration time.Duration) *JWTManager {
	if secret == "" {
		secret = "pressingos-default-dev-secret-key-change-in-production-12345"
	}
	if duration == 0 {
		duration = 24 * time.Hour
	}
	return &JWTManager{
		secretKey:     []byte(secret),
		tokenDuration: duration,
	}
}

// Generate crée un nouveau jeton signé HMAC-SHA256
func (m *JWTManager) Generate(user *User, tenantName string) (string, error) {
	headerJSON, err := json.Marshal(map[string]string{
		"alg": "HS256",
		"typ": "JWT",
	})
	if err != nil {
		return "", err
	}

	claims := UserClaims{
		UserID:     user.ID,
		TenantID:   user.TenantID,
		TenantName: tenantName,
		Email:      user.Email,
		FullName:   user.FullName,
		Role:       user.Role,
		ExpiresAt:  time.Now().Add(m.tokenDuration).Unix(),
	}

	payloadJSON, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}

	headerEnc := base64.RawURLEncoding.EncodeToString(headerJSON)
	payloadEnc := base64.RawURLEncoding.EncodeToString(payloadJSON)

	unsignedToken := fmt.Sprintf("%s.%s", headerEnc, payloadEnc)
	sig := m.sign(unsignedToken)
	sigEnc := base64.RawURLEncoding.EncodeToString(sig)

	return fmt.Sprintf("%s.%s", unsignedToken, sigEnc), nil
}

// Verify valide l'intégrité et l'expiration d'un jeton
func (m *JWTManager) Verify(tokenString string) (*UserClaims, error) {
	parts := strings.Split(tokenString, ".")
	if len(parts) != 3 {
		return nil, ErrInvalidToken
	}

	unsignedToken := fmt.Sprintf("%s.%s", parts[0], parts[1])
	expectedSig := m.sign(unsignedToken)

	actualSig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, ErrInvalidToken
	}

	if !hmac.Equal(expectedSig, actualSig) {
		return nil, ErrInvalidToken
	}

	payloadJSON, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, ErrInvalidToken
	}

	var claims UserClaims
	if err := json.Unmarshal(payloadJSON, &claims); err != nil {
		return nil, ErrInvalidToken
	}

	if time.Now().Unix() > claims.ExpiresAt {
		return nil, ErrExpiredToken
	}

	return &claims, nil
}

func (m *JWTManager) sign(data string) []byte {
	h := hmac.New(sha256.New, m.secretKey)
	h.Write([]byte(data))
	return h.Sum(nil)
}
