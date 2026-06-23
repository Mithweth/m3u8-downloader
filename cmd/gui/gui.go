package main

import (
	"fmt"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	"github.com/Mithweth/m3u8-downloader/internal/config"
	"github.com/Mithweth/m3u8-downloader/internal/domain"
	"github.com/Mithweth/m3u8-downloader/internal/ffmpeg"
	"github.com/Mithweth/m3u8-downloader/internal/m3u8"
	"github.com/Mithweth/m3u8-downloader/internal/utils/ostools"
	"path/filepath"
	"slices"
)

type GUI struct {
	window           fyne.Window
	cfg              config.TomlConfig
	m3uUrl           *widget.Entry
	outputFile       *widget.Entry
	ffmpegPath       *widget.Entry
	formatSelect     *widget.Select
	progressBar      *widget.ProgressBar
	currentFileLabel *widget.Label
	processButton    *widget.Button
}

func (g *GUI) consumeEvents(events <-chan domain.DownloadEvent) {
	for ev := range events {
		fyne.Do(func() {
			g.progressBar.SetValue(float64(ev.Done) / float64(ev.Total))
			g.currentFileLabel.SetText(
				fmt.Sprintf("Downloading %s", filepath.Base(ev.FilePath)),
			)

			if !ev.Success {
				dialog.ShowError(ev.Error, g.window)
			}
		})
	}
}

func (g *GUI) showError(err error) {
	fyne.Do(func() {
		dialog.ShowError(err, g.window)
		g.processButton.Enable()
	})
}

func (g *GUI) resetForm() {
	g.outputFile.SetText("")
	g.m3uUrl.SetText("")
	g.formatSelect.OnChanged = nil
	g.formatSelect.ClearSelected()
	g.formatSelect.Options = nil
	g.formatSelect.Disable()
	g.processButton.Disable()
	g.progressBar.SetValue(0)
}

func (g *GUI) Process() {
	fyne.Do(func() {
		g.processButton.Disable()
		g.progressBar.SetValue(0)
		g.currentFileLabel.SetText("Preparing...")
	})

	playlist, err := m3u8.DownloadPlaylist(g.m3uUrl.Text, g.cfg.HTTP.Headers, g.cfg.HTTP.Insecure)
	if err != nil {
		g.showError(err)
		return
	}
	if playlist.Type == domain.PlaylistMaster {
		if g.formatSelect.Selected == "" {
			formats := m3u8.GetFormats(playlist.Segments)

			fyne.Do(func() {
				g.formatSelect.Options = formats
				g.formatSelect.Enable()

				if slices.Contains(formats, g.cfg.Videos.PreferredFormat) {
					g.formatSelect.SetSelected(g.cfg.Videos.PreferredFormat)
					g.processButton.Enable()
				}
			})
			return
		}

		for _, entry := range playlist.Segments {
			if m3u8.IsPreferredFormat(entry.URL, g.formatSelect.Selected) {
				playlist, err = m3u8.DownloadPlaylist(entry.URL, g.cfg.HTTP.Headers, g.cfg.HTTP.Insecure)
				if err != nil {
					g.showError(err)
					return
				}
				break
			}
		}
	}

	absFfmpegPath, err := ostools.ExpandPath(g.ffmpegPath.Text)
	if err != nil {
		g.showError(err)
		return
	}

	absFfmpegPath, err = ffmpeg.ValidateBinary(absFfmpegPath)
	if err != nil {
		g.showError(err)
		return
	}

	fmt.Println("ffmpeg binary:", absFfmpegPath)
	absOutputFile, err := ostools.ExpandPath(g.outputFile.Text)
	if err != nil {
		g.showError(err)
		return
	}

	fmt.Println("output file:", absOutputFile)

	workDir, cleanup, err := ostools.GetWorkingDirectory("")
	if err != nil {
		g.showError(err)
		return
	}
	defer cleanup()
	fmt.Println("temporary directory:", workDir)

	events := make(chan domain.DownloadEvent)
	defer close(events)
	go g.consumeEvents(events)
	files, err := m3u8.DownloadVideos(
		playlist,
		workDir,
		g.cfg.HTTP.Headers,
		g.cfg.Videos.MaxParallel,
		g.cfg.HTTP.Insecure,
		events,
	)
	if err != nil {
		g.showError(err)
		return
	}

	var inputFile string
	switch playlist.Type {
	case domain.PlaylistTS:
		inputFile, err = ffmpeg.PrepareFileList(workDir, files)
		if err != nil {
			g.showError(err)
		}
	case domain.PlaylistFMP4:
		inputFile, err = ffmpeg.ConcatFiles(workDir, files)
		if err != nil {
			g.showError(err)
		}
	default:
		g.showError(fmt.Errorf("playlist type not supported"))
	}

	fyne.Do(func() {
		g.currentFileLabel.SetText(
			fmt.Sprintf("Converting to %s", filepath.Base(absOutputFile)),
		)
	})

	ffmevents := make(chan domain.FFmpegEvent)
	defer close(ffmevents)
	go func() {
		for ev := range ffmevents {
			fyne.Do(func() {
				g.progressBar.SetValue(ev.Percent)
			})
		}
	}()

	ffm := ffmpeg.New(absFfmpegPath)
	if err := ffm.Convert(playlist, inputFile, absOutputFile, ffmevents); err != nil {
		g.showError(err)
		return
	}

	fyne.Do(func() {
		g.currentFileLabel.SetText(
			fmt.Sprintf("%s created!", filepath.Base(absOutputFile)),
		)
		dialog.ShowInformation(
			"File created",
			fmt.Sprintf("%s created!", filepath.Base(absOutputFile)),
			g.window,
		)
		g.resetForm()
	})
}
