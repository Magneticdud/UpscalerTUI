package ui

import (
	"context"
	"path/filepath"
	"strings"

	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"github.com/Magneticdud/UpscalerGUI/internal/config"
	"github.com/Magneticdud/UpscalerGUI/internal/engine"
)

func showGuidedFlow(app fyne.App, parent fyne.Window) {
	w := app.NewWindow("Guided mode")
	w.Resize(fyne.NewSize(560, 520))

	// ── Preset selector ───────────────────────────────────────────────────────
	presets := config.LoadPresets()
	presetNames := make([]string, 0, len(presets)+1)
	presetNames = append(presetNames, "(no preset)")
	for name := range presets {
		presetNames = append(presetNames, name)
	}
	presetSelect := widget.NewSelect(presetNames, nil)
	presetSelect.SetSelected("(no preset)")

	// ── Input path ────────────────────────────────────────────────────────────
	inputEntry := widget.NewEntry()
	inputEntry.SetPlaceHolder("File or directory…")
	browseBtn := widget.NewButton("Browse…", func() {
		dialog.ShowFolderOpen(func(uri fyne.ListableURI, err error) {
			if err != nil || uri == nil {
				// Try file open too
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
	inputRow := container.NewBorder(nil, nil, nil, browseBtn, inputEntry)

	// ── Image type ────────────────────────────────────────────────────────────
	typeLabels := make([]string, len(config.ImageTypeOrder))
	for i, k := range config.ImageTypeOrder {
		typeLabels[i] = config.ImageTypeLabels[k]
	}
	typeSelect := widget.NewSelect(typeLabels, nil)
	typeSelect.SetSelected(typeLabels[0])

	// ── Scale ─────────────────────────────────────────────────────────────────
	scaleSelect := widget.NewSelect([]string{"4x"}, nil)
	scaleSelect.SetSelected("4x")

	// Update scale choices when image type changes
	typeSelect.OnChanged = func(label string) {
		key := labelToTypeKey(label)
		scales := config.GuidedValidScales[key]
		defaultScale := config.ImageTypePresets[key].Scale
		opts := make([]string, len(scales))
		for i, s := range scales {
			if s == defaultScale {
				opts[i] = s + "x (recommended)"
			} else {
				opts[i] = s + "x"
			}
		}
		scaleSelect.Options = opts
		scaleSelect.SetSelected(opts[0])
		scaleSelect.Refresh()
	}
	// Trigger initial update
	typeSelect.OnChanged(typeLabels[0])

	// ── Quality ───────────────────────────────────────────────────────────────
	qualitySelect := widget.NewSelect([]string{
		"Balanced (faster)",
		"Maximum quality (~8x slower)",
	}, nil)
	qualitySelect.SetSelected("Balanced (faster)")

	// ── Output directory ──────────────────────────────────────────────────────
	outputEntry := widget.NewEntry()
	outputEntry.SetPlaceHolder("Same directory as input (default)")
	outputBrowse := widget.NewButton("Browse…", func() {
		dialog.ShowFolderOpen(func(uri fyne.ListableURI, err error) {
			if err != nil || uri == nil {
				return
			}
			outputEntry.SetText(uri.Path())
		}, w)
	})
	outputRow := container.NewBorder(nil, nil, nil, outputBrowse, outputEntry)

	// ── Save preset ───────────────────────────────────────────────────────────
	saveCheck := widget.NewCheck("Save as preset", nil)
	presetNameEntry := widget.NewEntry()
	presetNameEntry.SetPlaceHolder("Preset name…")
	presetNameEntry.Hide()
	saveCheck.OnChanged = func(checked bool) {
		if checked {
			presetNameEntry.Show()
		} else {
			presetNameEntry.Hide()
		}
	}

	// ── Start button ──────────────────────────────────────────────────────────
	startBtn := widget.NewButton("Start", nil)
	startBtn.Importance = widget.HighImportance
	startBtn.OnTapped = func() {
		inputPath := strings.TrimSpace(inputEntry.Text)
		if inputPath == "" {
			dialog.ShowError(fmt.Errorf("please select an input file or directory"), w)
			return
		}

		// Load from preset if selected
		typeKey := labelToTypeKey(typeSelect.Selected)
		scaleVal := scaleValue(scaleSelect.Selected)
		useTTA := qualitySelect.Selected != "Balanced (faster)"
		outputDir := strings.TrimSpace(outputEntry.Text)

		if sel := presetSelect.Selected; sel != "(no preset)" {
			if p, ok := presets[sel]; ok {
				typeKey = p.ImageType
				scaleVal = p.Scale
				useTTA = p.TTA
			}
		}

		// Save preset if requested
		if saveCheck.Checked && presetNameEntry.Text != "" {
			_ = config.SavePreset(presetNameEntry.Text, config.SavedPreset{
				ImageType: typeKey,
				Scale:     scaleVal,
				TTA:       useTTA,
			})
		}

		images, err := engine.GetImageFiles(inputPath)
		if err != nil || len(images) == 0 {
			dialog.ShowError(fmt.Errorf("no images found at: %s", inputPath), w)
			return
		}

		preset := config.ImageTypePresets[typeKey]
		useJpeg2png := config.Jpeg2pngGuidedTypes[typeKey]

		ctx, cancel := context.WithCancel(context.Background())
		logCh := make(chan string, 256)
		pw := NewProgressWindow(app, "Processing…", cancel)

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

				tag := typeKey + "_" + scaleVal + "x"
				outPath := engine.GetOutputPath(img, "guided", tag, outputDir)
				if j2p.PngPath != "" {
					outPath = replaceExt(outPath, ".png")
				}

				var ok bool
				if preset.Engine == "realcugan" {
					ok = engine.ProcessRealcugan(ctx, logCh, engine.RealcuganOptions{
						InputPath:  srcPath,
						OutputPath: outPath,
						Scale:      scaleVal,
						NoiseLevel: preset.Noise,
						ModelDir:   preset.Model,
						TTA:        useTTA,
					})
				} else {
					ok = engine.ProcessRealesrgan(ctx, logCh, engine.RealesrganOptions{
						InputPath:  srcPath,
						OutputPath: outPath,
						Scale:      scaleVal,
						ModelName:  preset.Model,
						TTA:        useTTA,
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
	}

	form := widget.NewForm(
		widget.NewFormItem("Preset", presetSelect),
		widget.NewFormItem("Input", inputRow),
		widget.NewFormItem("Image type", typeSelect),
		widget.NewFormItem("Scale", scaleSelect),
		widget.NewFormItem("Quality", qualitySelect),
		widget.NewFormItem("Output dir", outputRow),
	)

	saveRow := container.NewVBox(saveCheck, presetNameEntry)
	content := container.NewVBox(form, saveRow, startBtn)
	w.SetContent(container.NewPadded(content))
	w.Show()
}

// labelToTypeKey converts a display label back to the map key.
func labelToTypeKey(label string) string {
	for k, v := range config.ImageTypeLabels {
		if v == label {
			return k
		}
	}
	return "not_sure"
}

// scaleValue strips the "x" and " (recommended)" from a scale display string.
func scaleValue(s string) string {
	s = strings.TrimSuffix(s, " (recommended)")
	s = strings.TrimSuffix(s, "x")
	return s
}

// replaceExt replaces the extension of a file path.
func replaceExt(path, ext string) string {
	return strings.TrimSuffix(path, filepath.Ext(path)) + ext
}
