package main

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	"github.com/Mithweth/m3u8-downloader/internal/config"
	"strconv"
)

func main() {
	a := app.NewWithID("com.github.mithweth.m3u8-downloader")

	mainWindow := a.NewWindow("M3U8 Downloader")
	mainWindow.Resize(fyne.NewSize(600, 200))
	tomlCfg, err := config.LoadConfig()
	if err != nil {
		dialog.ShowError(err, mainWindow)
	}
	ffmpegPath := widget.NewEntry()
	ffmpegPath.SetText(tomlCfg.Paths.FFmpegBinary)
	outputFile := widget.NewEntry()
	referer := widget.NewEntry()
	referer.SetText(tomlCfg.HTTP.Headers["Referer"])
	userAgent := widget.NewEntry()
	userAgent.SetText(tomlCfg.HTTP.Headers["User-Agent"])
	quitButton := widget.NewButton("Quit", func() {
		a.Quit()
	})
	formatSelect := widget.NewSelect([]string{}, nil)
	formatSelect.Disable()
	m3uUrl := widget.NewEntry()
	progressBar := widget.NewProgressBar()
	currentFileLabel := widget.NewLabel("")
	var processButton *widget.Button
	m3uUrl.OnChanged = func(value string) {
		if value == "" || outputFile.Text == "" {
			processButton.Disable()
		} else {
			processButton.Enable()
		}
	}
	outputFile.OnChanged = func(value string) {
		if m3uUrl.Text == "" || value == "" {
			processButton.Disable()
		} else {
			processButton.Enable()
		}
	}
	formatSelect.OnChanged = func(value string) {
		if value == "" || m3uUrl.Text == "" || outputFile.Text == "" {
			processButton.Disable()
		} else {
			processButton.Enable()
		}
	}
	threadNumber := widget.NewSelect([]string{"1", "2", "3", "4", "5", "6", "7", "8"}, nil)
	threadNumber.SetSelected(strconv.Itoa(tomlCfg.Videos.MaxParallel))
	insecureCheckbox := widget.NewCheck("Insecure SSL", nil)
	insecureCheckbox.SetChecked(tomlCfg.HTTP.Insecure)
	processButton = widget.NewButton("Process", func() {
		maxParallel, err := strconv.Atoi(threadNumber.Selected)
		if err != nil {
			dialog.ShowError(err, mainWindow)
		}
		gui := GUI{
			window: mainWindow,
			cfg: config.TomlConfig{
				Videos: config.VideosConfig{
					PreferredFormat: formatSelect.Selected,
					MaxParallel:     maxParallel,
				},
				Paths: config.PathsConfig{FFmpegBinary: ffmpegPath.Text},
				HTTP: config.HTTPConfig{
					Headers: map[string]string{
						"Referer":    referer.Text,
						"User-Agent": userAgent.Text,
					},
					Insecure: insecureCheckbox.Checked,
				},
			},
			m3uUrl:           m3uUrl,
			outputFile:       outputFile,
			ffmpegPath:       ffmpegPath,
			formatSelect:     formatSelect,
			progressBar:      progressBar,
			currentFileLabel: currentFileLabel,
			processButton:    processButton,
		}
		go gui.Process()
	})
	processButton.Disable()
	saveConfigButton := widget.NewButton("Save config", func() {
		tomlCfg.HTTP.Insecure = insecureCheckbox.Checked
		tomlCfg.Videos.PreferredFormat = formatSelect.Selected
		tomlCfg.Videos.MaxParallel, err = strconv.Atoi(threadNumber.Selected)
		if err != nil {
			dialog.ShowError(err, mainWindow)
		}
		tomlCfg.Paths.FFmpegBinary = ffmpegPath.Text
		tomlCfg.HTTP.Headers["Referer"] = referer.Text
		tomlCfg.HTTP.Headers["User-Agent"] = userAgent.Text
		if err := config.SaveConfig(tomlCfg); err != nil {
			dialog.ShowError(err, mainWindow)
			return
		}

		dialog.ShowInformation(
			"Config saved",
			"Configuration saved successfully.",
			mainWindow,
		)
	})

	browseButton := widget.NewButton("Browse", func() {
		dialog.ShowFileSave(func(writer fyne.URIWriteCloser, err error) {
			if err != nil {
				dialog.ShowError(err, mainWindow)
				return
			}

			if writer == nil {
				return
			}

			outputFile.SetText(writer.URI().Path())
			_ = writer.Close()
		}, mainWindow)
	})

	mainWindow.SetContent(
		container.NewVBox(
			container.NewBorder(
				nil, nil,
				widget.NewLabel("M3U8 URL"),
				nil,
				m3uUrl,
			),
			container.NewBorder(
				nil, nil,
				widget.NewLabel("Output file"),
				browseButton,
				outputFile,
			),
			container.NewBorder(
				nil, nil,
				widget.NewLabel("Format"),
				nil,
				formatSelect,
			),
			container.NewCenter(processButton),
			container.NewGridWithColumns(
				2,
				container.NewBorder(
					nil, nil,
					widget.NewLabel("Referer"),
					nil,
					referer,
				),
				container.NewBorder(
					nil, nil,
					widget.NewLabel("User-Agent"),
					nil,
					userAgent,
				),
			),
			container.NewGridWithColumns(
				2,
				container.NewBorder(
					nil, nil,
					widget.NewLabel("FFmpeg"),
					nil,
					ffmpegPath,
				),
				container.NewHBox(
					widget.NewLabel("Threads"),
					threadNumber,
					insecureCheckbox,
				),
			),
			container.NewCenter(saveConfigButton),
			container.NewBorder(
				nil, nil,
				widget.NewLabel("Progress"),
				nil,
				progressBar,
			),
			container.NewBorder(
				nil, nil,
				widget.NewLabel("File"),
				nil,
				currentFileLabel,
			),
			container.NewCenter(quitButton),
		),
	)

	mainWindow.ShowAndRun()
}
