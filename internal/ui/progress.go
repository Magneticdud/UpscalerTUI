package ui

import (
	"context"
	"strings"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

const maxLogLines = 1000

// ProgressWindow is a modal window showing subprocess log output with a cancel button.
type ProgressWindow struct {
	win    fyne.Window
	label  *widget.Label
	scroll *container.Scroll
	mu     sync.Mutex
	lines  []string
	cancel context.CancelFunc
	done   chan struct{}
}

// NewProgressWindow creates and shows the progress window.
// Call cancel() to stop the running job. Close the window when done.
func NewProgressWindow(parent fyne.App, title string, cancel context.CancelFunc) *ProgressWindow {
	pw := &ProgressWindow{
		win:    parent.NewWindow(title),
		cancel: cancel,
		done:   make(chan struct{}),
	}

	pw.label = widget.NewLabel("")
	pw.label.Wrapping = fyne.TextWrapWord

	pw.scroll = container.NewVScroll(pw.label)
	pw.scroll.SetMinSize(fyne.NewSize(700, 400))

	cancelBtn := widget.NewButton("Cancel", func() {
		pw.cancel()
		pw.AppendLine("\n[Cancelled]")
	})

	doneBtn := widget.NewButton("Close", func() {
		pw.win.Close()
	})

	buttons := container.NewHBox(cancelBtn, doneBtn)
	content := container.NewBorder(nil, buttons, nil, nil, pw.scroll)

	pw.win.SetContent(content)
	pw.win.Resize(fyne.NewSize(720, 460))
	pw.win.Show()

	return pw
}

// AppendLine adds a line to the log (thread-safe). Keeps at most maxLogLines.
func (pw *ProgressWindow) AppendLine(line string) {
	pw.mu.Lock()
	pw.lines = append(pw.lines, line)
	if len(pw.lines) > maxLogLines {
		pw.lines = pw.lines[len(pw.lines)-maxLogLines:]
	}
	text := strings.Join(pw.lines, "\n")
	pw.mu.Unlock()

	pw.label.SetText(text)
	pw.scroll.ScrollToBottom()
}

// DrainChannel reads from logCh and appends each line until the channel is closed.
// Blocks until done. Call in a goroutine.
func (pw *ProgressWindow) DrainChannel(logCh <-chan string) {
	for line := range logCh {
		pw.AppendLine(line)
	}
}
