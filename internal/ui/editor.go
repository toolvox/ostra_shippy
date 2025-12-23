package ui

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"math/rand"

	"github.com/ebitenui/ebitenui"
	ebitenui_image "github.com/ebitenui/ebitenui/image"
	"github.com/ebitenui/ebitenui/widget"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/user/ostra_shippy/internal/models"
	"github.com/user/ostra_shippy/internal/renderer"
	"github.com/user/ostra_shippy/internal/utils"
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
	wallToggle   *widget.Button
	showFloor    bool
	showWall     bool
	tileInfoText *widget.Text
	rKeyPressed  bool
	currentLayer string // "floor" or "wall"

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
	paletteScrollY       int
	paletteMaxScroll     int
	availableWallTiles   []map[string]interface{}
	filteredWallTiles    []map[string]interface{}
	layerChoiceContainer *widget.Container
}

func NewEditor() (*Editor, error) {
	e := &Editor{
		renderer:     renderer.NewShipRenderer(),
		showFloor:    true,
		showWall:     true,
		currentLayer: "floor",
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

	// Build tile palettes at startup
	e.buildFloorPalette()
	e.buildWallPalette()

	// Create UI
	e.createUI()

	return e, nil
}

func (e *Editor) createUI() {
	// Main container with horizontal layout (left panel | center viewport | right panel)
	rootContainer := widget.NewContainer(
		widget.ContainerOpts.BackgroundImage(ebitenui_image.NewNineSliceColor(color.NRGBA{20, 20, 30, 255})),
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
		widget.ContainerOpts.BackgroundImage(ebitenui_image.NewNineSliceColor(color.NRGBA{15, 15, 20, 255})),
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
		// If floor is turned off and current layer is floor, switch to wall
		if e.currentLayer == "floor" && e.showWall {
			e.setCurrentLayer("wall")
		}
	}

	// Update layer choice visibility
	e.updateLayerChoiceVisibility()
	e.rebuildTilePaletteUI()
}

func (e *Editor) toggleWall() {
	e.showWall = !e.showWall
	e.renderer.SetShowWall(e.showWall)

	// Update button text
	if e.showWall {
		e.wallToggle.Text().Label = "Wall: ON"
	} else {
		e.wallToggle.Text().Label = "Wall: OFF"
		// If wall is turned off and current layer is wall, switch to floor
		if e.currentLayer == "wall" && e.showFloor {
			e.setCurrentLayer("floor")
		}
	}

	// Update layer choice visibility
	e.updateLayerChoiceVisibility()
	e.rebuildTilePaletteUI()
}

func (e *Editor) setCurrentLayer(layer string) {
	e.currentLayer = layer
	e.setStatus(fmt.Sprintf("Switched to %s layer", layer))
	e.rebuildTilePaletteUI()
}

func (e *Editor) updateLayerChoiceVisibility() {
	if e.layerChoiceContainer == nil {
		return
	}

	// Show layer choice buttons only when both layers are visible
	if e.showFloor && e.showWall {
		e.layerChoiceContainer.GetWidget().Visibility = widget.Visibility_Show
	} else {
		e.layerChoiceContainer.GetWidget().Visibility = widget.Visibility_Hide
		// Auto-select the visible layer
		if e.showFloor && !e.showWall {
			e.currentLayer = "floor"
		} else if e.showWall && !e.showFloor {
			e.currentLayer = "wall"
		}
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

func (e *Editor) generateGUID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// Convenience wrappers for utils functions
func getStringValue(m map[string]interface{}, key string) string {
	return utils.GetStringValue(m, key)
}

func getFloatValue(m map[string]interface{}, key string) float64 {
	return utils.GetFloatValue(m, key)
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

	// For walls, extract sprite at index 13 for preview
	isWall := len(cursorName) >= 7 && cursorName[:7] == "ItmWall"
	if isWall {
		tileImg = e.extractSpriteFromSheet(tileImg, 13)
		if tileImg == nil {
			return
		}
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

func (e *Editor) extractSpriteFromSheet(sheet *ebiten.Image, index int) *ebiten.Image {
	// Wall sprite sheets are 16x8 grids
	cols := 16
	rows := 8
	totalSprites := cols * rows

	if index < 0 || index >= totalSprites {
		return nil
	}

	sheetWidth := sheet.Bounds().Dx()
	sheetHeight := sheet.Bounds().Dy()
	spriteWidth := sheetWidth / cols
	spriteHeight := sheetHeight / rows

	col := index % cols
	row := index / cols

	x := col * spriteWidth
	y := row * spriteHeight

	rect := image.Rect(x, y, x+spriteWidth, y+spriteHeight)
	subImg := sheet.SubImage(rect).(*ebiten.Image)
	return subImg
}
