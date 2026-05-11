package config

import (
	"fmt"

	"github.com/zalando/go-keyring"
)

const (
	keyringService = "todoist-cli"
	keyringUser    = "token"
)

func SetToken(token string) error {
	return keyring.Set(keyringService, keyringUser, token)
}

func GetToken() (string, error) {
	token, err := keyring.Get(keyringService, keyringUser)
	if err != nil {
		return "", fmt.Errorf("no token found — run: todoist-cli auth login")
	}
	return token, nil
}

func DeleteToken() error {
	return keyring.Delete(keyringService, keyringUser)
}
