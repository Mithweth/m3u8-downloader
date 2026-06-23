package ffmpeg

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
)

func ValidateBinary(path string) (string, error) {
	var err error

	if path == "" {
		path, err = exec.LookPath("ffmpeg")
		if err != nil {
			return "", fmt.Errorf("ffmpeg not found: please install it or specify with --ffmpeg")
		}
	}

	path, err = filepath.Abs(path)
	if err != nil {
		return "", err
	}

	if _, err := exec.Command(path, "-version").CombinedOutput(); err != nil {
		if pathErr, ok := err.(*os.PathError); ok {
			return "", fmt.Errorf("%s is not a valid ffmpeg executable: %v", path, pathErr.Err)
		}
		return "", err
	}

	return path, nil
}

func PrepareFileList(path string, files []string) (string, error) {
	concatFile := filepath.Join(path, "concat.txt")
	fd, err := os.Create(concatFile)
	if err != nil {
		return "", err
	}
	defer func() {
		if cerr := fd.Close(); err == nil && cerr != nil {
			err = cerr
		}
	}()
	for _, e := range files {
		if filepath.Dir(e) == path {
			if _, err := fmt.Fprintf(fd, "file '%s'\n", filepath.Base(e)); err != nil {
				return "", err
			}
		}
	}
	return concatFile, nil
}

func ConcatFiles(path string, files []string) (string, error) {
	concatFile := filepath.Join(path, "temporary.av1")
	out, err := os.Create(concatFile)
	if err != nil {
		return "", err
	}
	defer out.Close()

	for _, file := range files {
		in, err := os.Open(file)
		if err != nil {
			return "", fmt.Errorf("open %s: %w", file, err)
		}

		_, copyErr := io.Copy(out, in)
		closeErr := in.Close()

		if copyErr != nil {
			return "", fmt.Errorf("copy %s: %w", file, copyErr)
		}
		if closeErr != nil {
			return "", fmt.Errorf("close %s: %w", file, closeErr)
		}
	}

	return concatFile, nil
}
