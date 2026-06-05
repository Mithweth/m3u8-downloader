package ostools

import (
	"os"
	"path/filepath"
	"strings"
)

func GetWorkingDirectory(tempDirectory string) (string, func(), error) {
	if tempDirectory != "" {
		if err := os.MkdirAll(tempDirectory, 0755); err != nil {
			return "", nil, err
		}

		return tempDirectory, func() {}, nil
	}

	dir, err := os.MkdirTemp("", "m3u8-downloader-*")
	if err != nil {
		return "", nil, err
	}

	return dir, func() {
		_ = os.RemoveAll(dir)
	}, nil
}

func ExpandPath(path string) (string, error) {
	if path == "" {
		return "", nil
	}
	if strings.HasPrefix(path, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}

		if path == "~" {
			path = home
		} else {
			path = filepath.Join(home, path[2:])
		}
	}
	return filepath.Abs(path)
}
