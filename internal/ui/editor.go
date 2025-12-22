package ui

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"image/color"
	"log"
	"path/filepath"

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
	cursorTile     map[string]interface{}
	cursorTileText *widget.Text
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
		widget.ContainerOpts.Layout(widget.NewRowLayout(
			widget.RowLayoutOpts.Direction(widget.DirectionVertical),
			widget.RowLayoutOpts.Spacing(10),
			widget.RowLayoutOpts.Padding(widget.NewInsetsSimple(15)),
		)),
	)

	// Small font for this panel
	fontSource, _ := text.NewGoTextFaceSource(bytes.NewReader(goregular.TTF))
	smallGoTextFace := &text.GoTextFace{
		Source: fontSource,
		Size:   9,
	}
	var smallFace text.Face = smallGoTextFace
	smallFontFace := &smallFace

	// Title
	title := widget.NewText(
		widget.TextOpts.Text("Tile Selector", e.fontFace, color.NRGBA{220, 220, 255, 255}),
	)
	panel.AddChild(title)

	// Note about floor mode
	noteText := widget.NewText(
		widget.TextOpts.Text("(Only visible when\nFloor layer is ON)", smallFontFace, color.NRGBA{150, 150, 170, 255}),
	)
	panel.AddChild(noteText)

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

	// Instructions
	instructionsText := widget.NewText(
		widget.TextOpts.Text("\nInstructions:\n• Q: Copy hovered tile\n• Left-click: Place tile\n• R: Rotate tile", smallFontFace, color.NRGBA{120, 120, 140, 255}),
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

	e.setStatus(fmt.Sprintf("Loaded: %s", filename))
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
	e.renderer.Update()

	// Update tile info display
	hoveredTile := e.renderer.GetHoveredTile()
	if hoveredTile != nil {
		info := "strName: " + getStringValue(hoveredTile, "strName") + "\n"
		info += "strID: " + getStringValue(hoveredTile, "strID") + "\n"
		info += fmt.Sprintf("fX: %.2f\n", getFloatValue(hoveredTile, "fX"))
		info += fmt.Sprintf("fY: %.2f\n", getFloatValue(hoveredTile, "fY"))
		info += fmt.Sprintf("fRotation: %.2f", getFloatValue(hoveredTile, "fRotation"))
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

		// Handle Q key to copy tile to cursor
		if inpututil.IsKeyJustPressed(ebiten.KeyQ) {
			e.copyCursorTile(hoveredTile)
		}
	} else {
		e.tileInfoText.Label = "Hover over a tile..."
	}

	// Update cursor tile display
	e.updateCursorTileDisplay()

	// Handle left-click for tile placement
	if e.cursorTile != nil && inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		e.placeTileAtMouse()
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

	// Find the cursor tile's CO definition to copy
	cursorTileID := getStringValue(e.cursorTile, "strID")
	var cursorCO map[string]interface{}
	for _, coInterface := range aCOs {
		if co, ok := coInterface.(map[string]interface{}); ok {
			if strID, ok := co["strID"].(string); ok && strID == cursorTileID {
				cursorCO = co
				break
			}
		}
	}

	if config.Verbose {
		if cursorCO != nil {
			log.Printf("Found cursor tile's CO: strCODef=%s", getStringValue(cursorCO, "strCODef"))
		} else {
			log.Printf("WARNING: Could not find CO for cursor tile ID: %s", cursorTileID)
		}
	}

	// Create new CO based on cursor tile's CO
	if cursorCO != nil {
		newCO := make(map[string]interface{})

		// Copy all CO fields from cursor tile's CO
		for k, v := range cursorCO {
			newCO[k] = v
		}

		// Update the CO's ID to match the new tile
		newCO["strID"] = newGUID

		// If replacing an old tile, find and remove its CO
		if oldTileID != "" {
			removed := false
			for i, coInterface := range aCOs {
				if co, ok := coInterface.(map[string]interface{}); ok {
					if strID, ok := co["strID"].(string); ok && strID == oldTileID {
						// Remove old CO
						if config.Verbose {
							log.Printf("Removed old CO: strCODef=%s, strID=%s", getStringValue(co, "strCODef"), oldTileID)
						}
						aCOs = append(aCOs[:i], aCOs[i+1:]...)
						removed = true
						break
					}
				}
			}
			if config.Verbose && !removed {
				log.Printf("WARNING: Could not find old CO to remove: %s", oldTileID)
			}
		}

		// Add new CO to aCOs
		aCOs = append(aCOs, newCO)
		e.currentShip.RawData["aCOs"] = aCOs

		if config.Verbose {
			log.Printf("Added new CO with ID: %s, strCODef: %s", newGUID, getStringValue(newCO, "strCODef"))
			log.Printf("Total COs: %d", len(aCOs))
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
}
