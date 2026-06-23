package ffmpeg

import (
	"bufio"
	"fmt"
	"github.com/Mithweth/m3u8-downloader/internal/domain"
	"os/exec"
	"strconv"
	"strings"
)

type Ffmpeg struct {
	Path string
}

func New(path string) Ffmpeg {
	return Ffmpeg{Path: path}
}

func (f *Ffmpeg) Convert(p *domain.Playlist, input, output string, events chan<- domain.FFmpegEvent) error {
	var cmd *exec.Cmd
	var percent float64
	errCh := make(chan error, 1)
	duration := GetDuration(p)
	switch p.Type {
	case domain.PlaylistTS:
		cmd = exec.Command(
			f.Path,
			"-v", "error",
			"-f", "concat",
			"-y",
			"-i", input,
			"-c", "copy",
			"-bsf:a", "aac_adtstoasc",
			"-progress", "pipe:1",
			output,
		)
	case domain.PlaylistFMP4:
		cmd = exec.Command(
			f.Path,
			"-v", "error",
			"-y",
			"-i", input,
			"-c:v", "libx264",
			"-c:a", "copy",
			"-progress", "pipe:1",
			output,
		)
	default:
		return fmt.Errorf("Playlist type not supported")
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		return err
	}
	go func() {
		defer close(errCh)
		scanner := bufio.NewScanner(stdout)

		for scanner.Scan() {
			line := scanner.Text()

			if strings.HasPrefix(line, "out_time_ms=") {
				elapsedMs, err := strconv.ParseFloat(strings.TrimPrefix(line, "out_time_ms="), 64)
				if err != nil {
					errCh <- err
				}
				percent = elapsedMs / 1000000.0 / duration
				if percent > 1 {
					percent = 1
				}
				events <- domain.FFmpegEvent{Percent: percent}
			}
		}
	}()

	err = cmd.Wait()
	err = <-errCh
	if err != nil {
		return fmt.Errorf("ffmpeg failed: %w", err)
	}

	return nil
}

func GetDuration(p *domain.Playlist) float64 {
	var retval float64
	for _, e := range p.Segments {
		retval += e.Duration
	}
	return retval
}
