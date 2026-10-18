package config_test

import (
	"strings"
	"testing"

	"github.com/nyactl/todoist-cli/internal/config"
)

func TestMain(m *testing.M) {
	config.MockKeychain()
	m.Run()
}

func TestToken_SetGetDelete(t *testing.T) {
	t.Setenv("TODOIST_TOKEN", "")
	if err := config.SetToken("secret-a"); err != nil {
		t.Fatalf("SetToken: %v", err)
	}
	got, err := config.GetToken()
	if err != nil || got != "secret-a" {
		t.Fatalf("GetToken = %q, %v; want secret-a", got, err)
	}
	if err := config.DeleteToken(); err != nil {
		t.Fatalf("DeleteToken: %v", err)
	}
	if _, err := config.GetToken(); err == nil {
		t.Fatal("GetToken after delete: expected error")
	}
}

func TestGetToken_EnvOverridesKeychain(t *testing.T) {
	t.Setenv("TODOIST_TOKEN", "from-env")
	if err := config.SetToken("from-keychain"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { config.DeleteToken() })
	if got, _ := config.GetToken(); got != "from-env" {
		t.Errorf("GetToken = %q, want from-env", got)
	}
}

func TestGetToken_MissingExplainsHowToLogIn(t *testing.T) {
	t.Setenv("TODOIST_TOKEN", "")
	_, err := config.GetToken()
	if err == nil || !strings.Contains(err.Error(), "auth login") {
		t.Errorf("GetToken error = %v, want hint to run auth login", err)
	}
}

func TestDeleteToken_NothingStoredErrors(t *testing.T) {
	if err := config.DeleteToken(); err == nil {
		t.Error("DeleteToken with nothing stored: expected error")
	}
}
