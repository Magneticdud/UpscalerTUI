package ui

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"github.com/Magneticdud/UpscalerGUI/internal/config"
	"github.com/Magneticdud/UpscalerGUI/internal/engine"
)

// tryAllResult stores a single combination result for the summary table.
type tryAllResult struct {
	filename   string
	modelLabel string
	scale      string
	ok         bool
}

func showTryAllFlow(app fyne.App, parent fyne.Window) {
	w := app.NewWindow("Try All")
	w.Resize(fyne.NewSize(560, 300))

	inputEntry := widget.NewEntry()
	inputEntry.SetPlaceHolder("File or directory…")
	inputBrowse := widget.NewButton("Browse…", func() {
		dialog.ShowFolderOpen(func(uri fyne.ListableURI, err error) {
			if err != nil || uri == nil {
				dialog.ShowFileOpen(func(uri fyne.URIReadCloser, err error) {
					if err != nil || uri == nil {
						return
					}
					inputEntry.SetText(uri.URI().Path())
					uri.Close()
				}, w)
				return
			}
			inputEntry.SetText(uri.Path())
		}, w)
	})
	inputRow := container.NewBorder(nil, nil, nil, inputBrowse, inputEntry)

	outputEntry := widget.NewEntry()
	outputEntry.SetPlaceHolder("Same as input (default)")
	outputBrowse := widget.NewButton("Browse…", func() {
		dialog.ShowFolderOpen(func(uri fyne.ListableURI, err error) {
			if err != nil || uri == nil {
				return
			}
			outputEntry.SetText(uri.Path())
		}, w)
	})
	outputRow := container.NewBorder(nil, nil, nil, outputBrowse, outputEntry)

	startBtn := widget.NewButton("Start", func() {
		inputPath := strings.TrimSpace(inputEntry.Text)
		if inputPath == "" {
			dialog.ShowError(errorf("Please select an input file or directory."), w)
			return
		}

		images, err := engine.GetImageFiles(inputPath)
		if err != nil || len(images) == 0 {
			dialog.ShowError(errorf("No images found at: %s", inputPath), w)
			return
		}

		// Build all combinations
		type rcCombo struct{ modelDir, scale, noise string }
		type esrCombo struct{ modelName, scale string }

		var rcCombos []rcCombo
		for _, modelDir := range config.RealcuganModels {
			for _, scale := range config.GetRealcuganScales(modelDir) {
				for _, noise := range config.GetValidNoiseLevels(modelDir, scale) {
					rcCombos = append(rcCombos, rcCombo{modelDir, scale, noise})
				}
			}
		}
		var esrCombos []esrCombo
		for _, modelName := range config.RealesrganModels {
			scales := config.RealesrganModelScales[modelName]
			for _, scale := range scales {
				esrCombos = append(esrCombos, esrCombo{modelName, scale})
			}
		}

		total := len(images) * (len(rcCombos) + len(esrCombos))
		msg := fmt.Sprintf(
			"%d image(s) × %d combinations = %d outputs\n\nThis may take a long time. Continue?",
			len(images), len(rcCombos)+len(esrCombos), total,
		)
		dialog.ShowConfirm("Try All", msg, func(confirmed bool) {
			if !confirmed {
				return
			}

			outputDir := strings.TrimSpace(outputEntry.Text)
			ctx, cancel := context.WithCancel(context.Background())
			logCh := make(chan string, 256)
			pw := NewProgressWindow(app, "Try All — processing…", cancel)

			var results []tryAllResult

			go func() {
				allOK := true
				count := 0

				for _, img := range images {
					pw.AppendLine(fmt.Sprintf("\n### %s", filepath.Base(img)))

					j2p, _ := engine.RunJpeg2png(ctx, img)
					if j2p.PngPath != "" {
						pw.AppendLine("  jpeg2png: " + filepath.Base(img) + " → " + filepath.Base(j2p.PngPath))
					}

					for _, rc := range rcCombos {
						count++
						src := img
						if j2p.PngPath != "" {
							src = j2p.PngPath
						}
						tag := rc.modelDir + "_n" + rc.noise + "_s" + rc.scale
						outPath := engine.GetOutputPath(img, "realcugan", tag, outputDir)
						if j2p.PngPath != "" {
							outPath = replaceExt(outPath, ".png")
						}
						pw.AppendLine(fmt.Sprintf("[%d] Real-CUGAN | %s | noise=%s | scale=%sx",
							count, rc.modelDir, rc.noise, rc.scale))
						ok := engine.ProcessRealcugan(ctx, logCh, engine.RealcuganOptions{
							InputPath:  src,
							OutputPath: outPath,
							Scale:      rc.scale,
							NoiseLevel: rc.noise,
							ModelDir:   rc.modelDir,
						})
						if !ok {
							allOK = false
						}
						label := config.RealcuganModelLabels[rc.modelDir] + ", " + config.NoiseLabels[rc.noise]
						results = append(results, tryAllResult{
							filename:   filepath.Base(outPath),
							modelLabel: label,
							scale:      rc.scale + "x",
							ok:         ok,
						})
					}

					for _, esr := range esrCombos {
						count++
						src := img
						useJ2p := j2p.PngPath != "" && config.Jpeg2pngModels[esr.modelName]
						if useJ2p {
							src = j2p.PngPath
						}
						tag := esr.modelName + "_s" + esr.scale
						outPath := engine.GetOutputPath(img, "realesrgan", tag, outputDir)
						if useJ2p {
							outPath = replaceExt(outPath, ".png")
						}
						pw.AppendLine(fmt.Sprintf("[%d] Real-ESRGAN | %s | scale=%sx",
							count, esr.modelName, esr.scale))
						ok := engine.ProcessRealesrgan(ctx, logCh, engine.RealesrganOptions{
							InputPath:  src,
							OutputPath: outPath,
							Scale:      esr.scale,
							ModelName:  esr.modelName,
						})
						if !ok {
							allOK = false
						}
						results = append(results, tryAllResult{
							filename:   filepath.Base(outPath),
							modelLabel: config.RealesrganModelLabels[esr.modelName],
							scale:      esr.scale + "x",
							ok:         ok,
						})
					}

					j2p.Cleanup()
				}

			if allOK {
					logCh <- "\n✓ Done!"
				} else {
					logCh <- "\n✗ Completed with errors."
				}
				close(logCh)
				// Show results table in a new window
				showResultsTable(app, results)
			}()

			go pw.DrainChannel(logCh)
			w.Hide()
		}, w)
	})
	startBtn.Importance = widget.HighImportance

	form := widget.NewForm(
		widget.NewFormItem("Input", inputRow),
		widget.NewFormItem("Output dir", outputRow),
	)
	w.SetContent(container.NewPadded(container.NewVBox(form, startBtn)))
	w.Show()
}

