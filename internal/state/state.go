package state

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type State struct {
	ProjectID   string `json:"project_id"`
	ProjectName string `json:"project_name"`
}

func (s *State) HasProject() bool {
	return s != nil && s.ProjectID != ""
}

func Load() (*State, error) {
	path, err := statePath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return &State{}, nil
	}
	if err != nil {
		return nil, err
	}
	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		return &State{}, nil
	}
	return &s, nil
}

func Save(s *State) error {
	path, err := statePath()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

func Clear() error {
	path, err := statePath()
	if err != nil {
		return err
	}
	return os.Remove(path)
}

func statePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".todoist-cli")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	return filepath.Join(dir, "state.json"), nil
}
