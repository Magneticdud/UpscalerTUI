package ui

import (
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"github.com/Magneticdud/UpscalerGUI/internal/engine"
)

// Run initialises and runs the Fyne application.
func Run() {
	a := app.New()
	a.SetIcon(nil) // icon set via main.go if desired

	w := a.NewWindow("Upscaler")
	w.Resize(fyne.NewSize(420, 300))
	w.SetFixedSize(true)

	// Startup validation
	missing, err := engine.MissingBinaries()
	if err != nil || len(missing) > 0 {
		msg := "Required binaries not found in bin/:\n" + strings.Join(missing, "\n") +
			"\n\nPlease run the app from the correct directory."
		dialog.ShowError(errorf(msg), w)
	}

	missingModels, _ := engine.MissingModels()
	if len(missingModels) > 0 {
		msg := "Some model directories are missing from bin/:\n" + strings.Join(missingModels, "\n") +
			"\n\nSome upscaling options may not work."
		dialog.ShowInformation("Warning", msg, w)
	}

	// ── Main menu ─────────────────────────────────────────────────────────────
	guidedBtn := widget.NewButton("Guided mode — choose what you're upscaling", func() {
		showGuidedFlow(a, w)
	})
	guidedBtn.Importance = widget.HighImportance

	expertBtn := widget.NewButton("Expert mode — full control over parameters", func() {
		showExpertFlow(a, w)
	})

	tryAllBtn := widget.NewButton("Try All — run all models and compare", func() {
		showTryAllFlow(a, w)
	})

	quitBtn := widget.NewButton("Exit", func() {
		a.Quit()
	})

	content := container.NewVBox(
		widget.NewLabel("Upscaler"),
		widget.NewSeparator(),
		guidedBtn,
		expertBtn,
		tryAllBtn,
		widget.NewSeparator(),
		quitBtn,
	)

	w.SetContent(container.NewPadded(content))
	w.ShowAndRun()
}
