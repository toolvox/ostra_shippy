package renderer

import (
	"fmt"
	"image"
	"image/color"
	_ "image/png"
	"log"
	"math"
	"os"
	"path/filepath"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"github.com/user/ostra_shippy/internal/config"
	"github.com/user/ostra_shippy/internal/models"
	"github.com/user/ostra_shippy/internal/utils"
)

const (
	tileSize = 8
	padding  = 10
)

type ShipRenderer struct {
	offsetX     float32
	offsetY     float32
	scale       float32
	isDragging  bool
	lastMouseX  int
	lastMouseY  int
	showFloor   bool
	showWall    bool
	tileImages  map[string]*ebiten.Image
	viewportX   float32
	viewportY   float32
	viewportW   float32
	viewportH   float32
	hoveredTile map[string]interface{}

	// Current mouse grid position (updated during render)
	mouseGridX  float64
	mouseGridY  float64
	mouseInGrid bool

	// Paint mode - when true, left-click paints instead of panning
	paintMode bool
}

func NewShipRenderer() *ShipRenderer {
	return &ShipRenderer{
		offsetX:    250,
		offsetY:    250,
		scale:      1.0,
		showFloor:  true,
		showWall:   true,
		tileImages: make(map[string]*ebiten.Image),
	}
}

func (r *ShipRenderer) Update() {
	mx, my := ebiten.CursorPosition()

	// Check if mouse is within viewport bounds
	inViewport := float32(mx) >= r.viewportX && float32(mx) < r.viewportX+r.viewportW &&
		float32(my) >= r.viewportY && float32(my) < r.viewportY+r.viewportH

	if !inViewport {
		r.isDragging = false
		return
	}

	// Handle mouse wheel zoom
	_, scrollY := ebiten.Wheel()
	if scrollY != 0 {
		oldScale := r.scale
		r.scale += float32(scrollY) * 0.1
		if r.scale < 0.5 {
			r.scale = 0.5
		}
		if r.scale > 5.0 {
			r.scale = 5.0
		}

		// Adjust offset to zoom towards center of viewport
		if oldScale != r.scale {
			centerX := r.viewportX + r.viewportW/2
			centerY := r.viewportY + r.viewportH/2
			r.offsetX += (float32(mx) - centerX) * (1.0/oldScale - 1.0/r.scale)
			r.offsetY += (float32(my) - centerY) * (1.0/oldScale - 1.0/r.scale)
		}
	}

	// Handle mouse drag for panning (only when not in paint mode)
	if !r.paintMode && ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
		if r.isDragging {
			dx := mx - r.lastMouseX
			dy := my - r.lastMouseY
			r.offsetX += float32(dx)
			r.offsetY += float32(dy)
		}
		r.isDragging = true
		r.lastMouseX = mx
		r.lastMouseY = my
	} else {
		r.isDragging = false
	}

	// Handle WASD panning (speed scales with zoom level)
	panSpeed := float32(5.0) * r.scale
	if ebiten.IsKeyPressed(ebiten.KeyW) {
		r.offsetY += panSpeed
	}
	if ebiten.IsKeyPressed(ebiten.KeyS) {
		r.offsetY -= panSpeed
	}
	if ebiten.IsKeyPressed(ebiten.KeyA) {
		r.offsetX += panSpeed
	}
	if ebiten.IsKeyPressed(ebiten.KeyD) {
		r.offsetX -= panSpeed
	}
}

