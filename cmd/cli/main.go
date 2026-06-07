package main

import (
	"fmt"
	"github.com/Mithweth/m3u8-downloader/internal/config"
	"github.com/Mithweth/m3u8-downloader/internal/ffmpeg"
	"github.com/Mithweth/m3u8-downloader/internal/m3u8"
	"github.com/Mithweth/m3u8-downloader/internal/utils/ostools"
	"github.com/spf13/pflag"
	"os"
	"strings"
)

var (
	version = "dev"
	commit  = "none"
	date    = "never"
)

func parseArgs() (config.Config, error) {
	var (
		extraHeaders []string
		saveConfig   bool
		overwrite    bool
	)
	cfg := config.Config{}
	tomlCfg, err := config.LoadConfig()
	if err != nil {
		return config.Config{}, err
	}
	pflag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s <m3u8 url> <output-file>\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Options:\n")
		pflag.PrintDefaults()
	}
	pflag.StringP(
		"referer",
		"e",
		tomlCfg.Headers["Referer"],
		"HTTP Referer header",
	)

	pflag.StringP(
		"user-agent",
		"A",
		tomlCfg.Headers["User-Agent"],
		"HTTP User-Agent header",
	)

	pflag.StringArrayVarP(
		&extraHeaders,
		"header",
		"H",
		nil,
		"Extra HTTP header, e.g. 'Authorization: Basic xxx'",
	)

	pflag.StringVarP(
		&cfg.TemporaryDirectory,
		"working-directory",
		"w",
		"",
		"temporary directory where to download segments",
	)
	pflag.BoolVarP(
		&saveConfig,
		"save-config",
		"S",
		false,
		"Save config to file",
	)

	pflag.StringVarP(
		&tomlCfg.Paths.FFmpegBinary,
		"ffmpeg",
		"b",
		tomlCfg.Paths.FFmpegBinary,
		"Path to ffmpeg binary",
	)

	pflag.BoolVarP(
		&overwrite,
		"overwrite",
		"y",
		false,
		"Overwrite output file if exists",
	)

	pflag.IntVarP(
		&tomlCfg.Videos.MaxParallel,
		"max-parallel",
		"f",
		tomlCfg.Videos.MaxParallel,
		"Maximum number of parallel downloads",
	)

	pflag.StringVarP(
		&tomlCfg.Videos.PreferredFormat,
		"format",
		"F",
		tomlCfg.Videos.PreferredFormat,
		"Preferred video format",
	)

	pflag.Parse()

	if pflag.NArg() != 2 {
		pflag.Usage()
		os.Exit(1)
	}

	tomlCfg.Headers["Referer"], _ = pflag.CommandLine.GetString("referer")
	tomlCfg.Headers["User-Agent"], _ = pflag.CommandLine.GetString("user-agent")
	for _, h := range extraHeaders {
		name, value, ok := strings.Cut(h, ":")
		if !ok {
			return config.Config{}, fmt.Errorf("invalid header %q, expected 'Name: value'", h)
		}
		name = strings.TrimSpace(name)
		value = strings.TrimSpace(value)
		tomlCfg.Headers[name] = value
	}
	tomlCfg.Paths.FFmpegBinary, err = ostools.ExpandPath(tomlCfg.Paths.FFmpegBinary)
	if err != nil {
		return config.Config{}, err
	}
	if saveConfig {
		if err = config.SaveConfig(tomlCfg); err != nil {
			return config.Config{}, err
		}
	}
	tomlCfg.Paths.FFmpegBinary, err = ffmpeg.ValidateBinary(tomlCfg.Paths.FFmpegBinary)
	if err != nil {
		return config.Config{}, err
	}
	fmt.Printf("Using ffmpeg: %s\n", tomlCfg.Paths.FFmpegBinary)
	cfg.FileConfig = tomlCfg
	cfg.URL = pflag.Arg(0)
	cfg.OutputFile, err = ostools.ExpandPath(pflag.Arg(1))
	if err != nil {
		return config.Config{}, err
	}
	if !overwrite {
		if _, err := os.Stat(cfg.OutputFile); err == nil {
			return config.Config{}, fmt.Errorf("file already exists: %s", cfg.OutputFile)
		}
	}
	return cfg, nil
}

func Run() error {
	if len(os.Args) > 1 {
		if os.Args[1] == "version" {
			fmt.Printf("%s (%s, %s)\n", version, commit, date)
			os.Exit(0)
		}
	}
	cfg, err := parseArgs()
	if err != nil {
		return err
	}
	entries, err := m3u8.DownloadM3U8(cfg.URL, cfg.FileConfig.Headers)
	if err != nil {
		return err
	}
	if m3u8.IsPlaylist(entries) {
		hasDownloaded := false
		for _, entry := range entries {
			if m3u8.IsPreferredFormat(entry, cfg.FileConfig.Videos.PreferredFormat) {
				entries, err = m3u8.DownloadM3U8(entry, cfg.FileConfig.Headers)
				if err != nil {
					return err
				}
				hasDownloaded = true
				break
			}
		}
		if !hasDownloaded {
			return fmt.Errorf("valid formats are: %v", m3u8.GetFormats(entries))
		}
	}
	workDir, cleanup, err := ostools.GetWorkingDirectory(cfg.TemporaryDirectory)
	if err != nil {
		return err
	}
	defer cleanup()
	events := make(chan m3u8.DownloadEvent)

	go func() {
		for ev := range events {
			switch {
			case ev.AlreadyExists:
				fmt.Printf("[%d/%d] Downloading %s... SKIP\n", ev.Done, ev.Total, ev.URL)
			case ev.Success:
				fmt.Printf("[%d/%d] Downloading %s... OK\n", ev.Done, ev.Total, ev.URL)
			default:
				fmt.Printf("[%d/%d] Downloading %s... FAIL (%v)\n", ev.Done, ev.Total, ev.URL, ev.Error)
			}
		}
	}()

	files, err := m3u8.DownloadVideos(
		entries,
		workDir,
		cfg.FileConfig.Headers,
		cfg.FileConfig.Videos.MaxParallel,
		events,
	)
	close(events)

	if err != nil {
		return err
	}

	fileList, err := ffmpeg.PrepareFileList(workDir, files)
	if err != nil {
		return err
	}
	fmt.Printf("Creating %s... ", cfg.OutputFile)
	if errConvert := ffmpeg.Convert(cfg.FileConfig.Paths.FFmpegBinary, fileList, cfg.OutputFile); errConvert != nil {
		fmt.Printf("FAIL (%s)\n", errConvert)
		return err
	}
	fmt.Printf("OK\n")
	return nil
}

func main() {
	if err := Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	os.Exit(0)
}
