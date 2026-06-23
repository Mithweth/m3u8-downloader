package m3u8

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"github.com/Mithweth/m3u8-downloader/internal/domain"
	"github.com/Mithweth/m3u8-downloader/internal/utils/strtools"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func generateName(segmentUrl, prefix string, index int) (string, error) {
	u, err := url.Parse(segmentUrl)
	if err != nil {
		return "", err
	}
	filename := filepath.Base(u.Path)
	ext := filepath.Ext(filename)
	base := strings.TrimSuffix(filename, ext)
	if prefix != "" {
		base = prefix
	}

	filename = fmt.Sprintf("%s%05d%s", base, index, ext)
	if !strings.HasSuffix(filename, ".ts") {
		filename += ".ts"
	}
	return filename, nil
}

func extractByteRange(line string) (*domain.ByteRange, error) {
	var offset int64
	lengthStr, offsetStr, found := strings.Cut(strings.TrimPrefix(line, "#EXT-X-BYTERANGE:"), "@")
	length, err := strconv.ParseInt(lengthStr, 10, 64)
	if err != nil {
		return nil, err
	}
	if found {
		offset, err = strconv.ParseInt(offsetStr, 10, 64)
		if err != nil {
			return nil, err
		}
	}
	return &domain.ByteRange{Length: length, Offset: offset}, nil
}

func extractStreamInf(line string) *domain.VideoVariation {
	var v domain.VideoVariation
	attrs := strtools.RealSplit(strings.TrimPrefix(line, "#EXTINF:"), ',')
	for _, attr := range attrs {
		key, val, found := strings.Cut(attr, "=")
		if !found {
			continue
		}

		switch key {
		case "BANDWIDTH":
			v.Bandwidth, _ = strconv.Atoi(key)
		case "RESOLUTION":
			v.Resolution = val
		case "AVERAGE-BANDWIDTH":
			v.AverageBandwidth, _ = strconv.Atoi(key)
		case "CODECS":
			for _, codec := range strings.Split(val, ",") {
				v.Codecs = append(v.Codecs, strings.TrimPrefix(codec, " "))
			}
		default:
		}
	}
	return &v
}

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

func extractDuration(line string) (float64, error) {
	val := strings.TrimPrefix(line, "#EXTINF:")
	val = strings.TrimSpace(strings.TrimSuffix(val, ","))

	return strconv.ParseFloat(val, 64)
}

func DownloadPlaylist(m3u8url, prefix string, headers map[string]string, insecure bool) (*domain.Playlist, error) {
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

	p := domain.Playlist{URL: m3u8url}

	var (
		pendingVariation *domain.VideoVariation
		pendingByteRange *domain.ByteRange
		pendingDuration  *float64
		lastOffset       int64
	)

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
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
			pendingVariation = extractStreamInf(line)
		}
		if strings.HasPrefix(line, "#EXTINF") {
			d, err := extractDuration(line)
			if err != nil {
				return nil, err
			}
			pendingDuration = &d
		}

		if strings.HasPrefix(line, "#EXT-X-BYTERANGE") {
			br, err := extractByteRange(line)
			if err != nil {
				return nil, err
			}
			if br.Offset == 0 && lastOffset > 0 {
				br.Offset = lastOffset + br.Length
			}
			pendingByteRange = br
		}

		if strings.HasPrefix(line, "#") {
			continue
		}
		u, err := baseUrl.Parse(line)
		if err != nil {
			return nil, err
		}
		if pendingVariation != nil {
			pendingVariation.URL = u.String()
			p.VideoVariations = append(p.VideoVariations, *pendingVariation)
			pendingVariation = nil
		} else {
			name, err := generateName(u.String(), prefix, len(p.Segments))
			if err != nil {
				return nil, err
			}
			seg := domain.Segment{URL: u.String(), Name: name}
			if pendingDuration != nil {
				seg.Duration = *pendingDuration
				pendingDuration = nil
			}
			if pendingByteRange != nil {
				seg.Range = pendingByteRange
				lastOffset = pendingByteRange.Offset
				pendingByteRange = nil
			}
			p.Segments = append(p.Segments, seg)
		}
	}

	if p.Type == domain.PlaylistUnknown {
		if len(p.VideoVariations) > 0 {
			p.Type = domain.PlaylistMaster
		} else if len(p.Segments) > 0 {
			p.Type = domain.PlaylistTS
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return &p, nil
}
