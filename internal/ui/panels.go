package ui

import (
	"bytes"
	"image/color"
	"math"

	ebitenui_image "github.com/ebitenui/ebitenui/image"
	"github.com/ebitenui/ebitenui/widget"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"golang.org/x/image/font/gofont/goregular"
)

func (e *Editor) createLeftControlPanel() *widget.Container {
	panel := widget.NewContainer(
		widget.ContainerOpts.BackgroundImage(ebitenui_image.NewNineSliceColor(color.NRGBA{35, 35, 45, 255})),
		widget.ContainerOpts.WidgetOpts(widget.WidgetOpts.MinSize(300, 600)),
		widget.ContainerOpts.Layout(widget.NewRowLayout(
			widget.RowLayoutOpts.Direction(widget.DirectionVertical),
			widget.RowLayoutOpts.Spacing(10),
			widget.RowLayoutOpts.Padding(widget.NewInsetsSimple(15)),
		)),
	)

	// Title
	title := widget.NewText(
		widget.TextOpts.Text("Ship Editor", e.fontFace, color.NRGBA{220, 220, 255, 255}),
		widget.TextOpts.Position(widget.TextPositionStart, widget.TextPositionCenter),
	)
	panel.AddChild(title)

	// Load button
	e.loadButton = widget.NewButton(
		widget.ButtonOpts.WidgetOpts(
			widget.WidgetOpts.MinSize(270, 35),
		),
		widget.ButtonOpts.Image(&widget.ButtonImage{
			Idle:    ebitenui_image.NewNineSliceColor(color.NRGBA{70, 100, 150, 255}),
			Hover:   ebitenui_image.NewNineSliceColor(color.NRGBA{90, 120, 170, 255}),
			Pressed: ebitenui_image.NewNineSliceColor(color.NRGBA{50, 80, 130, 255}),
		}),
		widget.ButtonOpts.Text("Browse Ship...", e.fontFace, &widget.ButtonTextColor{
			Idle: color.NRGBA{255, 255, 255, 255},
		}),
		widget.ButtonOpts.TextPadding(widget.NewInsetsSimple(5)),
		widget.ButtonOpts.ClickedHandler(func(args *widget.ButtonClickedEventArgs) {
			e.browseForShip()
		}),
	)
	panel.AddChild(e.loadButton)

	// File path display (smaller font)
	fontSource, _ := text.NewGoTextFaceSource(bytes.NewReader(goregular.TTF))
	smallGoTextFace := &text.GoTextFace{
		Source: fontSource,
		Size:   9,
	}
	var smallFace text.Face = smallGoTextFace
	smallFontFace := &smallFace

	e.filePathText = widget.NewText(
		widget.TextOpts.Text("No file loaded", smallFontFace, color.NRGBA{150, 150, 170, 255}),
		widget.TextOpts.WidgetOpts(
			widget.WidgetOpts.MinSize(270, 20),
		),
	)
	panel.AddChild(e.filePathText)

	// Ship Name and Reg ID side by side
	shipInfoContainer := widget.NewContainer(
		widget.ContainerOpts.Layout(widget.NewGridLayout(
			widget.GridLayoutOpts.Columns(2),
			widget.GridLayoutOpts.Stretch([]bool{true, false}, []bool{true}),
			widget.GridLayoutOpts.Spacing(5, 5),
		)),
	)

	// Ship Name column
	nameContainer := widget.NewContainer(
		widget.ContainerOpts.Layout(widget.NewRowLayout(
			widget.RowLayoutOpts.Direction(widget.DirectionVertical),
			widget.RowLayoutOpts.Spacing(3),
		)),
	)
	nameLabel := widget.NewText(
		widget.TextOpts.Text("Name:", smallFontFace, color.NRGBA{180, 180, 200, 255}),
	)
	nameContainer.AddChild(nameLabel)

	e.nameInput = widget.NewTextInput(
		widget.TextInputOpts.WidgetOpts(
			widget.WidgetOpts.MinSize(160, 25),
		),
		widget.TextInputOpts.Image(&widget.TextInputImage{
			Idle:     ebitenui_image.NewNineSliceColor(color.NRGBA{50, 50, 60, 255}),
			Disabled: ebitenui_image.NewNineSliceColor(color.NRGBA{30, 30, 35, 255}),
		}),
		widget.TextInputOpts.Face(smallFontFace),
		widget.TextInputOpts.Color(&widget.TextInputColor{
			Idle:          color.NRGBA{255, 255, 255, 255},
			Disabled:      color.NRGBA{100, 100, 100, 255},
			Caret:         color.NRGBA{255, 255, 255, 255},
			DisabledCaret: color.NRGBA{100, 100, 100, 255},
		}),
		widget.TextInputOpts.Padding(widget.NewInsetsSimple(3)),
	)
	nameContainer.AddChild(e.nameInput)
	shipInfoContainer.AddChild(nameContainer)

	// Registration ID column
	regIDContainer := widget.NewContainer(
		widget.ContainerOpts.Layout(widget.NewRowLayout(
			widget.RowLayoutOpts.Direction(widget.DirectionVertical),
			widget.RowLayoutOpts.Spacing(3),
		)),
	)
	regIDLabel := widget.NewText(
		widget.TextOpts.Text("Reg:", smallFontFace, color.NRGBA{180, 180, 200, 255}),
	)
	regIDContainer.AddChild(regIDLabel)

	e.regIDInput = widget.NewTextInput(
		widget.TextInputOpts.WidgetOpts(
			widget.WidgetOpts.MinSize(90, 25),
		),
		widget.TextInputOpts.Image(&widget.TextInputImage{
			Idle:     ebitenui_image.NewNineSliceColor(color.NRGBA{50, 50, 60, 255}),
			Disabled: ebitenui_image.NewNineSliceColor(color.NRGBA{30, 30, 35, 255}),
		}),
		widget.TextInputOpts.Face(smallFontFace),
		widget.TextInputOpts.Color(&widget.TextInputColor{
			Idle:          color.NRGBA{255, 255, 255, 255},
			Disabled:      color.NRGBA{100, 100, 100, 255},
			Caret:         color.NRGBA{255, 255, 255, 255},
			DisabledCaret: color.NRGBA{100, 100, 100, 255},
		}),
		widget.TextInputOpts.Padding(widget.NewInsetsSimple(3)),
	)
	regIDContainer.AddChild(e.regIDInput)
	shipInfoContainer.AddChild(regIDContainer)

	panel.AddChild(shipInfoContainer)

	// Layers section
	layersLabel := widget.NewText(
		widget.TextOpts.Text("Layers:", smallFontFace, color.NRGBA{180, 180, 200, 255}),
	)
	panel.AddChild(layersLabel)

	// Layer toggles container (floor and wall side by side)
	layerTogglesContainer := widget.NewContainer(
		widget.ContainerOpts.Layout(widget.NewGridLayout(
			widget.GridLayoutOpts.Columns(2),
			widget.GridLayoutOpts.Stretch([]bool{true, true}, []bool{true}),
			widget.GridLayoutOpts.Spacing(5, 5),
		)),
	)

	// Floor layer toggle button
	e.floorToggle = widget.NewButton(
		widget.ButtonOpts.WidgetOpts(
			widget.WidgetOpts.MinSize(130, 25),
		),
		widget.ButtonOpts.Image(&widget.ButtonImage{
			Idle:    ebitenui_image.NewNineSliceColor(color.NRGBA{70, 150, 70, 255}),
			Hover:   ebitenui_image.NewNineSliceColor(color.NRGBA{90, 170, 90, 255}),
			Pressed: ebitenui_image.NewNineSliceColor(color.NRGBA{50, 130, 50, 255}),
		}),
		widget.ButtonOpts.Text("Floor: ON", smallFontFace, &widget.ButtonTextColor{
			Idle: color.NRGBA{255, 255, 255, 255},
		}),
		widget.ButtonOpts.TextPadding(widget.NewInsetsSimple(3)),
		widget.ButtonOpts.ClickedHandler(func(args *widget.ButtonClickedEventArgs) {
			e.toggleFloor()
		}),
	)
	layerTogglesContainer.AddChild(e.floorToggle)

	// Wall layer toggle button
	e.wallToggle = widget.NewButton(
		widget.ButtonOpts.WidgetOpts(
			widget.WidgetOpts.MinSize(130, 25),
		),
		widget.ButtonOpts.Image(&widget.ButtonImage{
			Idle:    ebitenui_image.NewNineSliceColor(color.NRGBA{150, 100, 70, 255}),
			Hover:   ebitenui_image.NewNineSliceColor(color.NRGBA{170, 120, 90, 255}),
			Pressed: ebitenui_image.NewNineSliceColor(color.NRGBA{130, 80, 50, 255}),
		}),
		widget.ButtonOpts.Text("Wall: ON", smallFontFace, &widget.ButtonTextColor{
			Idle: color.NRGBA{255, 255, 255, 255},
		}),
		widget.ButtonOpts.TextPadding(widget.NewInsetsSimple(3)),
		widget.ButtonOpts.ClickedHandler(func(args *widget.ButtonClickedEventArgs) {
			e.toggleWall()
		}),
	)
	layerTogglesContainer.AddChild(e.wallToggle)

	panel.AddChild(layerTogglesContainer)

	// Save button
	saveButton := widget.NewButton(
		widget.ButtonOpts.WidgetOpts(
			widget.WidgetOpts.MinSize(270, 35),
		),
		widget.ButtonOpts.Image(&widget.ButtonImage{
			Idle:    ebitenui_image.NewNineSliceColor(color.NRGBA{70, 100, 70, 255}),
			Hover:   ebitenui_image.NewNineSliceColor(color.NRGBA{90, 120, 90, 255}),
			Pressed: ebitenui_image.NewNineSliceColor(color.NRGBA{50, 80, 50, 255}),
		}),
		widget.ButtonOpts.Text("Save Ship", e.fontFace, &widget.ButtonTextColor{
			Idle: color.NRGBA{255, 255, 255, 255},
		}),
		widget.ButtonOpts.TextPadding(widget.NewInsetsSimple(5)),
		widget.ButtonOpts.ClickedHandler(func(args *widget.ButtonClickedEventArgs) {
			e.saveShip()
		}),
	)
	panel.AddChild(saveButton)

	// Status text
	e.statusText = widget.NewText(
		widget.TextOpts.Text("", smallFontFace, color.NRGBA{100, 255, 100, 255}),
		widget.TextOpts.WidgetOpts(
			widget.WidgetOpts.MinSize(270, 30),
		),
	)
	panel.AddChild(e.statusText)

	// Tile info section
	tileInfoLabel := widget.NewText(
		widget.TextOpts.Text("Hovered Tile:", smallFontFace, color.NRGBA{180, 180, 200, 255}),
	)
	panel.AddChild(tileInfoLabel)

	e.tileInfoText = widget.NewText(
		widget.TextOpts.Text("Hover over a tile...", smallFontFace, color.NRGBA{150, 150, 170, 255}),
		widget.TextOpts.WidgetOpts(
			widget.WidgetOpts.MinSize(270, 120),
		),
	)
	panel.AddChild(e.tileInfoText)

	return panel
}

