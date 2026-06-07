package main

import (
	"fmt"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	"github.com/Mithweth/m3u8-downloader/internal/config"
	"github.com/Mithweth/m3u8-downloader/internal/ffmpeg"
	"github.com/Mithweth/m3u8-downloader/internal/m3u8"
	"github.com/Mithweth/m3u8-downloader/internal/utils/ostools"
	"path/filepath"
	"slices"
	"strconv"
)

func main() {
	a := app.New()

	mainWindow := a.NewWindow("M3U8 Downloader")
	mainWindow.Resize(fyne.NewSize(600, 200))
	tomlCfg, err := config.LoadConfig()
	if err != nil {
		dialog.ShowError(err, mainWindow)
	}
	ffmpegPath := widget.NewEntry()
	ffmpegPath.SetText(tomlCfg.Paths.FFmpegBinary)
	ffmpegPath.OnChanged = func(value string) {
		tomlCfg.Paths.FFmpegBinary = value
	}
	outputFile := widget.NewEntry()
	referer := widget.NewEntry()
	referer.SetText(tomlCfg.Headers["Referer"])
	referer.OnChanged = func(value string) {
		tomlCfg.Headers["Referer"] = value
	}
	userAgent := widget.NewEntry()
	userAgent.SetText(tomlCfg.Headers["User-Agent"])
	userAgent.OnChanged = func(value string) {
		tomlCfg.Headers["User-Agent"] = value
	}
	quitButton := widget.NewButton("Quit", func() {
		a.Quit()
	})
	formatSelect := widget.NewSelect([]string{}, nil)
	formatSelect.Disable()
	saveConfigButton := widget.NewButton("Save config", func() {
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
	m3uUrl := widget.NewEntry()
	progressBar := widget.NewProgressBar()
	currentFileLabel := widget.NewLabel("")
	var processButton *widget.Button
	processButton = widget.NewButton("Process", func() {
		entries, err := m3u8.DownloadM3U8(m3uUrl.Text, tomlCfg.Headers)
		if err != nil {
			dialog.ShowError(err, mainWindow)
			return
		}
		fmt.Printf("Is %v a playlist ? %t\n", m3uUrl.Text, m3u8.IsPlaylist(entries))
		if m3u8.IsPlaylist(entries) {
			if formatSelect.Selected == "" {
				formats := m3u8.GetFormats(entries)
				formatSelect.Options = formats
				processButton.Disable()
				if slices.Contains(formats, tomlCfg.Videos.PreferredFormat) {
					formatSelect.SetSelected(tomlCfg.Videos.PreferredFormat)
					processButton.Enable()
				}
				formatSelect.Enable()
				return
			} else {
				for _, entry := range entries {
					if m3u8.IsPreferredFormat(entry, formatSelect.Selected) {
						entries, err = m3u8.DownloadM3U8(entry, tomlCfg.Headers)
						if err != nil {
							dialog.ShowError(err, mainWindow)
							return
						}
						break
					}
				}
			}
		}
		fmt.Println("files to download:", entries)

		absFfmpegPath, err := ostools.ExpandPath(ffmpegPath.Text)
		if err != nil {
			dialog.ShowError(err, mainWindow)
			return
		}
		absFfmpegPath, err = ffmpeg.ValidateBinary(absFfmpegPath)
		if err != nil {
			dialog.ShowError(err, mainWindow)
			return
		}
		absOutputFile, err := ostools.ExpandPath(outputFile.Text)
		if err != nil {
			dialog.ShowError(err, mainWindow)
			return
		}
		fmt.Println("output file:", absOutputFile)
		fmt.Println("ffmpeg binary:", absFfmpegPath)
		workDir, cleanup, err := ostools.GetWorkingDirectory("")
		fmt.Println("temporary directory:", workDir)
		if err != nil {
			dialog.ShowError(err, mainWindow)
			return
		}
		events := make(chan m3u8.DownloadEvent)

		go func() {
			for ev := range events {
				fyne.Do(func() {
					progressBar.SetValue(float64(ev.Done) / float64(ev.Total))
					currentFileLabel.SetText(fmt.Sprintf("Downloading %s", filepath.Base(ev.FilePath)))

					if !ev.Success {
						err = ev.Error
						fmt.Println(err)
						dialog.ShowError(ev.Error, mainWindow)
					}
				})
			}
		}()

		go func() {
			defer cleanup()
			files, err := m3u8.DownloadVideos(
				entries,
				workDir,
				tomlCfg.Headers,
				tomlCfg.Videos.MaxParallel,
				events,
			)
			close(events)

			if err != nil {
				fmt.Println(err)
				fyne.Do(func() {
					dialog.ShowError(err, mainWindow)
				})
				return
			}

			fmt.Println("downloaded files:", files)
			fileList, err := ffmpeg.PrepareFileList(workDir, files)
			if err != nil {
				fyne.Do(func() {
					dialog.ShowError(err, mainWindow)
				})
				return
			}

			fyne.Do(func() {
				currentFileLabel.SetText(fmt.Sprintf("Converting to %s", filepath.Base(absOutputFile)))
			})

			if errConvert := ffmpeg.Convert(absFfmpegPath, fileList, absOutputFile); errConvert != nil {
				err = errConvert
				dialog.ShowError(err, mainWindow)
				return
			}

			fyne.Do(func() {
				currentFileLabel.SetText(fmt.Sprintf("%s created !", filepath.Base(absOutputFile)))
				outputFile.SetText("")
				m3uUrl.SetText("")
				formatSelect.Disable()
				outputFile.Disable()
				m3uUrl.Disable()
				processButton.Disable()
			})
		}()
	})
	processButton.Disable()
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
		tomlCfg.Videos.PreferredFormat = value
		if value == "" || m3uUrl.Text == "" || outputFile.Text == "" {
			processButton.Disable()
		} else {
			processButton.Enable()
		}
	}
	threadNumber := widget.NewSelect([]string{"1", "2", "3", "4", "5", "6", "7", "8"}, nil)
	threadNumber.SetSelected(strconv.Itoa(tomlCfg.Videos.MaxParallel))
	threadNumber.OnChanged = func(value string) {
		tomlCfg.Videos.MaxParallel, err = strconv.Atoi(threadNumber.Selected)
		if err != nil {
			dialog.ShowError(err, mainWindow)
		}
	}

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
				nil,
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
				container.NewBorder(
					nil, nil,
					widget.NewLabel("Threads"),
					nil,
					threadNumber,
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
