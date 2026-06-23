package main

import (
	"fmt"
	"github.com/Mithweth/m3u8-downloader/internal/config"
	"github.com/Mithweth/m3u8-downloader/internal/domain"
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
		tomlCfg.HTTP.Headers["Referer"],
		"HTTP Referer header",
	)

	pflag.StringP(
		"user-agent",
		"A",
		tomlCfg.HTTP.Headers["User-Agent"],
		"HTTP User-Agent header",
	)

	pflag.BoolVarP(
		&tomlCfg.HTTP.Insecure,
		"insecure",
		"k",
		tomlCfg.HTTP.Insecure,
		"Skip SSL verification and proceed without checking",
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

	tomlCfg.HTTP.Headers["Referer"], _ = pflag.CommandLine.GetString("referer")
	tomlCfg.HTTP.Headers["User-Agent"], _ = pflag.CommandLine.GetString("user-agent")
	for _, h := range extraHeaders {
		name, value, ok := strings.Cut(h, ":")
		if !ok {
			return config.Config{}, fmt.Errorf("invalid header %q, expected 'Name: value'", h)
		}
		name = strings.TrimSpace(name)
		value = strings.TrimSpace(value)
		tomlCfg.HTTP.Headers[name] = value
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
	if !strings.HasSuffix(pflag.Arg(1), ".mp4") {
		return config.Config{}, fmt.Errorf("output file needs a .mp4 extension: %s", pflag.Arg(1))
	}
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
	playlist, err := m3u8.DownloadPlaylist(cfg.URL, cfg.FileConfig.HTTP.Headers, cfg.FileConfig.HTTP.Insecure)
	if err != nil {
		return err
	}
	if playlist.Type == domain.PlaylistMaster {
		hasDownloaded := false
		for _, entry := range playlist.VideoVariations {
			if entry.Resolution == cfg.FileConfig.Videos.PreferredFormat {
				playlist, err = m3u8.DownloadPlaylist(entry.URL, cfg.FileConfig.HTTP.Headers, cfg.FileConfig.HTTP.Insecure)
				if err != nil {
					return err
				}
				hasDownloaded = true
				break
			}
		}
		if !hasDownloaded {
			return fmt.Errorf("valid formats are: %v", m3u8.GetAvailableFormats(playlist.VideoVariations))
		}
	}
	workDir, cleanup, err := ostools.GetWorkingDirectory(cfg.TemporaryDirectory)
	if err != nil {
		return err
	}
	defer cleanup()
	fmt.Println("temporary directory:", workDir)
	events := make(chan domain.DownloadEvent)

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
		playlist,
		workDir,
		cfg.FileConfig.HTTP.Headers,
		cfg.FileConfig.Videos.MaxParallel,
		cfg.FileConfig.HTTP.Insecure,
		events,
	)
	close(events)
	if err != nil {
		return err
	}

	fmt.Println("duration", ffmpeg.GetDuration(playlist))
	var inputFile string
	switch playlist.Type {
	case domain.PlaylistTS:
		inputFile, err = ffmpeg.PrepareFileList(workDir, files)
		if err != nil {
			return err
		}
	case domain.PlaylistFMP4:
		inputFile, err = ffmpeg.ConcatFiles(workDir, files)
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("playlist type not supported")
	}
	ffm := ffmpeg.New(cfg.FileConfig.Paths.FFmpegBinary)
	ffmevents := make(chan domain.FFmpegEvent)
	defer close(ffmevents)
	go func() {
		for ev := range ffmevents {
			fmt.Printf("Creating %s... %.1f%%\n", cfg.OutputFile, ev.Percent*100)
		}
	}()
	if errConvert := ffm.Convert(playlist, inputFile, cfg.OutputFile, ffmevents); errConvert != nil {
		return errConvert
	}
	fmt.Printf("OK!\n")
	return nil
}

func main() {
	if err := Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	os.Exit(0)
}
