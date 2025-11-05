package modules

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

type reportsModule struct{}

func (m *reportsModule) Name() string {
	return "Reports"
}

func (m *reportsModule) Content() fyne.CanvasObject {
	reportList := widget.NewList(
		func() int { return len(mockReports) },
		func() fyne.CanvasObject { return widget.NewLabel("") },
		func(i int, obj fyne.CanvasObject) {
			obj.(*widget.Label).SetText(mockReports[i])
		},
	)

	return container.NewBorder(
		nil, nil, nil, nil,
		container.NewVBox(
			widget.NewLabelWithStyle("Recent Reports", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			reportList,
		),
	)
}

var mockReports = []string{
	"Daily Scan - OK",
	"Credential Audit - Attention",
	"Network Surface - OK",
}
