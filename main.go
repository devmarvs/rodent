package main

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	appmodules "github.com/devmarvs/rodent.git/modules"
)

func main() {
	application := app.New()
	window := application.NewWindow("Rodent")

	registered := appmodules.Registered()
	if len(registered) == 0 {
		return
	}

	titleLabel := widget.NewLabel("")
	titleLabel.TextStyle = fyne.TextStyle{Bold: true}

	leftColumn := container.NewVBox(
		widget.NewLabel("Menu"),
	)

	contentContainer := container.NewMax()
	rightColumn := container.NewBorder(
		titleLabel, nil, nil, nil, contentContainer,
	)

	var buttons []*widget.Button
	setActive := func(targetIndex int) {
		for index, btn := range buttons {
			if index == targetIndex {
				btn.Importance = widget.HighImportance
			} else {
				btn.Importance = widget.MediumImportance
			}
			btn.Refresh()
		}

		active := registered[targetIndex]
		titleLabel.SetText(active.Name())
		contentContainer.Objects = []fyne.CanvasObject{active.Content()}
		contentContainer.Refresh()
	}

	for i, module := range registered {
		index := i
		btn := widget.NewButton(module.Name(), func() {
			setActive(index)
		})
		buttons = append(buttons, btn)
		leftColumn.Add(btn)
	}

	setActive(0)

	topBar := container.NewHBox(
		widget.NewButton("File", func() {}),
		widget.NewButton("Settings", func() {}),
	)

	split := container.NewHSplit(leftColumn, rightColumn)

	content := container.NewBorder(
		topBar,
		nil,
		nil,
		nil,
		split,
	)

	window.SetContent(content)

	const (
		windowWidth   = 1000.0
		windowHeight  = 600.0
		leftMenuWidth = 180.0
	)
	window.Resize(fyne.NewSize(windowWidth, windowHeight))
	split.SetOffset(leftMenuWidth / windowWidth)
	window.SetFixedSize(true)
	window.CenterOnScreen()

	window.ShowAndRun()
}
