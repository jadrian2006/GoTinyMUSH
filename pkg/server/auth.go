package server

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/crystal-mush/gotinymush/pkg/gamedb"
	"github.com/golang-jwt/jwt/v5"
)

// Claims holds the JWT claims for an authenticated player session.
type Claims struct {
	PlayerRef  gamedb.DBRef `json:"player_ref"`
	PlayerName string       `json:"player_name"`
	jwt.RegisteredClaims
}

// AuthService provides JWT-based authentication bound to player identity.
type AuthService struct {
	game   *Game
	jwtKey []byte
	expiry time.Duration
}

// NewAuthService creates an auth service. If jwtSecret is empty, a random
// 32-byte key is generated.
func NewAuthService(game *Game, jwtSecret string, expirySeconds int) *AuthService {
	var key []byte
	if jwtSecret != "" {
		key = []byte(jwtSecret)
	} else {
		key = make([]byte, 32)
		rand.Read(key)
	}
	expiry := 24 * time.Hour
	if expirySeconds > 0 {
		expiry = time.Duration(expirySeconds) * time.Second
	}
	return &AuthService{
		game:   game,
		jwtKey: key,
		expiry: expiry,
	}
}

// Login authenticates a player and returns a JWT token.
func (a *AuthService) Login(name, password string) (string, error) {
	player := LookupPlayer(a.game.DB, name)
	if player == gamedb.Nothing {
		return "", fmt.Errorf("invalid credentials")
	}
	if !CheckPassword(a.game.DB, player, password) {
		return "", fmt.Errorf("invalid credentials")
	}

	playerName := a.game.PlayerName(player)
	now := time.Now()
	claims := Claims{
		PlayerRef:  player,
		PlayerName: playerName,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   fmt.Sprintf("#%d", player),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(a.expiry)),
			Issuer:    "gotinymush",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(a.jwtKey)
}

// ValidateToken parses and validates a JWT token string.
func (a *AuthService) ValidateToken(tokenStr string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return a.jwtKey, nil
	})
	if err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}
	return claims, nil
}

// RefreshToken creates a new token with a fresh expiry for an existing valid token.
func (a *AuthService) RefreshToken(tokenStr string) (string, error) {
	claims, err := a.ValidateToken(tokenStr)
	if err != nil {
		return "", err
	}

	now := time.Now()
	claims.IssuedAt = jwt.NewNumericDate(now)
	claims.ExpiresAt = jwt.NewNumericDate(now.Add(a.expiry))

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(a.jwtKey)
}

// GenerateJWTSecret generates a random hex-encoded secret suitable for jwt_secret config.
func GenerateJWTSecret() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}
