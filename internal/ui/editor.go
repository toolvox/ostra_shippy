package ui

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"image/color"
	"log"
	"math"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ebitenui/ebitenui"
	"github.com/ebitenui/ebitenui/image"
	"github.com/ebitenui/ebitenui/widget"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/sqweek/dialog"
	"github.com/user/ostra_shippy/internal/config"
	"github.com/user/ostra_shippy/internal/loader"
	"github.com/user/ostra_shippy/internal/models"
	"github.com/user/ostra_shippy/internal/renderer"
	"golang.org/x/image/font/gofont/goregular"
)

type Editor struct {
	UI          *ebitenui.UI
	currentShip *models.Ship
	currentPath string
	fontFace    *text.Face
	renderer    *renderer.ShipRenderer

	// UI widgets
	nameInput    *widget.TextInput
	regIDInput   *widget.TextInput
	statusText   *widget.Text
	filePathText *widget.Text
	loadButton   *widget.Button
	floorToggle  *widget.Button
	showFloor    bool
	tileInfoText *widget.Text
	rKeyPressed  bool

	// Cursor tile for placement
	cursorTile           map[string]interface{}
	cursorTileText       *widget.Text
	cursorTilePreview    *widget.Container
	tilePaletteContainer *widget.Container
	tileSearchInput      *widget.TextInput
	availableTiles       []map[string]interface{}
	filteredTiles        []map[string]interface{}
	smallFontFace        *text.Face

	// Tile palette rendering
	paletteScrollY   int
	paletteMaxScroll int
}

func NewEditor() (*Editor, error) {
	e := &Editor{
		renderer:  renderer.NewShipRenderer(),
		showFloor: true,
	}

	// Load font
	fontSource, err := text.NewGoTextFaceSource(bytes.NewReader(goregular.TTF))
	if err != nil {
		return nil, fmt.Errorf("failed to load font: %w", err)
	}
	goTextFace := &text.GoTextFace{
		Source: fontSource,
		Size:   14,
	}
	var face text.Face = goTextFace
	e.fontFace = &face

	// Build tile palette at startup
	e.buildTilePalette()

	// Create UI
	e.createUI()

	return e, nil
}

func (e *Editor) createUI() {
	// Main container with horizontal layout (left panel | center viewport | right panel)
	rootContainer := widget.NewContainer(
		widget.ContainerOpts.BackgroundImage(image.NewNineSliceColor(color.NRGBA{20, 20, 30, 255})),
		widget.ContainerOpts.Layout(widget.NewGridLayout(
			widget.GridLayoutOpts.Columns(3),
			widget.GridLayoutOpts.Stretch([]bool{false, true, false}, []bool{true}),
			widget.GridLayoutOpts.Spacing(0, 0),
		)),
	)

	// Left control panel (300px wide)
	leftPanel := e.createLeftControlPanel()
	rootContainer.AddChild(leftPanel)

	// Center viewport (ship rendering area - handled in Draw)
	centerPanel := widget.NewContainer(
		widget.ContainerOpts.BackgroundImage(image.NewNineSliceColor(color.NRGBA{15, 15, 20, 255})),
		widget.ContainerOpts.WidgetOpts(widget.WidgetOpts.MinSize(400, 600)),
	)
	rootContainer.AddChild(centerPanel)

	// Right tile selector panel (250px wide)
	rightPanel := e.createRightTilePanel()
	rootContainer.AddChild(rightPanel)

	e.UI = &ebitenui.UI{
		Container: rootContainer,
	}
}

