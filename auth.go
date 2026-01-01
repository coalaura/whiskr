package main

import (
	"crypto/hmac"
	"crypto/sha3"
	"encoding/hex"
	"encoding/json"
	"hash"
	"net/http"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

type AuthenticationRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func NewHash() hash.Hash {
	return sha3.New512()
}

func (u *EnvUser) Signature(secret string) []byte {
	mac := hmac.New(NewHash, []byte(secret))

	mac.Write([]byte(u.Password))
	mac.Write([]byte(u.Username))

	return mac.Sum(nil)
}

func (e *Environment) Authenticate(username, password string) *EnvUser {
	user, ok := e.Authentication.lookup[username]
	if !ok {
		return nil
	}

	if bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)) != nil {
		return nil
	}

	return user
}

func (e *Environment) SignAuthToken(user *EnvUser) string {
	signature := user.Signature(e.Tokens.Secret)

	return user.Username + ":" + hex.EncodeToString(signature)
}

func (e *Environment) VerifyAuthToken(token string) bool {
	index := strings.Index(token, ":")
	if index == -1 {
		return false
	}

	username := token[:index]

	user, ok := e.Authentication.lookup[username]
	if !ok {
		return false
	}

	signature, err := hex.DecodeString(token[index+1:])
	if err != nil {
		return false
	}

	expected := user.Signature(e.Tokens.Secret)

	return hmac.Equal(signature, expected)
}

func IsAuthenticated(r *http.Request) bool {
	if !env.Authentication.Enabled {
		return true
	}

	cookie, err := r.Cookie("whiskr_token")
	if err != nil {
		return false
	}

	return env.VerifyAuthToken(cookie.Value)
}

func Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !IsAuthenticated(r) {
			RespondJson(w, http.StatusUnauthorized, map[string]any{
				"error": "unauthorized",
			})

			return
		}

		next.ServeHTTP(w, r)
	})
}

func HandleAuthentication(w http.ResponseWriter, r *http.Request) {
	var request AuthenticationRequest

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		RespondJson(w, http.StatusBadRequest, map[string]any{
			"error": "missing username or password",
		})

		return
	}

	user := env.Authenticate(request.Username, request.Password)
	if user == nil {
		RespondJson(w, http.StatusUnauthorized, map[string]any{
			"error": "invalid username or password",
		})

		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:  "whiskr_token",
		Value: env.SignAuthToken(user),
	})

	RespondJson(w, http.StatusOK, map[string]any{
		"authenticated": true,
	})
}
