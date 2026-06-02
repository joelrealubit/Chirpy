package auth

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/alexedwards/argon2id"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

func HashPassword(password string) (string, error) {
	hash, err := argon2id.CreateHash(password, argon2id.DefaultParams)
	return hash, err
}

func CheckPasswordHash(password, hash string) (bool, error) {
	match, err := argon2id.ComparePasswordAndHash(password, hash)
	return match, err
}

func MakeJWT(userID uuid.UUID, tokenSecret string, expiresIn time.Duration) (string, error) {

	claims := jwt.RegisteredClaims{}
	claims.Issuer = "chirpy-access"
	now := time.Now().UTC().Local()
	claims.IssuedAt = jwt.NewNumericDate(now)
	claims.ExpiresAt = jwt.NewNumericDate(now.Add(expiresIn))
	claims.Subject = userID.String()

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	return token.SignedString([]byte(tokenSecret))

}

func ValidateJWT(tokenString, tokenSecret string) (uuid.UUID, error) {
	err_id, err := uuid.Parse("000")

	claims := jwt.RegisteredClaims{}
	token, err := jwt.ParseWithClaims(tokenString, &claims, func(token *jwt.Token) (interface{}, error) {
		return []byte(tokenSecret), nil
	}, jwt.WithLeeway(5*time.Second))
	if err != nil {
		log.Printf("FAIL: %s", err)
		return err_id, err
		//log.Fatal(err)
	} else if claims, ok := token.Claims.(*jwt.RegisteredClaims); ok {
		fmt.Println(claims.Issuer)
	} else {
		log.Print("unknown claims type, cannot proceed")
		return err_id, errors.New("unknown claims type, cannot proceed")

	}

	subj, err := token.Claims.GetSubject()
	if err != nil {
		log.Printf("error ValidateJWT ParseWithClaims claims.getSubject error = %s", err)
		return err_id, err
	}
	id, err := uuid.Parse(subj)
	log.Printf("ok uuid = %s", id)
	return id, err

}

func GetBearerToken(headers http.Header) (string, error) {

	//get authorization header

	raw_authHeader := headers.Get("Authorization")

	if raw_authHeader == "" {

		return raw_authHeader, errors.New("no authorization header")

	}

	authHeader := strings.TrimSpace(strings.TrimPrefix(raw_authHeader, "Bearer"))

	return authHeader, nil
}

func MakeRefreshToken() string {
	key := make([]byte, 32)
	rand.Read(key)
	return hex.EncodeToString(key)
}
