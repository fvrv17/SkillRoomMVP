package security

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type AccessClaims struct {
	Subject        string `json:"sub"`
	Role           string `json:"role"`
	OrganizationID string `json:"org_id,omitempty"`
	ExpiresAt      int64  `json:"exp"`
	IssuedAt       int64  `json:"iat"`
	Issuer         string `json:"iss"`
}

type TokenManager struct {
	secret []byte
	issuer string
}

func NewTokenManager(secret, issuer string) *TokenManager {
	return &TokenManager{
		secret: []byte(secret),
		issuer: issuer,
	}
}

func (tm *TokenManager) MintAccessToken(subject, role, organizationID string, ttl time.Duration) (string, AccessClaims, error) {
	now := time.Now().UTC()
	claims := AccessClaims{
		Subject:        subject,
		Role:           role,
		OrganizationID: organizationID,
		ExpiresAt:      now.Add(ttl).Unix(),
		IssuedAt:       now.Unix(),
		Issuer:         tm.issuer,
	}

	headerJSON, err := json.Marshal(map[string]string{
		"alg": "HS256",
		"typ": "JWT",
	})
	if err != nil {
		return "", AccessClaims{}, err
	}

	payloadJSON, err := json.Marshal(claims)
	if err != nil {
		return "", AccessClaims{}, err
	}

	header := base64.RawURLEncoding.EncodeToString(headerJSON)
	payload := base64.RawURLEncoding.EncodeToString(payloadJSON)
	signingInput := header + "." + payload
	signature := tm.sign(signingInput)

	return signingInput + "." + signature, claims, nil
}

func (tm *TokenManager) ParseAccessToken(token string) (AccessClaims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return AccessClaims{}, errors.New("invalid token format")
	}

	signingInput := parts[0] + "." + parts[1]
	expected := tm.sign(signingInput)
	if !hmac.Equal([]byte(expected), []byte(parts[2])) {
		return AccessClaims{}, errors.New("invalid token signature")
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return AccessClaims{}, fmt.Errorf("decode payload: %w", err)
	}

	var claims AccessClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return AccessClaims{}, fmt.Errorf("decode claims: %w", err)
	}

	if claims.Issuer != tm.issuer {
		return AccessClaims{}, errors.New("invalid token issuer")
	}
	if time.Now().UTC().Unix() > claims.ExpiresAt {
		return AccessClaims{}, errors.New("token expired")
	}

	return claims, nil
}

func (tm *TokenManager) ClaimsFromRequest(r *http.Request) (AccessClaims, error) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return AccessClaims{}, errors.New("missing authorization header")
	}

	token := strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer"))
	if token == authHeader {
		return AccessClaims{}, errors.New("authorization header must use Bearer")
	}

	return tm.ParseAccessToken(token)
}

func (tm *TokenManager) sign(value string) string {
	mac := hmac.New(sha256.New, tm.secret)
	_, _ = mac.Write([]byte(value))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