func (e *Editor) createLeftControlPanel() *widget.Container {
	panel := widget.NewContainer(
		widget.ContainerOpts.BackgroundImage(image.NewNineSliceColor(color.NRGBA{35, 35, 45, 255})),
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
			Idle:    image.NewNineSliceColor(color.NRGBA{70, 100, 150, 255}),
			Hover:   image.NewNineSliceColor(color.NRGBA{90, 120, 170, 255}),
			Pressed: image.NewNineSliceColor(color.NRGBA{50, 80, 130, 255}),
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
			Idle:     image.NewNineSliceColor(color.NRGBA{50, 50, 60, 255}),
			Disabled: image.NewNineSliceColor(color.NRGBA{30, 30, 35, 255}),
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
			Idle:     image.NewNineSliceColor(color.NRGBA{50, 50, 60, 255}),
			Disabled: image.NewNineSliceColor(color.NRGBA{30, 30, 35, 255}),
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

	// Floor layer toggle button
	e.floorToggle = widget.NewButton(
		widget.ButtonOpts.WidgetOpts(
			widget.WidgetOpts.MinSize(120, 25),
		),
		widget.ButtonOpts.Image(&widget.ButtonImage{
			Idle:    image.NewNineSliceColor(color.NRGBA{70, 150, 70, 255}),
			Hover:   image.NewNineSliceColor(color.NRGBA{90, 170, 90, 255}),
			Pressed: image.NewNineSliceColor(color.NRGBA{50, 130, 50, 255}),
		}),
		widget.ButtonOpts.Text("Floor: ON", smallFontFace, &widget.ButtonTextColor{
			Idle: color.NRGBA{255, 255, 255, 255},
		}),
		widget.ButtonOpts.TextPadding(widget.NewInsetsSimple(3)),
		widget.ButtonOpts.ClickedHandler(func(args *widget.ButtonClickedEventArgs) {
			e.toggleFloor()
		}),
	)
	panel.AddChild(e.floorToggle)

	// Save button
	saveButton := widget.NewButton(
		widget.ButtonOpts.WidgetOpts(
			widget.WidgetOpts.MinSize(270, 35),
		),
		widget.ButtonOpts.Image(&widget.ButtonImage{
			Idle:    image.NewNineSliceColor(color.NRGBA{70, 100, 70, 255}),
			Hover:   image.NewNineSliceColor(color.NRGBA{90, 120, 90, 255}),
			Pressed: image.NewNineSliceColor(color.NRGBA{50, 80, 50, 255}),
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
		widget.ContainerOpts.BackgroundImage(image.NewNineSliceColor(color.NRGBA{35, 35, 45, 255})),
		widget.ContainerOpts.WidgetOpts(widget.WidgetOpts.MinSize(250, 600)),
		widget.ContainerOpts.Layout(widget.NewGridLayout(
			widget.GridLayoutOpts.Columns(1),
			// Rows: 0=title, 1=cursor label, 2=cursor text, 3=preview, 4=palette label, 5=search, 6=scroll, 7=instructions
			widget.GridLayoutOpts.Stretch([]bool{true}, []bool{false, false, false, false, false, false, true, false}),
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
		widget.ContainerOpts.BackgroundImage(image.NewNineSliceColor(color.NRGBA{25, 25, 35, 255})),
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

	// Search input for filtering tiles
	e.tileSearchInput = widget.NewTextInput(
		widget.TextInputOpts.WidgetOpts(
			widget.WidgetOpts.MinSize(220, 25),
		),
		widget.TextInputOpts.Image(&widget.TextInputImage{
			Idle:     image.NewNineSliceColor(color.NRGBA{50, 50, 60, 255}),
			Disabled: image.NewNineSliceColor(color.NRGBA{30, 30, 35, 255}),
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
			Idle: image.NewNineSliceColor(color.NRGBA{25, 25, 35, 255}),
			Mask: image.NewNineSliceColor(color.NRGBA{25, 25, 35, 255}),
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
				Idle:  image.NewNineSliceColor(color.NRGBA{40, 40, 50, 255}),
				Hover: image.NewNineSliceColor(color.NRGBA{50, 50, 60, 255}),
			},
			&widget.ButtonImage{
				Idle:    image.NewNineSliceColor(color.NRGBA{70, 70, 80, 255}),
				Hover:   image.NewNineSliceColor(color.NRGBA{80, 80, 90, 255}),
				Pressed: image.NewNineSliceColor(color.NRGBA{60, 60, 70, 255}),
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

func (e *Editor) browseForShip() {
	filename, err := dialog.File().Filter("JSON files", "json").Load()
	if err != nil {
		if err.Error() != "Cancelled" {
			e.setStatus(fmt.Sprintf("Error: %v", err))
		}
		return
	}

	e.loadShip(filename)
}

func (e *Editor) loadShip(path string) {
	ship, err := loader.LoadShip(path)
	if err != nil {
		e.setStatus(fmt.Sprintf("Error: %v", err))
		return
	}

	e.currentShip = ship
	e.currentPath = path

	// Update UI - use PublicName for the name field
	e.nameInput.SetText(ship.PublicName)
	e.regIDInput.SetText(ship.StrRegID)

	// Show only filename in UI, print full path to stdout
	filename := filepath.Base(path)
	e.filePathText.Label = filename
	fmt.Printf("Loaded ship: %s\n", path)

	// Rebuild tile palette UI now that we have a ship loaded
	e.rebuildTilePaletteUI()

	e.setStatus(fmt.Sprintf("Loaded: %s", filename))
}

func (e *Editor) buildTilePalette() {
	// Build tile palette from cooverlays_floors.json in the data directory
	dataPath := ""
	if config.GlobalConfig != nil {
		dataPath = config.GlobalConfig.Data
	}

	if dataPath == "" {
		log.Printf("Warning: Data path not set, cannot build tile palette")
		return
	}

	jsonPath := filepath.Join(dataPath, "cooverlays", "cooverlays_floors.json")

	// Load JSON file
	jsonData, err := loader.LoadJSON(jsonPath)
	if err != nil {
		log.Printf("Error loading cooverlays_floors.json: %v", err)
		return
	}

	// Parse the JSON array
	tileDefinitions, ok := jsonData.([]interface{})
	if !ok {
		log.Printf("Error: cooverlays_floors.json is not an array")
		return
	}

	e.availableTiles = make([]map[string]interface{}, 0)

	for _, tileDefInterface := range tileDefinitions {
		tileDef, ok := tileDefInterface.(map[string]interface{})
		if !ok {
			continue
		}

		strName := getStringValue(tileDef, "strName")

		// Only include floor tiles
		if len(strName) < 8 || strName[:8] != "ItmFloor" {
			continue
		}

		// Filter out damaged, patched, and loose variants
		if strings.HasSuffix(strName, "Dmg") ||
			strings.HasSuffix(strName, "Patch") ||
			strings.HasSuffix(strName, "Loose") {
			continue
		}

		// Create a tile template from the definition
		tile := make(map[string]interface{})
		tile["strName"] = strName
		tile["fX"] = 0.0
		tile["fY"] = 0.0
		tile["fRotation"] = 0.0
		tile["strID"] = e.generateGUID() // Generate a temporary ID

		// Store the full CO definition from JSON for later use when placing tiles
		tile["_coDefinition"] = tileDef

		// Store additional info for later use (like image path)
		if strImg, ok := tileDef["strImg"].(string); ok {
			tile["strImg"] = strImg
		}
		if strNameFriendly, ok := tileDef["strNameFriendly"].(string); ok {
			tile["strNameFriendly"] = strNameFriendly
		}

		e.availableTiles = append(e.availableTiles, tile)
	}

	// Sort tiles by name
	sort.Slice(e.availableTiles, func(i, j int) bool {
		return getStringValue(e.availableTiles[i], "strName") < getStringValue(e.availableTiles[j], "strName")
	})

	if config.Verbose {
		log.Printf("Built tile palette with %d floor tiles from %s", len(e.availableTiles), jsonPath)
	}

	// Initialize filtered tiles
	e.filterTilePalette("")
}

func (e *Editor) filterTilePalette(searchText string) {
	if e.availableTiles == nil {
		return
	}

	searchText = strings.ToLower(searchText)

	if searchText == "" {
		e.filteredTiles = e.availableTiles
	} else {
		e.filteredTiles = make([]map[string]interface{}, 0)
		for _, tile := range e.availableTiles {
			tileName := strings.ToLower(getStringValue(tile, "strName"))
			if strings.Contains(tileName, searchText) {
				e.filteredTiles = append(e.filteredTiles, tile)
			}
		}
	}

	// Rebuild the tile palette UI
	e.rebuildTilePaletteUI()
}

func (e *Editor) rebuildTilePaletteUI() {
	if e.tilePaletteContainer == nil {
		return
	}

	// Clear existing widgets
	e.tilePaletteContainer.RemoveChildren()

	// Add all filtered tiles as buttons in a 5-column grid
	for _, tile := range e.filteredTiles {
		tileName := getStringValue(tile, "strName")
		// Use the strImg field from the preloaded tile data
		imagePath := getStringValue(tile, "strImg")
		if imagePath == "" {
			imagePath = tileName
		}
		tileImg := e.renderer.LoadTileImage(imagePath)

		// Create button with tile image
		btn := widget.NewButton(
			widget.ButtonOpts.WidgetOpts(
				widget.WidgetOpts.MinSize(40, 40),
			),
			widget.ButtonOpts.Image(&widget.ButtonImage{
				Idle:    image.NewNineSliceColor(color.NRGBA{50, 50, 60, 255}),
				Hover:   image.NewNineSliceColor(color.NRGBA{70, 100, 150, 255}),
				Pressed: image.NewNineSliceColor(color.NRGBA{60, 90, 130, 255}),
			}),
			widget.ButtonOpts.Graphic(&widget.GraphicImage{
				Idle: tileImg,
			}),
			widget.ButtonOpts.GraphicPadding(widget.Insets{Top: 4, Bottom: 4, Left: 4, Right: 4}),
			widget.ButtonOpts.ClickedHandler(func(args *widget.ButtonClickedEventArgs) {
				e.selectTileFromPalette(tile)
			}),
		)

		e.tilePaletteContainer.AddChild(btn)
	}
}

func (e *Editor) selectTileFromPalette(tile map[string]interface{}) {
	// Deep copy the tile
	e.cursorTile = make(map[string]interface{})
	for k, v := range tile {
		e.cursorTile[k] = v
	}
	e.setStatus(fmt.Sprintf("Selected from palette: %s", getStringValue(tile, "strName")))
}

func (e *Editor) saveShip() {
	if e.currentShip == nil || e.currentPath == "" {
		e.setStatus("No ship loaded!")
		return
	}

	// Update ship with edited values
	e.currentShip.PublicName = e.nameInput.GetText()
	e.currentShip.StrRegID = e.regIDInput.GetText()

	// Clean up duplicate floor tiles before saving
	e.removeDuplicateFloorTiles()

	if err := loader.SaveShip(e.currentPath, e.currentShip); err != nil {
		e.setStatus(fmt.Sprintf("Error: %v", err))
		return
	}

	e.setStatus(fmt.Sprintf("Saved: %s", filepath.Base(e.currentPath)))
}

func (e *Editor) removeDuplicateFloorTiles() {
	if e.currentShip == nil {
		return
	}

	aItems, ok := e.currentShip.RawData["aItems"].([]interface{})
	if !ok {
		return
	}

	aCOs, ok := e.currentShip.RawData["aCOs"].([]interface{})
	if !ok {
		return
	}

	// Track tiles by position (x,y) -> list of tile indices
	type posKey struct {
		x, y float64
	}
	positionMap := make(map[posKey][]int)

	// Build map of positions to tile indices (floor tiles only)
	for i, itemInterface := range aItems {
		if item, ok := itemInterface.(map[string]interface{}); ok {
			strName := getStringValue(item, "strName")

			// Only process floor tiles (ItmFloor*)
			if len(strName) < 8 || strName[:8] != "ItmFloor" {
				continue
			}

			// Skip loose items (they can stack)
			if len(strName) > 5 && strName[len(strName)-5:] == "Loose" {
				continue
			}

			x := getFloatValue(item, "fX")
			y := getFloatValue(item, "fY")
			key := posKey{x, y}
			positionMap[key] = append(positionMap[key], i)
		}
	}

	// Find duplicates and collect indices to remove
	indicesToRemove := make(map[int]bool)
	removedCOIDs := make(map[string]bool)

	for pos, indices := range positionMap {
		if len(indices) > 1 {
			if config.Verbose {
				log.Printf("Found %d tiles at position (%.0f, %.0f), keeping last one", len(indices), pos.x, pos.y)
			}
			// Keep the last tile (highest index), remove all others
			for i := 0; i < len(indices)-1; i++ {
				idx := indices[i]
				indicesToRemove[idx] = true

				// Mark this tile's CO for removal
				if item, ok := aItems[idx].(map[string]interface{}); ok {
					if strID, ok := item["strID"].(string); ok {
						removedCOIDs[strID] = true
						if config.Verbose {
							log.Printf("  Removing duplicate: %s (ID: %s) at index %d",
								getStringValue(item, "strName"), strID, idx)
						}
					}
				}
			}
		}
	}

	if len(indicesToRemove) == 0 {
		if config.Verbose {
			log.Printf("No duplicate floor tiles found")
		}
		return
	}

	// Remove tiles (backwards to maintain indices)
	newItems := make([]interface{}, 0, len(aItems)-len(indicesToRemove))
	for i, item := range aItems {
		if !indicesToRemove[i] {
			newItems = append(newItems, item)
		}
	}
	e.currentShip.RawData["aItems"] = newItems

	// Remove corresponding COs
	newCOs := make([]interface{}, 0)
	removedCount := 0
	for _, coInterface := range aCOs {
		if co, ok := coInterface.(map[string]interface{}); ok {
			if strID, ok := co["strID"].(string); ok {
				if removedCOIDs[strID] {
					removedCount++
					continue
				}
			}
		}
		newCOs = append(newCOs, coInterface)
	}
	e.currentShip.RawData["aCOs"] = newCOs

	if config.Verbose {
		log.Printf("Removed %d duplicate tiles and %d corresponding COs", len(indicesToRemove), removedCount)
		log.Printf("Total tiles: %d -> %d", len(aItems), len(newItems))
		log.Printf("Total COs: %d -> %d", len(aCOs), len(newCOs))
	}

	e.setStatus(fmt.Sprintf("Cleaned up %d duplicate tiles", len(indicesToRemove)))
}

func (e *Editor) setStatus(msg string) {
	e.statusText.Label = msg
}

func (e *Editor) toggleFloor() {
	e.showFloor = !e.showFloor
	e.renderer.SetShowFloor(e.showFloor)

	// Update button text
	if e.showFloor {
		e.floorToggle.Text().Label = "Floor: ON"
	} else {
		e.floorToggle.Text().Label = "Floor: OFF"
	}
}

func (e *Editor) Update() {
	e.UI.Update()

	// Set paint mode based on whether cursor tile is selected
	e.renderer.SetPaintMode(e.cursorTile != nil)

	e.renderer.Update()

	// Update tile info display
	hoveredTile := e.renderer.GetHoveredTile()

	// Handle Q key to copy hovered tile to cursor
	if inpututil.IsKeyJustPressed(ebiten.KeyQ) && hoveredTile != nil {
		e.copyCursorTile(hoveredTile)
	}

	// Handle ESC key to deselect cursor tile
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) && e.cursorTile != nil {
		e.cursorTile = nil
		e.setStatus("Tile deselected")
	}

	if hoveredTile != nil {
		info := "strName: " + getStringValue(hoveredTile, "strName") + "\n"
		info += "strID: " + getStringValue(hoveredTile, "strID") + "\n"
		info += fmt.Sprintf("fX: %.2f\n", getFloatValue(hoveredTile, "fX"))
		info += fmt.Sprintf("fY: %.2f\n", getFloatValue(hoveredTile, "fY"))
		info += fmt.Sprintf("fRotation: %.2f\n", getFloatValue(hoveredTile, "fRotation"))

		// Add CO information if available
		if e.currentShip != nil {
			tileID := getStringValue(hoveredTile, "strID")
			if aCOs, ok := e.currentShip.RawData["aCOs"].([]interface{}); ok {
				for _, coInterface := range aCOs {
					if co, ok := coInterface.(map[string]interface{}); ok {
						if coID, ok := co["strID"].(string); ok && coID == tileID {
							info += "\nCO Data:\n"
							if strIMGPreview := getStringValue(co, "strIMGPreview"); strIMGPreview != "" {
								info += "strIMGPreview: " + strIMGPreview + "\n"
							}
							if strCODef := getStringValue(co, "strCODef"); strCODef != "" {
								info += "strCODef: " + strCODef + "\n"
							}
							if strCOBase := getStringValue(co, "strCOBase"); strCOBase != "" {
								info += "strCOBase: " + strCOBase
							}
							break
						}
					}
				}
			}
		}

		e.tileInfoText.Label = info

		// Handle R key to rotate tile
		if inpututil.IsKeyJustPressed(ebiten.KeyR) {
			currentRotation := getFloatValue(hoveredTile, "fRotation")
			newRotation := currentRotation + 90.0
			if newRotation >= 360.0 {
				newRotation -= 360.0
			}
			hoveredTile["fRotation"] = newRotation
			e.setStatus(fmt.Sprintf("Rotated tile to %.0f°", newRotation))
		}
	} else {
		e.tileInfoText.Label = "Hover over a tile..."
	}

	// Update cursor tile display
	e.updateCursorTileDisplay()

	// Handle tile placement: left-click or hovering while holding left button in paint mode
	if e.cursorTile != nil {
		if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
			e.placeTileAtMouse()
		} else if ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) && hoveredTile != nil {
			// Paint continuously while dragging in paint mode
			e.placeTileAtMouse()
		}
	}
}

func (e *Editor) copyCursorTile(tile map[string]interface{}) {
	// Deep copy the tile data
	e.cursorTile = make(map[string]interface{})
	for k, v := range tile {
		e.cursorTile[k] = v
	}
	e.setStatus(fmt.Sprintf("Copied tile: %s", getStringValue(tile, "strName")))
}

func (e *Editor) updateCursorTileDisplay() {
	if e.cursorTile == nil {
		e.cursorTileText.Label = "No tile selected\n\nHover over a tile\nand press Q to copy"
		return
	}

	info := "strName: " + getStringValue(e.cursorTile, "strName") + "\n"
	info += "strID: " + getStringValue(e.cursorTile, "strID") + "\n"
	info += fmt.Sprintf("Rotation: %.0f°", getFloatValue(e.cursorTile, "fRotation"))
	e.cursorTileText.Label = info
}

func (e *Editor) placeTileAtMouse() {
	if e.currentShip == nil {
		return
	}

	// Get mouse grid coordinates (cursor snap position)
	gridX, gridY, valid := e.renderer.GetMouseGridPos()
	if !valid {
		if config.Verbose {
			log.Printf("No valid cursor grid position, placement cancelled")
		}
		return
	}

	// Get the hovered tile to know which specific tile to replace
	hoveredTile := e.renderer.GetHoveredTile()
	if hoveredTile == nil {
		if config.Verbose {
			log.Printf("No hovered tile, placement cancelled")
		}
		return
	}

	if config.Verbose {
		hoveredX := getFloatValue(hoveredTile, "fX")
		hoveredY := getFloatValue(hoveredTile, "fY")
		hoveredName := getStringValue(hoveredTile, "strName")
		hoveredID := getStringValue(hoveredTile, "strID")
		log.Printf("Hovered tile: %s (ID: %s) at (%.0f, %.0f)", hoveredName, hoveredID, hoveredX, hoveredY)
		log.Printf("Cursor grid position: (%.0f, %.0f)", gridX, gridY)
	}

	// Replace the specific hovered tile (use its ID to ensure we replace the right one)
	hoveredTileID := getStringValue(hoveredTile, "strID")
	e.replaceTileAt(gridX, gridY, hoveredTileID)
}

func (e *Editor) replaceTileAt(tileX, tileY float64, targetTileID string) {
	if e.currentShip == nil || e.cursorTile == nil {
		if config.Verbose {
			log.Printf("replaceTileAt failed: currentShip=%v, cursorTile=%v", e.currentShip != nil, e.cursorTile != nil)
		}
		return
	}

	if config.Verbose {
		log.Printf("=== Tile Placement: %s at (%.0f, %.0f) ===",
			getStringValue(e.cursorTile, "strName"), tileX, tileY)
	}

	// Get aItems array
	aItems, ok := e.currentShip.RawData["aItems"].([]interface{})
	if !ok {
		e.setStatus("Error: aItems not found")
		if config.Verbose {
			log.Printf("ERROR: aItems array not found in ship data")
		}
		return
	}

	// Get aCOs array
	aCOs, ok := e.currentShip.RawData["aCOs"].([]interface{})
	if !ok {
		e.setStatus("Error: aCOs not found")
		if config.Verbose {
			log.Printf("ERROR: aCOs array not found in ship data")
		}
		return
	}

	// Find the specific tile by ID (to handle multiple tiles at same position)
	tileIndex := -1
	var oldTileID string
	var oldTileName string
	for i, itemInterface := range aItems {
		if item, ok := itemInterface.(map[string]interface{}); ok {
			if strID, ok := item["strID"].(string); ok && strID == targetTileID {
				tileIndex = i
				oldTileID = strID
				if strName, ok := item["strName"].(string); ok {
					oldTileName = strName
				}
				if config.Verbose {
					log.Printf("Found tile to replace by ID: strName=%s, strID=%s at index %d", oldTileName, oldTileID, i)
				}
				break
			}
		}
	}

	if config.Verbose && tileIndex < 0 {
		log.Printf("WARNING: Could not find tile with ID %s, placement cancelled", targetTileID)
	}

	if tileIndex < 0 {
		e.setStatus("Error: Could not find tile to replace")
		return
	}

	// Create new tile with only the necessary fields
	newTile := make(map[string]interface{})

	// Copy tile type
	if strName, ok := e.cursorTile["strName"].(string); ok {
		newTile["strName"] = strName
	}

	// Set position
	newTile["fX"] = tileX
	newTile["fY"] = tileY

	// Copy rotation from cursor tile
	if fRotation, ok := e.cursorTile["fRotation"].(float64); ok {
		newTile["fRotation"] = fRotation
	} else {
		newTile["fRotation"] = 0.0
	}

	// Generate new unique GUID for both the tile and its CO
	newGUID := e.generateGUID()
	newTile["strID"] = newGUID

	// Copy parent slot if it exists
	if strSlotParentID, ok := e.cursorTile["strSlotParentID"].(string); ok {
		newTile["strSlotParentID"] = strSlotParentID
	}

	// Copy strImg if it exists (for palette tiles)
	if strImg, ok := e.cursorTile["strImg"].(string); ok && strImg != "" {
		newTile["strImg"] = strImg
	}

	// Find the old tile's CO and update it in place
	var targetCO map[string]interface{}

	// Find the CO that belongs to the old tile we're replacing
	if oldTileID != "" {
		for _, coInterface := range aCOs {
			if co, ok := coInterface.(map[string]interface{}); ok {
				if strID, ok := co["strID"].(string); ok && strID == oldTileID {
					targetCO = co
					if config.Verbose {
						log.Printf("Found old CO to update in place: strID=%s", oldTileID)
					}
					break
				}
			}
		}
	}

	// If we found the old CO, update it with the cursor tile's data
	if targetCO != nil {
		// Get the cursor tile's CO definition (from palette or existing tile)
		cursorTileID := getStringValue(e.cursorTile, "strID")
		var cursorCO map[string]interface{}

		// Try to find cursor tile's CO in ship's aCOs
		for _, coInterface := range aCOs {
			if co, ok := coInterface.(map[string]interface{}); ok {
				if strID, ok := co["strID"].(string); ok && strID == cursorTileID {
					cursorCO = co
					break
				}
			}
		}

		// Update the old CO with cursor tile's data
		if cursorCO != nil {
			// Copy all fields from cursor tile's CO except strID
			for k, v := range cursorCO {
				if k != "strID" {
					targetCO[k] = v
				}
			}
			if config.Verbose {
				log.Printf("Updated CO in place with cursor tile data")
			}
		} else {
			// Cursor tile is from palette, use the stored CO definition from JSON
			if coDefinition, ok := e.cursorTile["_coDefinition"].(map[string]interface{}); ok {
				// Only update specific fields that should change, not all fields from the overlay definition
				// The overlay JSON has different structure than ship COs

				// Update strCODef if it exists in the definition
				if strCODef, ok := coDefinition["strName"].(string); ok {
					targetCO["strCODef"] = strCODef
				}

				// Update image paths
				if strImg, ok := coDefinition["strImg"].(string); ok {
					targetCO["strIMGPreview"] = strImg
				}

				if config.Verbose {
					log.Printf("Updated CO in place with palette tile: strCODef=%s", getStringValue(targetCO, "strCODef"))
				}
			} else {
				// Fallback: just update image if no CO definition available
				if strImg, ok := e.cursorTile["strImg"].(string); ok && strImg != "" {
					targetCO["strIMGPreview"] = strImg
				}
				if config.Verbose {
					log.Printf("Updated CO in place with palette tile image only: %s", getStringValue(e.cursorTile, "strImg"))
				}
			}
		}

		// Update the CO's ID to match the new tile
		targetCO["strID"] = newGUID

		if config.Verbose {
			log.Printf("Updated CO ID from %s to %s", oldTileID, newGUID)
		}
	} else {
		if config.Verbose {
			log.Printf("WARNING: Could not find old CO to update for tile: %s", oldTileID)
		}
	}

	// Replace or append tile in aItems
	if tileIndex >= 0 {
		aItems[tileIndex] = newTile
		e.setStatus(fmt.Sprintf("Replaced tile at (%.0f, %.0f)", tileX, tileY))
		if config.Verbose {
			log.Printf("Tile placement complete: new GUID=%s", newGUID)
		}
	} else {
		aItems = append(aItems, newTile)
		e.currentShip.RawData["aItems"] = aItems
		e.setStatus(fmt.Sprintf("Placed tile at (%.0f, %.0f)", tileX, tileY))
		if config.Verbose {
			log.Printf("Tile placement complete: new GUID=%s, total items=%d", newGUID, len(aItems))
		}
	}
}

func (e *Editor) generateGUID() string {
	// Simple GUID generator (format: xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx)
	b := make([]byte, 16)
	rand.Read(b)

	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

func getStringValue(m map[string]interface{}, key string) string {
	if val, ok := m[key].(string); ok {
		return val
	}
	return ""
}

func getFloatValue(m map[string]interface{}, key string) float64 {
	if val, ok := m[key].(float64); ok {
		return val
	}
	return 0.0
}

func (e *Editor) Draw(screen *ebiten.Image) {
	e.UI.Draw(screen)

	// Render ship visualization in the center panel (between left and right panels)
	// Left panel: 300px, Center starts at 300, Right panel: 250px
	// Screen width assumed to be at least 1000px (300 + 450 + 250)
	screenWidth, screenHeight := screen.Bounds().Dx(), screen.Bounds().Dy()
	centerX := float32(300)
	centerY := float32(0)
	centerWidth := float32(screenWidth - 300 - 250) // Subtract both panels
	centerHeight := float32(screenHeight)

	if e.currentShip != nil {
		e.renderer.Render(screen, e.currentShip, centerX, centerY, centerWidth, centerHeight, e.cursorTile)
	}

	// Render cursor tile preview in the right panel
	e.renderCursorTilePreview(screen)
}

func (e *Editor) renderCursorTilePreview(screen *ebiten.Image) {
	if e.cursorTile == nil {
		return
	}

	// Get the preview container's bounds
	rect := e.cursorTilePreview.GetWidget().Rect

	// Get tile image using the strImg field from the preloaded tile data
	cursorName := getStringValue(e.cursorTile, "strName")
	imagePath := getStringValue(e.cursorTile, "strImg")
	if imagePath == "" {
		imagePath = cursorName
	}
	tileImg := e.renderer.LoadTileImage(imagePath)

	if tileImg == nil {
		return
	}

	// Get cursor rotation
	cursorRotation := getFloatValue(e.cursorTile, "fRotation")

	// Calculate centered position within the preview container
	containerW := float32(rect.Dx())
	containerH := float32(rect.Dy())
	imgW := float32(tileImg.Bounds().Dx())
	imgH := float32(tileImg.Bounds().Dy())

	// Scale to fit container while maintaining aspect ratio (max 80x80)
	maxSize := float32(80.0)
	scale := maxSize / imgW
	if imgH > imgW {
		scale = maxSize / imgH
	}

	scaledW := imgW * scale
	scaledH := imgH * scale

	// Center in container
	offsetX := float32(rect.Min.X) + (containerW-scaledW)/2
	offsetY := float32(rect.Min.Y) + (containerH-scaledH)/2

	// Draw the tile image with rotation
	opts := &ebiten.DrawImageOptions{}
	opts.GeoM.Scale(float64(scale), float64(scale))

	// Apply rotation around center
	opts.GeoM.Translate(-float64(scaledW)/2, -float64(scaledH)/2)
	opts.GeoM.Rotate(cursorRotation * 3.14159265 / 180.0)
	opts.GeoM.Translate(float64(scaledW)/2, float64(scaledH)/2)

	opts.GeoM.Translate(float64(offsetX), float64(offsetY))

	screen.DrawImage(tileImg, opts)
}

func (e *Editor) getTileImagePath(tile map[string]interface{}, coMap map[string]map[string]interface{}, fallbackName string) string {
	var imagePath string

	// First, try to get strImg from the tile itself (from JSON definition)
	if strImg, ok := tile["strImg"].(string); ok && strImg != "" {
		imagePath = strImg
	}

	// Then try to get strIMGPreview from the CO
	if imagePath == "" {
		if strID, ok := tile["strID"].(string); ok {
			if co, ok := coMap[strID]; ok {
				if strIMGPreview, ok := co["strIMGPreview"].(string); ok {
					imagePath = strIMGPreview
				}
			}
		}
	}

	// Fallback to provided name if no image path found
	if imagePath == "" {
		imagePath = fallbackName
	}

	return imagePath
}
