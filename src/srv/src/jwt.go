package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/dgrijalva/jwt-go"
	"golang.org/x/crypto/bcrypt"
)

// JwtData store a single JWT configuration
type JwtData struct {
	Enabled   bool   `json:"enabled"`   // Enable or disable JWT authentication
	Key       []byte `json:"key"`       // JWT signing key
	Exp       int    `json:"exp"`       // JWT expiration time in minutes
	RenewTime int    `json:"renewTime"` // Time in second before the JWT expiration time when the renewal is allowed
}

// Credentials holds the user name and password from the request body
type Credentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// Claims holds the JWT information to be encoded
type Claims struct {
	Username string `json:"username"`
	jwt.StandardClaims
}

// sendTokenResponse sends the signed JWT token if claims are valid
func sendTokenResponse(rw http.ResponseWriter, hr *http.Request, claims *Claims) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := token.SignedString(appParams.jwt.Key)
	if err != nil {
		sendResponse(rw, hr, http.StatusInternalServerError, "unable to sign the JWT token")
		return
	}
	sendResponse(rw, hr, http.StatusOK, signedToken)
}

// loginHandler handles the /auth/login
func loginHandler(rw http.ResponseWriter, hr *http.Request) {
	var creds Credentials
	err := json.NewDecoder(hr.Body).Decode(&creds)
	if err != nil {
		sendResponse(rw, hr, http.StatusBadRequest, err.Error())
		return
	}
	hash, ok := appParams.user[creds.Username]
	if !ok {
		// invalid user
		sendResponse(rw, hr, http.StatusUnauthorized, "invalid authentication credentials")
		return
	}
	err = bcrypt.CompareHashAndPassword([]byte(hash), []byte(creds.Password))
	if err != nil {
		// invalid password
		sendResponse(rw, hr, http.StatusUnauthorized, "invalid authentication credentials")
		return
	}
	exp := time.Now().Add(time.Duration(appParams.jwt.Exp) * time.Minute)
	claims := Claims{
		Username: creds.Username,
		StandardClaims: jwt.StandardClaims{
			ExpiresAt: exp.Unix(),
		},
	}
	sendTokenResponse(rw, hr, &claims)
}

// renewJwtHandler handles the /auth/refresh
func renewJwtHandler(rw http.ResponseWriter, hr *http.Request) {
	claims, err := checkJwtToken(rw, hr)
	if err != nil {
		sendResponse(rw, hr, http.StatusUnauthorized, err.Error())
		return
	}
	if time.Until(time.Unix(claims.ExpiresAt, 0)) > time.Duration(appParams.jwt.RenewTime)*time.Second {
		sendResponse(rw, hr, http.StatusBadRequest, "the JWT token can be renewed only when it is close to expiration")
		return
	}
	sendTokenResponse(rw, hr, claims)
}

// checkJwtToken extract the JWT token from the header "Authorization: Bearer <TOKEN>"
// and returns an error if the token is invalid.
func checkJwtToken(rw http.ResponseWriter, hr *http.Request) (*Claims, error) {
	headAuth := hr.Header.Get("Authorization")
	if len(headAuth) == 0 {
		return nil, errors.New("missing Authorization header")
	}
	authSplit := strings.Split(headAuth, "Bearer ")
	if len(authSplit) != 2 {
		return nil, errors.New("missing JWT token")
	}
	signedToken := authSplit[1]
	claims := Claims{}
	_, err := jwt.ParseWithClaims(signedToken, &claims, func(token *jwt.Token) (interface{}, error) {
		return appParams.jwt.Key, nil
	})
	return &claims, err
}

// isAuthorized checks if the user is authorized via JWT token
func isAuthorized(rw http.ResponseWriter, hr *http.Request) bool {
	if appParams.jwt.Enabled {
		_, err := checkJwtToken(rw, hr)
		if err != nil {
			sendResponse(rw, hr, http.StatusUnauthorized, err.Error())
			return false
		}
	}
	return true
}
