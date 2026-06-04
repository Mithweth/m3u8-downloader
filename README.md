# m3u8-downloader

Download video files (mostly TS) from M3U8 playlist and convert it to MP4 with ffmpeg

## Build

```
make
```

## Installation

```
make install
```

## Config file

### Location

* GNU/Linux: `~/.config/m3u8-downloader/config.toml`
* MacOS: `/Library/ApplicationSupport/m3u8-downloader/config.toml`

### Example

```
[videos]
  max_parallel = 3             # number of videos the app is allowed to download in parallel
  preferred_format = "url_2"   # if the M3U8 file contains several playlist, which one is preferred by default

[paths]
  ffmpeg_binary = "/usr/bin/ffmpeg"  # FFMpeg binary path

[headers]                            # HTTP headers sent to download the playlist and videos
  Referer = ""
  User-Agent = ""

```

## Command line

```
Usage: m3u8-downloader <url> <output-file>

Options:
  -b, --ffmpeg string              Path to ffmpeg binary
  -F, --format string              Preferred video format
  -H, --header stringArray         Extra HTTP header, e.g. 'Authorization: Basic xxx'
  -f, --max-parallel int           Maximum number of parallel downloads (default 3)
  -e, --referer string             HTTP Referer header
  -S, --save-config                Save config to file
  -A, --user-agent string          HTTP User-Agent header
  -w, --working-directory string   temporary directory where to download segments
```

Note that, when a playlist with several format is found and there is neither `--format` argument in cmdline nor `preferred_format` parameter in config file, the application will fail with a message telling to choose among the available video formats.