func (e *Editor) createRightTilePanel() *widget.Container {
	panel := widget.NewContainer(
		widget.ContainerOpts.BackgroundImage(ebitenui_image.NewNineSliceColor(color.NRGBA{35, 35, 45, 255})),
		widget.ContainerOpts.WidgetOpts(widget.WidgetOpts.MinSize(250, 600)),
		widget.ContainerOpts.Layout(widget.NewGridLayout(
			widget.GridLayoutOpts.Columns(1),
			// Rows: 0=title, 1=cursor label, 2=cursor text, 3=preview, 4=palette label, 5=layer choice, 6=search, 7=scroll, 8=instructions
			widget.GridLayoutOpts.Stretch([]bool{true}, []bool{false, false, false, false, false, false, false, true, false}),
			widget.GridLayoutOpts.Spacing(10, 10),
			widget.GridLayoutOpts.Padding(widget.NewInsetsSimple(15)),
		)),
	)

	// Small font for this panel
	fontSource, _ := text.NewGoTextFaceSource(bytes.NewReader(goregular.TTF))
	smallGoTextFace := &text.GoTextFace{
		Source: fontSource,
		Size:   9,
	}
	var smallFace text.Face = smallGoTextFace
	e.smallFontFace = &smallFace
	smallFontFace := e.smallFontFace

	// Title
	title := widget.NewText(
		widget.TextOpts.Text("Tile Selector", e.fontFace, color.NRGBA{220, 220, 255, 255}),
	)
	panel.AddChild(title)

	// Cursor tile info
	cursorLabel := widget.NewText(
		widget.TextOpts.Text("Current Cursor:", smallFontFace, color.NRGBA{180, 180, 200, 255}),
	)
	panel.AddChild(cursorLabel)

	e.cursorTileText = widget.NewText(
		widget.TextOpts.Text("No tile selected\n\nHover over a tile\nand press Q to copy", smallFontFace, color.NRGBA{150, 150, 170, 255}),
		widget.TextOpts.WidgetOpts(
			widget.WidgetOpts.MinSize(220, 80),
		),
	)
	panel.AddChild(e.cursorTileText)

	// Tile preview container (for rendering the cursor tile image) - smaller now
	e.cursorTilePreview = widget.NewContainer(
		widget.ContainerOpts.BackgroundImage(ebitenui_image.NewNineSliceColor(color.NRGBA{25, 25, 35, 255})),
		widget.ContainerOpts.WidgetOpts(
			widget.WidgetOpts.MinSize(220, 100),
		),
	)
	panel.AddChild(e.cursorTilePreview)

	// Tile Palette section
	paletteLabel := widget.NewText(
		widget.TextOpts.Text("\nTile Palette:", smallFontFace, color.NRGBA{180, 180, 200, 255}),
	)
	panel.AddChild(paletteLabel)

	// Layer choice buttons (Floor/Wall) - shown when both layers are visible
	e.layerChoiceContainer = widget.NewContainer(
		widget.ContainerOpts.Layout(widget.NewGridLayout(
			widget.GridLayoutOpts.Columns(2),
			widget.GridLayoutOpts.Stretch([]bool{true, true}, []bool{true}),
			widget.GridLayoutOpts.Spacing(5, 5),
		)),
	)

	floorChoiceBtn := widget.NewButton(
		widget.ButtonOpts.WidgetOpts(
			widget.WidgetOpts.MinSize(105, 25),
		),
		widget.ButtonOpts.Image(&widget.ButtonImage{
			Idle:    ebitenui_image.NewNineSliceColor(color.NRGBA{70, 150, 70, 255}),
			Hover:   ebitenui_image.NewNineSliceColor(color.NRGBA{90, 170, 90, 255}),
			Pressed: ebitenui_image.NewNineSliceColor(color.NRGBA{50, 130, 50, 255}),
		}),
		widget.ButtonOpts.Text("Floor", smallFontFace, &widget.ButtonTextColor{
			Idle: color.NRGBA{255, 255, 255, 255},
		}),
		widget.ButtonOpts.TextPadding(widget.NewInsetsSimple(3)),
		widget.ButtonOpts.ClickedHandler(func(args *widget.ButtonClickedEventArgs) {
			e.setCurrentLayer("floor")
		}),
	)
	e.layerChoiceContainer.AddChild(floorChoiceBtn)

	wallChoiceBtn := widget.NewButton(
		widget.ButtonOpts.WidgetOpts(
			widget.WidgetOpts.MinSize(105, 25),
		),
		widget.ButtonOpts.Image(&widget.ButtonImage{
			Idle:    ebitenui_image.NewNineSliceColor(color.NRGBA{150, 100, 70, 255}),
			Hover:   ebitenui_image.NewNineSliceColor(color.NRGBA{170, 120, 90, 255}),
			Pressed: ebitenui_image.NewNineSliceColor(color.NRGBA{130, 80, 50, 255}),
		}),
		widget.ButtonOpts.Text("Wall", smallFontFace, &widget.ButtonTextColor{
			Idle: color.NRGBA{255, 255, 255, 255},
		}),
		widget.ButtonOpts.TextPadding(widget.NewInsetsSimple(3)),
		widget.ButtonOpts.ClickedHandler(func(args *widget.ButtonClickedEventArgs) {
			e.setCurrentLayer("wall")
		}),
	)
	e.layerChoiceContainer.AddChild(wallChoiceBtn)

	panel.AddChild(e.layerChoiceContainer)

	// Search input for filtering tiles
	e.tileSearchInput = widget.NewTextInput(
		widget.TextInputOpts.WidgetOpts(
			widget.WidgetOpts.MinSize(220, 25),
		),
		widget.TextInputOpts.Image(&widget.TextInputImage{
			Idle:     ebitenui_image.NewNineSliceColor(color.NRGBA{50, 50, 60, 255}),
			Disabled: ebitenui_image.NewNineSliceColor(color.NRGBA{30, 30, 35, 255}),
		}),
		widget.TextInputOpts.Face(smallFontFace),
		widget.TextInputOpts.Color(&widget.TextInputColor{
			Idle:          color.NRGBA{255, 255, 255, 255},
			Disabled:      color.NRGBA{100, 100, 100, 255},
			Caret:         color.NRGBA{255, 255, 255, 255},
			DisabledCaret: color.NRGBA{100, 100, 100, 255},
		}),
		widget.TextInputOpts.Placeholder("Search tiles..."),
		widget.TextInputOpts.Padding(widget.NewInsetsSimple(3)),
		widget.TextInputOpts.ChangedHandler(func(args *widget.TextInputChangedEventArgs) {
			e.filterTilePalette(args.InputText)
		}),
	)
	panel.AddChild(e.tileSearchInput)

	// Content container for tiles - use GridLayout for 5 columns of tiles
	e.tilePaletteContainer = widget.NewContainer(
		widget.ContainerOpts.Layout(widget.NewGridLayout(
			widget.GridLayoutOpts.Columns(5),
			widget.GridLayoutOpts.Spacing(2, 2),
			widget.GridLayoutOpts.Padding(widget.NewInsetsSimple(5)),
		)),
	)

	// Container to hold scroll container + slider in 2 columns
	scrollWrapper := widget.NewContainer(
		widget.ContainerOpts.Layout(widget.NewGridLayout(
			widget.GridLayoutOpts.Columns(2),
			widget.GridLayoutOpts.Stretch([]bool{true, false}, []bool{true}),
			widget.GridLayoutOpts.Spacing(2, 0),
		)),
	)

	// ScrollContainer
	scrollContainer := widget.NewScrollContainer(
		widget.ScrollContainerOpts.Content(e.tilePaletteContainer),
		widget.ScrollContainerOpts.StretchContentWidth(),
		widget.ScrollContainerOpts.Image(&widget.ScrollContainerImage{
			Idle: ebitenui_image.NewNineSliceColor(color.NRGBA{25, 25, 35, 255}),
			Mask: ebitenui_image.NewNineSliceColor(color.NRGBA{25, 25, 35, 255}),
		}),
	)
	scrollWrapper.AddChild(scrollContainer)

	// Page size function for slider
	pageSizeFunc := func() int {
		return int(math.Round(float64(scrollContainer.ViewRect().Dy())/float64(e.tilePaletteContainer.GetWidget().Rect.Dy())*1000) / 3)
	}

	// Vertical slider for scrolling
	vSlider := widget.NewSlider(
		widget.SliderOpts.Direction(widget.DirectionVertical),
		widget.SliderOpts.MinMax(0, 1000),
		widget.SliderOpts.PageSizeFunc(pageSizeFunc),
		widget.SliderOpts.ChangedHandler(func(args *widget.SliderChangedEventArgs) {
			scrollContainer.ScrollTop = float64(args.Slider.Current) / 1000
		}),
		widget.SliderOpts.Images(
			&widget.SliderTrackImage{
				Idle:  ebitenui_image.NewNineSliceColor(color.NRGBA{40, 40, 50, 255}),
				Hover: ebitenui_image.NewNineSliceColor(color.NRGBA{50, 50, 60, 255}),
			},
			&widget.ButtonImage{
				Idle:    ebitenui_image.NewNineSliceColor(color.NRGBA{70, 70, 80, 255}),
				Hover:   ebitenui_image.NewNineSliceColor(color.NRGBA{80, 80, 90, 255}),
				Pressed: ebitenui_image.NewNineSliceColor(color.NRGBA{60, 60, 70, 255}),
			},
		),
	)

	// Sync slider with scroll events
	scrollContainer.GetWidget().ScrolledEvent.AddHandler(func(args interface{}) {
		if a, ok := args.(*widget.WidgetScrolledEventArgs); ok {
			vSlider.Current -= int(math.Round(a.Y * float64(pageSizeFunc())))
		}
	})

	scrollWrapper.AddChild(vSlider)
	panel.AddChild(scrollWrapper)

	// Instructions
	instructionsText := widget.NewText(
		widget.TextOpts.Text("\nInstructions:\n• Q: Copy hovered tile\n• ESC: Deselect tile\n• Left-click: Place tile\n• R: Rotate tile", smallFontFace, color.NRGBA{120, 120, 140, 255}),
		widget.TextOpts.WidgetOpts(
			widget.WidgetOpts.MinSize(220, 100),
		),
	)
	panel.AddChild(instructionsText)

	return panel
}

func createSmallFont() (*text.Face, error) {
	fontSource, err := text.NewGoTextFaceSource(bytes.NewReader(goregular.TTF))
	if err != nil {
		return nil, err
	}
	smallGoTextFace := &text.GoTextFace{
		Source: fontSource,
		Size:   9,
	}
	var smallFace text.Face = smallGoTextFace
	return &smallFace, nil
}

func createStandardFont() (*text.Face, error) {
	fontSource, err := text.NewGoTextFaceSource(bytes.NewReader(goregular.TTF))
	if err != nil {
		return nil, err
	}
	fontGoTextFace := &text.GoTextFace{
		Source: fontSource,
		Size:   12,
	}
	var fontFace text.Face = fontGoTextFace
	return &fontFace, nil
}
