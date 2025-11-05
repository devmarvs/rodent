package modules

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
)

type scannerModule struct{}

func (m *scannerModule) Name() string {
	return "Scanner"
}

func (m *scannerModule) Content() fyne.CanvasObject {
	targetEntry := widget.NewEntry()
	targetEntry.SetPlaceHolder("Target hostname or IP")

	scanButton := widget.NewButton("Scan", func() {})
	buttonMin := scanButton.MinSize()
	const buttonWidthScale float32 = 1.5
	buttonWidth := fyne.NewSize(buttonMin.Width*buttonWidthScale, buttonMin.Height)

	entryContainer := container.New(layout.NewGridWrapLayout(fyne.NewSize(450, targetEntry.MinSize().Height)), targetEntry)
	buttonContainer := container.New(layout.NewGridWrapLayout(buttonWidth), scanButton)

	actionRow := container.NewHBox(entryContainer, buttonContainer)

	return container.NewVBox(
		widget.NewLabel("Configure and monitor scanning tasks."),
		actionRow,
	)
}
