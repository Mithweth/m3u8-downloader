package m3u8

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"github.com/Mithweth/m3u8-downloader/internal/domain"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

func extractURI(line string) string {
	const key = `URI="`

	start := strings.Index(line, key)
	if start == -1 {
		return ""
	}

	start += len(key)
	end := strings.Index(line[start:], `"`)
	if end == -1 {
		return ""
	}

	return line[start : start+end]
}

func DownloadPlaylist(m3u8url string, headers map[string]string, insecure bool) (*domain.Playlist, error) {
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
		return nil, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP error %d", resp.StatusCode)
	}

	var p domain.Playlist

	var duration float64

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if strings.HasPrefix(line, "#EXT-X-MAP") {
			init, err := baseUrl.Parse(extractURI(line))
			if err != nil {
				return nil, err
			}
			p.Segments = append([]domain.Segment{{URL: init.String()}}, p.Segments...)
			p.Type = domain.PlaylistFMP4
			continue
		}
		if strings.HasPrefix(line, "#EXT-X-STREAM-INF") {
			p.Type = domain.PlaylistMaster
		}
		if strings.HasPrefix(line, "#EXTINF") {
			val := strings.TrimPrefix(line, "#EXTINF:")
			if strings.HasSuffix(val, ",") {
				val = strings.TrimSuffix(val, ",")
			}
			duration, err = strconv.ParseFloat(val, 64)
			if err != nil {
				return nil, err
			}
		}

		if strings.HasPrefix(line, "#") {
			continue
		}

		u, err := baseUrl.Parse(line)
		if err != nil {
			return nil, err
		}
		p.Segments = append(p.Segments, domain.Segment{URL: u.String(), Duration: duration})
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return &p, nil
}
