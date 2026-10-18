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

// securityDelete removes the token with the macOS security CLI, because
// go-keyring's delete silently fails on macOS for unsigned binaries.
var securityDelete = func() error {
	return exec.Command("security",
		"delete-generic-password",
		"-s", keyringService,
		"-a", keyringUser,
	).Run()
}

func DeleteToken() error {
	if err := securityDelete(); err != nil {
		// Fall back to go-keyring; also handles non-macOS platforms.
		return keyring.Delete(keyringService, keyringUser)
	}
	return nil
}

// MockKeychain replaces the system keychain with an empty in-memory store for
// the rest of the process. Tests call it so they can never read, overwrite or
// delete the real token.
func MockKeychain() {
	keyring.MockInit()
	securityDelete = func() error { return keyring.ErrNotFound }
}
