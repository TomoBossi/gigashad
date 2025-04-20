package main

import (
	"fmt"
	"os"
)

func exists(path string) (bool, error) {
	_, err := os.Stat(path)
	return err == nil, err
}

func loadShaderSource(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read shader file: %w", err)
	}
	return string(data) + "\x00", nil
}