// showResultsTable opens a virtual results table window.
func showResultsTable(app fyne.App, results []tryAllResult) {
	w := app.NewWindow("Try All — Results")
	w.Resize(fyne.NewSize(800, 500))

	ok := 0
	for _, r := range results {
		if r.ok {
			ok++
		}
	}

	title := widget.NewLabel(fmt.Sprintf("Results: %d / %d completed", ok, len(results)))
	title.TextStyle = fyne.TextStyle{Bold: true}

	headers := []string{"Output file", "Model", "Scale", "Status"}
	list := widget.NewTable(
		func() (int, int) { return len(results) + 1, 4 },
		func() fyne.CanvasObject {
			return widget.NewLabel("placeholder")
		},
		func(id widget.TableCellID, obj fyne.CanvasObject) {
			label := obj.(*widget.Label)
			if id.Row == 0 {
				label.SetText(headers[id.Col])
				label.TextStyle = fyne.TextStyle{Bold: true}
				return
			}
			r := results[id.Row-1]
			switch id.Col {
			case 0:
				label.SetText(r.filename)
			case 1:
				label.SetText(r.modelLabel)
			case 2:
				label.SetText(r.scale)
			case 3:
				if r.ok {
					label.SetText("OK")
				} else {
					label.SetText("FAIL")
				}
			}
			label.TextStyle = fyne.TextStyle{}
		},
	)
	list.SetColumnWidth(0, 300)
	list.SetColumnWidth(1, 250)
	list.SetColumnWidth(2, 60)
	list.SetColumnWidth(3, 60)

	w.SetContent(container.NewBorder(container.NewPadded(title), nil, nil, nil, list))
	w.Show()
}
