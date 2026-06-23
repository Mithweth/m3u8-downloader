package m3u8

// TODO:
// Parse EXT-X-BYTERANGE and download byte ranges from shared TS files.
// Currently only supports playlists with distinct segment files.

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"github.com/Mithweth/m3u8-downloader/internal/domain"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

func GetAvailableFormats(a []domain.VideoVariation) []string {
	var retval []string
	for _, s := range a {
		if s.Resolution != "" {
			retval = append(retval, s.Resolution)
		}
	}
	return retval
}

func DownloadVideo(ctx context.Context, videoUrl, path string, index int, headers map[string]string, insecure bool) (bool, string, error) {
	u, err := url.Parse(videoUrl)
	if err != nil {
		return false, "", err
	}

	filename := filepath.Base(u.Path)
	ext := filepath.Ext(filename)
	base := strings.TrimSuffix(filename, ext)

	filename = fmt.Sprintf("%s%04d%s", base, index, ext)
	if !strings.HasSuffix(filename, ".ts") {
		filename += ".ts"
	}

	dest := filepath.Join(path, filename)

	if _, err := os.Stat(dest); err == nil {
		return true, dest, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return false, "", err
	}

	req, err := http.NewRequestWithContext(ctx, "GET", videoUrl, nil)
	if err != nil {
		return false, "", err
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}
	httpClient := &http.Client{
		Timeout: 60 * time.Second,
	}
	if insecure {
		httpClient.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		}
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return false, "", err
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return false, "", fmt.Errorf("HTTP error %d: %s", resp.StatusCode, videoUrl)
	}

	f, err := os.Create(dest)
	if err != nil {
		return false, "", err
	}
	defer func() {
		_ = f.Close()
	}()
	_, err = io.Copy(f, resp.Body)
	if err != nil {
		return false, "", err
	}

	return false, dest, nil
}

func DownloadVideos(
	playlist *domain.Playlist,
	path string,
	headers map[string]string,
	maxParallel int,
	insecure bool,
	events chan<- domain.DownloadEvent,
) ([]string, error) {
	if maxParallel <= 0 {
		maxParallel = 1
	}
	total := len(playlist.Segments)
	done := 0
	results := make([]string, len(playlist.Segments))
	errCh := make(chan error, len(playlist.Segments))
	sem := make(chan struct{}, maxParallel)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var mu sync.Mutex
	var wg sync.WaitGroup

	for i, entry := range playlist.Segments {
		wg.Add(1)

		go func() {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				return
			}

			exists, dest, err := DownloadVideo(ctx, entry.URL, path, i+1, headers, insecure)
			results[i] = dest
			mu.Lock()
			done++
			currentDone := done
			mu.Unlock()
			if err != nil {
				cancel()
				errCh <- fmt.Errorf("download %s: %w", entry.URL, err)
				events <- domain.DownloadEvent{
					Done:     currentDone,
					Total:    total,
					URL:      entry.URL,
					FilePath: dest,
					Success:  false,
					Error:    err,
				}
				return
			}
			events <- domain.DownloadEvent{
				Done:          currentDone,
				Total:         total,
				AlreadyExists: exists,
				URL:           entry.URL,
				FilePath:      dest,
				Success:       err == nil,
				Error:         err,
			}
		}()
	}

	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			return nil, err
		}
	}

	return results, nil
}
