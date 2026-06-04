package m3u8

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var httpClient = &http.Client{
	Timeout: 60 * time.Second,
}

func IsPlaylist(a []string) bool {
	for _, s := range a {
		if strings.HasSuffix(s, "m3u8") || strings.HasSuffix(s, "m3u") {
			return true
		}
	}
	return false
}

func GetFormats(a []string) []string {
	var retval []string
	for _, s := range a {
		p := strings.Split(s, "/")
		if len(p) >= 2 {
			retval = append(retval, p[len(p)-2])
		}
	}
	return retval
}

func IsPreferredFormat(s, f string) bool {
	p := strings.Split(s, "/")
	if len(p) < 2 {
		return false
	}
	return p[len(p)-2] == f
}

func DownloadM3U8(m3u8url string, headers map[string]string) ([]string, error) {
	baseUrl, err := url.Parse(m3u8url)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest("GET", m3u8url, nil)
	if err != nil {
		return nil, err
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP error %d", resp.StatusCode)
	}

	var retval []string

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		u, err := baseUrl.Parse(line)
		if err != nil {
			return nil, err
		}
		retval = append(retval, u.String())
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return retval, nil
}

func DownloadVideo(videoUrl, path string, index int, headers map[string]string) (bool, string, error) {
	u, err := url.Parse(videoUrl)
	if err != nil {
		return false, "", err
	}

	filename := filepath.Base(u.Path)
	filename = strings.Split(filename, ".")[0] + fmt.Sprintf("%04d", index)
	if !strings.HasSuffix(filename, ".ts") {
		filename += ".ts"
	}

	dest := filepath.Join(path, filename)

	if _, err := os.Stat(dest); err == nil {
		return true, dest, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return false, "", err
	}

	req, err := http.NewRequest("GET", videoUrl, nil)
	if err != nil {
		return false, "", err
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return false, "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return false, "", fmt.Errorf("HTTP error %d: %s", resp.StatusCode, videoUrl)
	}

	f, err := os.Create(dest)
	if err != nil {
		return false, "", err
	}
	defer f.Close()

	_, err = io.Copy(f, resp.Body)
	if err != nil {
		return false, "", err
	}

	return false, dest, nil
}

func DownloadVideos(entries []string, path string, headers map[string]string, maxParallel int) ([]string, error) {
	if maxParallel <= 0 {
		maxParallel = 1
	}

	results := make([]string, len(entries))
	errCh := make(chan error, len(entries))
	sem := make(chan struct{}, maxParallel)

	var wg sync.WaitGroup

	for i, entry := range entries {
		wg.Add(1)

		go func() {
			defer wg.Done()

			sem <- struct{}{}
			defer func() { <-sem }()

			exists, dest, err := DownloadVideo(entry, path, i+1, headers)
			if err != nil {
				fmt.Printf("[%d/%d] Downloading %s... FAIL (%v)\n", i+1, len(entries), filepath.Base(entry), err)
				errCh <- fmt.Errorf("download %s: %w", entry, err)
				return
			}

			results[i] = dest
			if !exists {
				fmt.Printf("[%d/%d] Downloading %s... OK (%s)\n", i+1, len(entries), filepath.Base(entry), filepath.Base(dest))
			} else {
				fmt.Printf("[%d/%d] Downloading %s... SKIP (%s)\n", i+1, len(entries), filepath.Base(entry), filepath.Base(dest))
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
