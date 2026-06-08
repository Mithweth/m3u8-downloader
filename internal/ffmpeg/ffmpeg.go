package ffmpeg

import (
	"fmt"
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

func Convert(ffmpegBin, fileList, output string) error {
	if _, err := os.Stat(ffmpegBin); err != nil {
		return err
	}
	cmd := exec.Command(
		ffmpegBin,
		"-v", "error",
		"-f", "concat",
		"-y",
		"-i", fileList,
		"-c", "copy",
		"-bsf:a", "aac_adtstoasc",
		output,
	)

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ffmpeg failed: %w: %s", err, string(out))
	}

	return nil
}
