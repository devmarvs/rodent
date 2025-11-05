package modules

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

type testBenchModule struct{}

func (m *testBenchModule) Name() string {
	return "Test Bench"
}

func (m *testBenchModule) Content() fyne.CanvasObject {
	return container.NewVBox(
		widget.NewLabelWithStyle("Testing Tools", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewLabel("Run checks and validate output from recent scans."),
		widget.NewButton("Run Diagnostics", func() {}),
	)
}
