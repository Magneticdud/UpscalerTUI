package ui

import (
	"context"
	"path/filepath"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"github.com/Magneticdud/UpscalerGUI/internal/config"
	"github.com/Magneticdud/UpscalerGUI/internal/engine"
)

func showExpertFlow(app fyne.App, parent fyne.Window) {
	w := app.NewWindow("Expert mode")
	w.Resize(fyne.NewSize(600, 600))

	// ── Engine ────────────────────────────────────────────────────────────────
	engineSelect := widget.NewSelect([]string{"Real-CUGAN", "Real-ESRGAN"}, nil)
	engineSelect.SetSelected("Real-CUGAN")

	// ── Model ─────────────────────────────────────────────────────────────────
	modelSelect := widget.NewSelect(modelLabels("realcugan"), nil)
	modelSelect.SetSelected(modelLabels("realcugan")[0])

	// ── Scale ─────────────────────────────────────────────────────────────────
	scaleSelect := widget.NewSelect([]string{}, nil)

	// ── Noise (RealCUGAN only) ────────────────────────────────────────────────
	noiseSelect := widget.NewSelect([]string{}, nil)
	noiseLabel := widget.NewLabel("Noise reduction")

	// ── TTA ───────────────────────────────────────────────────────────────────
	ttaCheck := widget.NewCheck("Maximum quality TTA (slower, ~8x)", nil)

	// ── GPU ID ────────────────────────────────────────────────────────────────
	gpuEntry := widget.NewEntry()
	gpuEntry.SetPlaceHolder("GPU ID (empty = default, -1 = CPU)")
	gpuError := widget.NewLabel("")
	gpuError.Hide()

	gpuEntry.OnChanged = func(s string) {
		if err := engine.ValidateGPUID(s); err != nil {
			gpuError.SetText(err.Error())
			gpuError.Show()
		} else {
			gpuError.Hide()
		}
	}

	// ── Threads ───────────────────────────────────────────────────────────────
	threadEntry := widget.NewEntry()
	threadEntry.SetPlaceHolder("Threads (empty = default, e.g. 2:2:2)")
	threadError := widget.NewLabel("")
	threadError.Hide()

	threadEntry.OnChanged = func(s string) {
		if err := engine.ValidateThreads(s); err != nil {
			threadError.SetText(err.Error())
			threadError.Show()
		} else {
			threadError.Hide()
		}
	}

	// ── Input path ────────────────────────────────────────────────────────────
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

	// ── Output directory ──────────────────────────────────────────────────────
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

	// ── Dynamic update helpers ────────────────────────────────────────────────
	updateNoiseAndScale := func(eng, model string) {
		if eng == "realcugan" {
			modelKey := modelKeyFromLabel(eng, model)
			scales := config.GetRealcuganScales(modelKey)
			scaleSelect.Options = scales
			if len(scales) > 0 {
				scaleSelect.SetSelected(scales[0])
			}
			scaleSelect.Refresh()

			noises := noiseDisplayLabels(modelKey, scaleSelect.Selected)
			noiseSelect.Options = noises
			if len(noises) > 0 {
				noiseSelect.SetSelected(noises[0])
			}
			noiseSelect.Refresh()
			noiseSelect.Show()
			noiseLabel.Show()
		} else {
			modelKey := modelKeyFromLabel(eng, model)
			scales := config.RealesrganModelScales[modelKey]
			scaleSelect.Options = scales
			if len(scales) > 0 {
				scaleSelect.SetSelected(scales[0])
			}
			scaleSelect.Refresh()
			noiseSelect.Hide()
			noiseLabel.Hide()
		}
	}

	// Scale change for RealCUGAN also updates noise options
	scaleSelect.OnChanged = func(scale string) {
		if engineSelect.Selected == "Real-CUGAN" {
			modelKey := modelKeyFromLabel("realcugan", modelSelect.Selected)
			noises := noiseDisplayLabels(modelKey, scale)
			noiseSelect.Options = noises
			if len(noises) > 0 {
				noiseSelect.SetSelected(noises[0])
			}
			noiseSelect.Refresh()
		}
	}

	modelSelect.OnChanged = func(model string) {
		eng := engineKey(engineSelect.Selected)
		updateNoiseAndScale(eng, model)
	}

	engineSelect.OnChanged = func(label string) {
		eng := engineKey(label)
		models := modelLabels(eng)
		modelSelect.Options = models
		if len(models) > 0 {
			modelSelect.SetSelected(models[0])
		}
		modelSelect.Refresh()
		updateNoiseAndScale(eng, models[0])
	}

	// Initial population
	updateNoiseAndScale("realcugan", modelLabels("realcugan")[0])

	// ── Start button ──────────────────────────────────────────────────────────
	startBtn := widget.NewButton("Start", func() {
		inputPath := strings.TrimSpace(inputEntry.Text)
		if inputPath == "" {
			dialog.ShowError(errorf("Please select an input file or directory."), w)
			return
		}
		if err := engine.ValidateGPUID(gpuEntry.Text); err != nil {
			dialog.ShowError(err, w)
			return
		}
		if err := engine.ValidateThreads(threadEntry.Text); err != nil {
			dialog.ShowError(err, w)
			return
		}

		images, err := engine.GetImageFiles(inputPath)
		if err != nil || len(images) == 0 {
			dialog.ShowError(errorf("No images found at: %s", inputPath), w)
			return
		}

		eng := engineKey(engineSelect.Selected)
		modelKey := modelKeyFromLabel(eng, modelSelect.Selected)
		scaleVal := scaleSelect.Selected
		noiseVal := noiseKeyFromLabel(noiseSelect.Selected)
		useTTA := ttaCheck.Checked
		gpuID := strings.TrimSpace(gpuEntry.Text)
		threads := strings.TrimSpace(threadEntry.Text)
		outputDir := strings.TrimSpace(outputEntry.Text)
		useJpeg2png := config.Jpeg2pngModels[modelKey]

		ctx, cancel := context.WithCancel(context.Background())
		logCh := make(chan string, 256)
		pw := NewProgressWindow(app, "Expert mode — processing…", cancel)

		go func() {
			allOK := true
			for _, img := range images {
				pw.AppendLine("Processing: " + filepath.Base(img))

				var srcPath string
				var j2p engine.Jpeg2pngResult
				if useJpeg2png {
					j2p, _ = engine.RunJpeg2png(ctx, img)
				}
				if j2p.PngPath != "" {
					srcPath = j2p.PngPath
					pw.AppendLine("  jpeg2png: " + filepath.Base(img) + " → " + filepath.Base(srcPath))
				} else {
					srcPath = img
				}

				var outPath string
				var ok bool
				if eng == "realcugan" {
					tag := modelKey + "_n" + noiseVal + "_s" + scaleVal
					outPath = engine.GetOutputPath(img, "realcugan", tag, outputDir)
					if j2p.PngPath != "" {
						outPath = replaceExt(outPath, ".png")
					}
					ok = engine.ProcessRealcugan(ctx, logCh, engine.RealcuganOptions{
						InputPath:  srcPath,
						OutputPath: outPath,
						Scale:      scaleVal,
						NoiseLevel: noiseVal,
						ModelDir:   modelKey,
						TTA:        useTTA,
						GPUID:      gpuID,
						Threads:    threads,
					})
				} else {
					tag := modelKey + "_s" + scaleVal
					outPath = engine.GetOutputPath(img, "realesrgan", tag, outputDir)
					if j2p.PngPath != "" {
						outPath = replaceExt(outPath, ".png")
					}
					ok = engine.ProcessRealesrgan(ctx, logCh, engine.RealesrganOptions{
						InputPath:  srcPath,
						OutputPath: outPath,
						Scale:      scaleVal,
						ModelName:  modelKey,
						TTA:        useTTA,
						GPUID:      gpuID,
						Threads:    threads,
					})
				}
				j2p.Cleanup()
				if !ok {
					allOK = false
				}
			}
			if allOK {
				logCh <- "\n✓ Done!"
			} else {
				logCh <- "\n✗ Completed with errors."
			}
			close(logCh)
		}()

		go pw.DrainChannel(logCh)
		w.Hide()
	})
	startBtn.Importance = widget.HighImportance

	// ── Layout ────────────────────────────────────────────────────────────────
	form := widget.NewForm(
		widget.NewFormItem("Engine", engineSelect),
		widget.NewFormItem("Model", modelSelect),
		widget.NewFormItem("Scale", scaleSelect),
		widget.NewFormItem(noiseLabel.Text, noiseSelect),
		widget.NewFormItem("", ttaCheck),
		widget.NewFormItem("GPU ID", container.NewVBox(gpuEntry, gpuError)),
		widget.NewFormItem("Threads", container.NewVBox(threadEntry, threadError)),
		widget.NewFormItem("Input", inputRow),
		widget.NewFormItem("Output dir", outputRow),
	)

	w.SetContent(container.NewPadded(container.NewVBox(form, startBtn)))
	w.Show()
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func engineKey(label string) string {
	if label == "Real-ESRGAN" {
		return "realesrgan"
	}
	return "realcugan"
}

func modelLabels(eng string) []string {
	if eng == "realesrgan" {
		labels := make([]string, len(config.RealesrganModels))
		for i, m := range config.RealesrganModels {
			labels[i] = config.RealesrganModelLabels[m]
		}
		return labels
	}
	labels := make([]string, len(config.RealcuganModels))
	for i, m := range config.RealcuganModels {
		labels[i] = config.RealcuganModelLabels[m]
	}
	return labels
}

func modelKeyFromLabel(eng, label string) string {
	if eng == "realesrgan" {
		for k, v := range config.RealesrganModelLabels {
			if v == label {
				return k
			}
		}
		return config.RealesrganModels[0]
	}
	for k, v := range config.RealcuganModelLabels {
		if v == label {
			return k
		}
	}
	return config.RealcuganModels[0]
}

func noiseDisplayLabels(modelKey, scale string) []string {
	levels := config.GetValidNoiseLevels(modelKey, scale)
	labels := make([]string, len(levels))
	for i, n := range levels {
		labels[i] = config.NoiseLabels[n] + " (" + n + ")"
	}
	return labels
}

func noiseKeyFromLabel(label string) string {
	// Extract the key from "Label text (key)"
	start := strings.LastIndex(label, "(")
	end := strings.LastIndex(label, ")")
	if start >= 0 && end > start {
		return label[start+1 : end]
	}
	return "-1"
}

func errorf(format string, args ...interface{}) error {
	if len(args) == 0 {
		return &simpleError{format}
	}
	return &simpleError{formatStr(format, args...)}
}

type simpleError struct{ msg string }

func (e *simpleError) Error() string { return e.msg }

func formatStr(format string, args ...interface{}) string {
	// Simple %s substitution for the few uses in this file
	result := format
	for _, a := range args {
		idx := strings.Index(result, "%s")
		if idx < 0 {
			break
		}
		result = result[:idx] + toString(a) + result[idx+2:]
	}
	return result
}

func toString(v interface{}) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}