// getTileImagePath extracts the image path from a tile using the CO lookup map
func getTileImagePath(tile map[string]interface{}, coMap map[string]map[string]interface{}, fallbackName string) string {
	var imagePath string

	// First, try to get strImg from the tile itself (from palette/preloaded data)
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

// calculateWallSpriteIndex determines which sprite from the 4x4 sheet to use
// based on neighboring walls. Returns index 0-15, or 13 as default.
func calculateWallSpriteIndex(items []interface{}, tileX, tileY float64) int {
	// Build a map of wall positions for quick lookup
	wallPositions := make(map[string]bool)
	for _, itemInterface := range items {
		if item, ok := itemInterface.(map[string]interface{}); ok {
			itemName := ""
			if name, ok := item["strName"].(string); ok {
				itemName = name
			}

			// Only consider walls (exclude Loose items which are not wall tiles)
			if len(itemName) >= 7 && itemName[:7] == "ItmWall" && !strings.HasSuffix(itemName, "Loose") {
				x := 0.0
				y := 0.0
				if fX, ok := item["fX"].(float64); ok {
					x = fX
				}
				if fY, ok := item["fY"].(float64); ok {
					y = fY
				}
				key := fmt.Sprintf("%.0f,%.0f", x, y)
				wallPositions[key] = true
			}
		}
	}

	// Check for neighbors (up, down, left, right)
	hasUp := wallPositions[fmt.Sprintf("%.0f,%.0f", tileX, tileY+1)]
	hasDown := wallPositions[fmt.Sprintf("%.0f,%.0f", tileX, tileY-1)]
	hasLeft := wallPositions[fmt.Sprintf("%.0f,%.0f", tileX-1, tileY)]
	hasRight := wallPositions[fmt.Sprintf("%.0f,%.0f", tileX+1, tileY)]

	if config.Verbose {
		log.Printf("Wall at (%.0f,%.0f): U=%v D=%v L=%v R=%v, positions found: %d",
			tileX, tileY, hasUp, hasDown, hasLeft, hasRight, len(wallPositions))
	}

	// Calculate bitmask (up=8, right=4, down=2, left=1)
	bitmask := 0
	if hasUp {
		bitmask |= 8
	}
	if hasRight {
		bitmask |= 4
	}
	if hasDown {
		bitmask |= 2
	}
	if hasLeft {
		bitmask |= 1
	}

	// Map bitmask to sprite index
	// This mapping may need adjustment based on actual sprite sheet layout
	switch bitmask {
	case 0: // No neighbors
		return 13
	case 1: // Left only
		return 12
	case 2: // Down only
		return 1
	case 3: // Left + Down
		return 0
	case 4: // Right only
		return 14
	case 5: // Left + Right (horizontal)
		return 13
	case 6: // Down + Right
		return 2
	case 7: // Left + Down + Right
		return 4
	case 8: // Up only
		return 9
	case 9: // Up + Left
		return 8
	case 10: // Up + Down (vertical)
		return 5
	case 11: // Up + Down + Left
		return 6
	case 12: // Up + Right
		return 10
	case 13: // Up + Right + Left
		return 7
	case 14: // Up + Down + Right
		return 11
	case 15: // All directions
		return 15
	default:
		return 13
	}
}

func (r *ShipRenderer) loadTileImage(imagePath string) *ebiten.Image {
	// Check cache first
	if img, ok := r.tileImages[imagePath]; ok {
		return img
	}

	// Try to load the image
	var tilePath string
	var img image.Image
	var err error

	// Get image path from config
	imagesPath := ""
	if config.GlobalConfig != nil {
		imagesPath = config.GlobalConfig.Images
	}

	if imagesPath == "" {
		if config.Verbose {
			log.Printf("Warning: Images path not set in config")
		}
		r.tileImages[imagePath] = nil
		return nil
	}

	// If imagePath starts with "tiles/", strip it since we're already adding the tiles path
	cleanPath := imagePath
	if strings.HasPrefix(imagePath, "tiles/") {
		cleanPath = strings.TrimPrefix(imagePath, "tiles/")
	}

	// Try different possible paths
	possibilities := []string{
		filepath.Join(imagesPath, "tiles", cleanPath+".png"),
		filepath.Join(imagesPath, imagePath+".png"),
	}

	// Also try treating it as just a filename
	if !strings.Contains(imagePath, "/") && !strings.Contains(imagePath, "\\") {
		possibilities = append(possibilities, filepath.Join(imagesPath, "tiles", imagePath+".png"))

		// Try to extract tile family (e.g., FloorGrate07 from ItmFloorGrate07D)
		if strings.HasPrefix(imagePath, "Itm") && len(imagePath) > 3 {
			baseName := imagePath[3:] // Remove "Itm" prefix
			// Find the tile family by looking for the pattern
			for i := len(baseName) - 1; i >= 0; i-- {
				if baseName[i] >= '0' && baseName[i] <= '9' {
					// Found a digit, the family is up to and including this digit
					family := baseName[:i+1]
					possibilities = append(possibilities, filepath.Join(imagesPath, "tiles", family, imagePath+".png"))
					break
				}
			}
		}
	}

	for _, path := range possibilities {
		file, openErr := os.Open(path)
		if openErr != nil {
			continue
		}
		img, _, err = image.Decode(file)
		file.Close()
		if err == nil {
			tilePath = path
			break
		}
	}

	if img == nil {
		// Return nil if not found (only log first time)
		r.tileImages[imagePath] = nil
		return nil
	}

	ebitenImg := ebiten.NewImageFromImage(img)
	r.tileImages[imagePath] = ebitenImg
	if config.Verbose {
		log.Printf("✓ Loaded tile image: %s from %s", imagePath, tilePath)
	}
	return ebitenImg
}

func (r *ShipRenderer) Render(screen *ebiten.Image, ship *models.Ship, x, y, width, height float32, cursorTile map[string]interface{}) {
	// Update viewport bounds
	r.viewportX = x
	r.viewportY = y
	r.viewportW = width
	r.viewportH = height

	if ship == nil || ship.RawData == nil {
		return
	}

	if !r.showFloor && !r.showWall {
		return
	}

	// Create a clipped sub-image for the viewport to prevent drawing over panels
	viewportRect := image.Rect(int(x), int(y), int(x+width), int(y+height))
	clippedScreen := screen.SubImage(viewportRect).(*ebiten.Image)

	// Build CO lookup map from aCOs
	coMap := make(map[string]map[string]interface{})
	if aCOs, ok := ship.RawData["aCOs"].([]interface{}); ok {
		for _, coInterface := range aCOs {
			if co, ok := coInterface.(map[string]interface{}); ok {
				if strID, ok := co["strID"].(string); ok {
					coMap[strID] = co
				}
			}
		}
	}

	// Get items from ship data
	items, ok := ship.RawData["aItems"].([]interface{})
	if !ok {
		log.Println("No aItems found in ship data")
		return
	}

	var minX, minY, maxX, maxY float64
	firstTile := true

	// First pass: find bounds (from both floor and wall tiles)
	for _, itemInterface := range items {
		item, ok := itemInterface.(map[string]interface{})
		if !ok {
			continue
		}

		itemName, ok := item["strName"].(string)
		if !ok {
			continue
		}

		// Check if this is a floor or wall tile (exclude Loose items)
		isFloor := len(itemName) >= 8 && itemName[:8] == "ItmFloor" && !strings.HasSuffix(itemName, "Loose")
		isWall := len(itemName) >= 7 && itemName[:7] == "ItmWall" && !strings.HasSuffix(itemName, "Loose")

		if !isFloor && !isWall {
			continue
		}

		// Get tile position
		tileX, okX := item["fX"].(float64)
		tileY, okY := item["fY"].(float64)
		if !okX || !okY {
			continue
		}

		if firstTile {
			minX, maxX = tileX, tileX
			minY, maxY = tileY, tileY
			firstTile = false
		} else {
			if tileX < minX {
				minX = tileX
			}
			if tileX > maxX {
				maxX = tileX
			}
			if tileY < minY {
				minY = tileY
			}
			if tileY > maxY {
				maxY = tileY
			}
		}
	}

	// Calculate center offset
	centerX := float32(-(minX + maxX) / 2.0)
	centerY := float32(-(minY + maxY) / 2.0)

	// Get mouse position for hover detection
	mx, my := ebiten.CursorPosition()
	r.hoveredTile = nil

	// Second pass: draw floors first
	for _, itemInterface := range items {
		item, ok := itemInterface.(map[string]interface{})
		if !ok {
			continue
		}

		itemName, ok := item["strName"].(string)
		if !ok {
			continue
		}

		isFloor := len(itemName) >= 8 && itemName[:8] == "ItmFloor" && !strings.HasSuffix(itemName, "Loose")
		if !isFloor || !r.showFloor {
			continue
		}

		r.renderTile(item, itemName, items, coMap, clippedScreen, x, y, centerX, centerY, mx, my, false)
	}

	// Third pass: draw walls on top
	for _, itemInterface := range items {
		item, ok := itemInterface.(map[string]interface{})
		if !ok {
			continue
		}

		itemName, ok := item["strName"].(string)
		if !ok {
			continue
		}

		isWall := len(itemName) >= 7 && itemName[:7] == "ItmWall" && !strings.HasSuffix(itemName, "Loose")
		if !isWall || !r.showWall {
			continue
		}

		r.renderTile(item, itemName, items, coMap, clippedScreen, x, y, centerX, centerY, mx, my, true)
	}

	// Draw cursor preview
	r.renderCursorPreview(cursorTile, items, coMap, clippedScreen, x, y, centerX, centerY)
}

func (r *ShipRenderer) renderTile(item map[string]interface{}, itemName string, items []interface{}, coMap map[string]map[string]interface{}, clippedScreen *ebiten.Image, x, y, centerX, centerY float32, mx, my int, isWall bool) {
	// Get tile position
	tileX, okX := item["fX"].(float64)
	tileY, okY := item["fY"].(float64)
	if !okX || !okY {
		return
	}

	// Get rotation
	rotation := 0.0
	if fRotation, ok := item["fRotation"].(float64); ok {
		rotation = fRotation
	}

	// Calculate draw position (flip Y-axis)
	drawX := float32(int(x + r.offsetX + (float32(tileX)+centerX)*tileSize*r.scale))
	drawY := float32(int(y + r.offsetY + (-float32(tileY)-centerY)*tileSize*r.scale))
	drawSize := float32(int(tileSize * r.scale))

	// Get the tile image path
	imagePath := getTileImagePath(item, coMap, itemName)

	// Check if mouse is hovering over this tile
	isHovered := float32(mx) >= drawX && float32(mx) < drawX+drawSize &&
		float32(my) >= drawY && float32(my) < drawY+drawSize
	if isHovered {
		r.hoveredTile = item
		// Store the grid position for cursor tile placement
		r.mouseGridX = tileX
		r.mouseGridY = tileY
		r.mouseInGrid = true
	}

	// Try to load and draw the tile image
	tileImg := r.loadTileImage(imagePath)
	if tileImg != nil {
		// For walls, extract the correct sprite from the 4x4 sheet
		var spriteToRender *ebiten.Image
		if isWall {
			// TODO: Fix neighbor detection - for now just use index 13
			spriteToRender = utils.ExtractSpriteFromSheet(tileImg, 13)
		} else {
			// Floors use the full image
			spriteToRender = tileImg
		}

		if spriteToRender != nil {
			// Draw the actual tile image with rotation
			opts := &ebiten.DrawImageOptions{}

			// Scale
			scaleX := drawSize / float32(spriteToRender.Bounds().Dx())
			scaleY := drawSize / float32(spriteToRender.Bounds().Dy())
			opts.GeoM.Scale(float64(scaleX), float64(scaleY))

			opts.GeoM.Translate(-float64(drawSize)/2, -float64(drawSize)/2)
			theta := (360 - rotation) * math.Pi / 180.0
			opts.GeoM.Rotate(theta)
			opts.GeoM.Translate(float64(drawSize)/2, float64(drawSize)/2)

			// Translate to position
			opts.GeoM.Translate(float64(drawX), float64(drawY))
			clippedScreen.DrawImage(spriteToRender, opts)
		}
	} else {
		// Fallback to colored rectangle if image not found
		tileColor := color.NRGBA{80, 120, 160, 255}
		vector.DrawFilledRect(clippedScreen, drawX, drawY, drawSize, drawSize, tileColor, false)
		borderColor := color.NRGBA{50, 50, 60, 255}
		vector.StrokeRect(clippedScreen, drawX, drawY, drawSize, drawSize, 1, borderColor, false)
	}

	// Highlight hovered tile
	if isHovered {
		highlightColor := color.NRGBA{255, 255, 0, 128}
		vector.StrokeRect(clippedScreen, drawX, drawY, drawSize, drawSize, 2, highlightColor, false)
	}
}

func (r *ShipRenderer) renderCursorPreview(cursorTile map[string]interface{}, items []interface{}, coMap map[string]map[string]interface{}, clippedScreen *ebiten.Image, x, y, centerX, centerY float32) {
	if cursorTile == nil || !r.mouseInGrid {
		return
	}

	// Get cursor tile image path
	cursorName := ""
	if strName, ok := cursorTile["strName"].(string); ok {
		cursorName = strName
	}
	cursorImagePath := getTileImagePath(cursorTile, coMap, cursorName)
	cursorImg := r.loadTileImage(cursorImagePath)
	if cursorImg != nil {
		// Check if cursor tile is a wall
		isCursorWall := len(cursorName) >= 7 && cursorName[:7] == "ItmWall"

		// For walls, extract sprite based on what neighbors would be after placement
		var spriteToRender *ebiten.Image
		if isCursorWall {
			// Use index 13 for cursor preview (standard wall segment)
			spriteToRender = utils.ExtractSpriteFromSheet(cursorImg, 13)
		} else {
			spriteToRender = cursorImg
		}

		if spriteToRender != nil {
			// Get cursor rotation
			cursorRotation := 0.0
			if fRotation, ok := cursorTile["fRotation"].(float64); ok {
				cursorRotation = fRotation
			}

			// Calculate draw size based on current zoom
			drawSize := float32(int(tileSize * r.scale))

			// Use the stored grid position (set when hovering over a tile)
			// Convert grid position to screen coordinates for rendering
			drawX := float32(int(x + r.offsetX + (float32(r.mouseGridX)+centerX)*tileSize*r.scale))
			drawY := float32(int(y + r.offsetY + (-float32(r.mouseGridY)-centerY)*tileSize*r.scale))

			// Draw semi-transparent cursor tile
			opts := &ebiten.DrawImageOptions{}

			// Scale to match current zoom
			imgW, imgH := spriteToRender.Bounds().Dx(), spriteToRender.Bounds().Dy()
			scaleX := drawSize / float32(imgW)
			scaleY := drawSize / float32(imgH)
			opts.GeoM.Scale(float64(scaleX), float64(scaleY))

			// Apply rotation
			if cursorRotation != 0 {
				opts.GeoM.Translate(-float64(drawSize)/2, -float64(drawSize)/2)
				opts.GeoM.Rotate((cursorRotation + 0) * math.Pi / 180.0)
				opts.GeoM.Translate(float64(drawSize)/2, float64(drawSize)/2)
			}

			opts.GeoM.Translate(float64(drawX), float64(drawY))

			// Draw with transparency
			opts.ColorScale.ScaleAlpha(0.6)
			clippedScreen.DrawImage(spriteToRender, opts)

			// Draw grid outline
			outlineColor := color.NRGBA{0, 255, 255, 180}
			vector.StrokeRect(clippedScreen, drawX, drawY, drawSize, drawSize, 2, outlineColor, false)
		}
	}
}

func (r *ShipRenderer) ToggleFloor() {
	r.showFloor = !r.showFloor
}

func (r *ShipRenderer) SetShowFloor(show bool) {
	r.showFloor = show
}

func (r *ShipRenderer) SetShowWall(show bool) {
	r.showWall = show
}

func (r *ShipRenderer) GetHoveredTile() map[string]interface{} {
	return r.hoveredTile
}

// GetMouseGridPos returns the current mouse grid position (snapped coordinates)
// These coordinates are updated during Render when a cursor tile is active
// Returns (gridX, gridY, valid)
func (r *ShipRenderer) GetMouseGridPos() (float64, float64, bool) {
	return r.mouseGridX, r.mouseGridY, r.mouseInGrid
}

// LoadTileImage exposes the tile image loading function for external use
func (r *ShipRenderer) LoadTileImage(imagePath string) *ebiten.Image {
	return r.loadTileImage(imagePath)
}

// SetPaintMode enables or disables paint mode (disables panning when true)
func (r *ShipRenderer) SetPaintMode(enabled bool) {
	r.paintMode = enabled
}
