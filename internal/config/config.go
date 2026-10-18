package config

import (
	"os"
	"path/filepath"
)

var (
	GroqBaseURL       string = "https://api.groq.com/openai/v1"
	OpenrouterBaseURL string = "https://openrouter.ai/api/v1"
	OllamaBaseURL     string = "http://localhost:11434/v1"
	GeminiBaseURL     string = "https://generativelanguage.googleapis.com/v1beta"
)

var kairoPath string

func Init() error {
	home, err := os.UserHomeDir()

	if err != nil {
		return err
	}

	kairoPath = filepath.Join(home, ".kairo")

	err = os.MkdirAll(kairoPath, 0755)

	if err != nil {
		return err
	}

	return nil
}

// Get kairoPath
func GetKairoPath() string {
	return kairoPath
}
