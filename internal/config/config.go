package config

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/zalando/go-keyring"
)

const (
	keyringService = "todoist-cli.personal"
	keyringUser    = "token"
)

func SetToken(token string) error {
	return keyring.Set(keyringService, keyringUser, token)
}

func GetToken() (string, error) {
	if token := os.Getenv("TODOIST_TOKEN"); token != "" {
		return token, nil
	}
	token, err := keyring.Get(keyringService, keyringUser)
	if err != nil {
		return "", fmt.Errorf("no token found — run: todoist-cli auth login (or set TODOIST_TOKEN)")
	}
	return token, nil
}

func DeleteToken() error {
	// go-keyring's delete silently fails on macOS for unsigned binaries.
	// Use the security CLI directly, scoped to our exact service and account.
	err := exec.Command("security",
		"delete-generic-password",
		"-s", keyringService,
		"-a", keyringUser,
	).Run()
	if err != nil {
		// Fall back to go-keyring; also handles non-macOS platforms.
		return keyring.Delete(keyringService, keyringUser)
	}
	return nil
}
