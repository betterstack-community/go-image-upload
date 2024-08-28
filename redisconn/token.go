package redisconn

import (
	"context"
	"crypto/rand"
	"encoding/base64"
)

func generateRandomBytes(n int) ([]byte, error) {
	b := make([]byte, n)

	_, err := rand.Read(b)
	if err != nil {
		return nil, err
	}

	return b, nil
}

func generateRandomStringURLSafe(n int) (string, error) {
	b, err := generateRandomBytes(n)
	if err != nil {
		return "", err
	}

	return base64.URLEncoding.EncodeToString(b), nil
}

func generateCryptoToken(_ context.Context) (string, error) {
	tokenLength := 32

	token, err := generateRandomStringURLSafe(tokenLength)
	if err != nil {
		return "", err
	}

	return token, nil
}
