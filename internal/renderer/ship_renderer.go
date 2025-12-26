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
	offsetX         float32
	offsetY         float32
	scale           float32
	isDragging      bool
	lastMouseX      int
	lastMouseY      int
	showFloor       bool
	showWall        bool
	showConduit     bool
	showInstallable bool
	showLoose       bool
	tileImages      map[string]*ebiten.Image
	viewportX       float32
	viewportY       float32
	viewportW       float32
	viewportH       float32
	hoveredTile     map[string]interface{}

	// Current mouse grid position (updated during render)
	mouseGridX  float64
	mouseGridY  float64
	mouseInGrid bool

	// Paint mode - when true, left-click paints instead of panning
	paintMode bool
}

func NewShipRenderer() *ShipRenderer {
	return &ShipRenderer{
		offsetX:         250,
		offsetY:         250,
		scale:           1.0,
		showFloor:       true,
		showWall:        true,
		showConduit:     true,
		showInstallable: true,
		showLoose:       true,
		tileImages:      make(map[string]*ebiten.Image),
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

	if !r.showFloor && !r.showWall && !r.showConduit && !r.showInstallable && !r.showLoose {
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

	// First pass: find bounds (from floor, wall, and conduit tiles)
	for _, itemInterface := range items {
		item, ok := itemInterface.(map[string]interface{})
		if !ok {
			continue
		}

		itemName, ok := item["strName"].(string)
		if !ok {
			continue
		}

		// Check if this is a floor, wall, conduit, or installed item (exclude Loose items)
		isFloor := len(itemName) >= 8 && itemName[:8] == "ItmFloor" && !strings.HasSuffix(itemName, "Loose")
		isWall := len(itemName) >= 7 && itemName[:7] == "ItmWall" && !strings.HasSuffix(itemName, "Loose")
		isConduit := len(itemName) >= 10 && itemName[:10] == "ItmConduit" && !strings.HasSuffix(itemName, "Loose")
		isInstalled := strings.HasPrefix(itemName, "Itm") && !isFloor && !isWall && !isConduit && !strings.HasSuffix(itemName, "Loose")

		if !isFloor && !isWall && !isConduit && !isInstalled {
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

	// Calculate grid position from mouse even if not hovering over a tile
	// This allows placement on empty adjacent cells
	if float32(mx) >= x && float32(mx) < x+width && float32(my) >= y && float32(my) < y+height {
		// Convert screen coords to world coords
		screenX := float32(mx) - x - r.offsetX
		screenY := float32(my) - y - r.offsetY

		// Convert to grid position
		gridWorldX := screenX / (tileSize * r.scale)
		gridWorldY := -screenY / (tileSize * r.scale) // Flip Y

		// Apply center offset
		gridWorldX -= centerX
		gridWorldY -= centerY

		// Snap to grid
		r.mouseGridX = math.Round(float64(gridWorldX))
		r.mouseGridY = math.Round(float64(gridWorldY))
		r.mouseInGrid = true
	} else {
		r.mouseInGrid = false
	}

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

	// Third pass: draw walls on top of floors
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

	// Fourth pass: draw conduits on top of walls
	for _, itemInterface := range items {
		item, ok := itemInterface.(map[string]interface{})
		if !ok {
			continue
		}

		itemName, ok := item["strName"].(string)
		if !ok {
			continue
		}

		isConduit := len(itemName) >= 10 && itemName[:10] == "ItmConduit" && !strings.HasSuffix(itemName, "Loose")
		if !isConduit || !r.showConduit {
			continue
		}

		r.renderTile(item, itemName, items, coMap, clippedScreen, x, y, centerX, centerY, mx, my, true)
	}

	// Fifth pass: draw installable items on top of conduits
	for _, itemInterface := range items {
		item, ok := itemInterface.(map[string]interface{})
		if !ok {
			continue
		}

		itemName, ok := item["strName"].(string)
		if !ok {
			continue
		}

		// Installable items are Itm* that are not Floor, Wall, Conduit, or Loose
		isFloor := len(itemName) >= 8 && itemName[:8] == "ItmFloor"
		isWall := len(itemName) >= 7 && itemName[:7] == "ItmWall"
		isConduit := len(itemName) >= 10 && itemName[:10] == "ItmConduit"
		isInstallable := strings.HasPrefix(itemName, "Itm") && !isFloor && !isWall && !isConduit && !strings.HasSuffix(itemName, "Loose")

		if !isInstallable || !r.showInstallable {
			continue
		}

		r.renderTile(item, itemName, items, coMap, clippedScreen, x, y, centerX, centerY, mx, my, false)
	}

	// Sixth pass: draw loose items on top of installables
	for _, itemInterface := range items {
		item, ok := itemInterface.(map[string]interface{})
		if !ok {
			continue
		}

		itemName, ok := item["strName"].(string)
		if !ok {
			continue
		}

		// Loose items are either regular Itm* items OR any item with "Loose" suffix
		isFloor := len(itemName) >= 8 && itemName[:8] == "ItmFloor"
		isWall := len(itemName) >= 7 && itemName[:7] == "ItmWall"
		isConduit := len(itemName) >= 10 && itemName[:10] == "ItmConduit"
		hasLooseSuffix := strings.HasSuffix(itemName, "Loose")

		// Loose items: either has Loose suffix, OR is a regular Itm* item (not Floor/Wall/Conduit and not installable-type)
		// For now, treat all non-Floor/Wall/Conduit Itm* items that aren't explicitly installable as loose
		isLoose := (strings.HasPrefix(itemName, "Itm") && !isFloor && !isWall && !isConduit) || hasLooseSuffix

		// Skip if it's an installable item (already rendered in previous pass)
		isInstallable := strings.HasPrefix(itemName, "Itm") && !isFloor && !isWall && !isConduit && !hasLooseSuffix
		if isInstallable {
			continue
		}

		if !isLoose || !r.showLoose {
			continue
		}

		r.renderTile(item, itemName, items, coMap, clippedScreen, x, y, centerX, centerY, mx, my, false)
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

	// Get item size from CO definition (for multi-tile items)
	sizeX := 1.0
	sizeY := 1.0
	tileID := utils.GetStringValue(item, "strID")
	if co, exists := coMap[tileID]; exists {
		if fSizeX, ok := co["fSizeX"].(float64); ok && fSizeX > 0 {
			sizeX = fSizeX
		}
		if fSizeY, ok := co["fSizeY"].(float64); ok && fSizeY > 0 {
			sizeY = fSizeY
		}
		// Debug: log size for non-1x1 items
		if (sizeX != 1.0 || sizeY != 1.0) && config.Verbose {
			log.Printf("Rendering multi-tile item %s with size %.1fx%.1f", itemName, sizeX, sizeY)
		}
	} else if config.Verbose {
		// Check if this item should have a CO but doesn't
		if !strings.HasPrefix(itemName, "ItmFloor") && !strings.HasPrefix(itemName, "ItmWall") && !strings.HasPrefix(itemName, "ItmConduit") {
			log.Printf("Warning: Item %s (ID: %s) has no CO definition in coMap", itemName, tileID)
		}
	}

	// Calculate draw position (flip Y-axis)
	drawX := float32(int(x + r.offsetX + (float32(tileX)+centerX)*tileSize*r.scale))
	drawY := float32(int(y + r.offsetY + (-float32(tileY)-centerY)*tileSize*r.scale))
	drawSize := float32(int(tileSize * r.scale))

	// Get the tile image path
	imagePath := getTileImagePath(item, coMap, itemName)

	// Load the image first to determine size
	tileImg := r.loadTileImage(imagePath)

	// For walls and conduits, extract the correct sprite from the sprite sheet
	var spriteToRender *ebiten.Image
	isConduit := len(itemName) >= 10 && itemName[:10] == "ItmConduit"
	isFloor := len(itemName) >= 8 && itemName[:8] == "ItmFloor"
	isInstallable := !isFloor && !isWall && !isConduit && strings.HasPrefix(itemName, "Itm") && !strings.HasSuffix(itemName, "Loose")
	isLoose := strings.HasSuffix(itemName, "Loose")

	if tileImg != nil {
		// Check if this is a sprite sheet (contains "Sheet" in filename case-insensitive)
		lowerPath := strings.ToLower(imagePath)
		isSpriteSheet := strings.Contains(lowerPath, "sheet")

		if config.Verbose && isSpriteSheet {
			log.Printf("Sprite sheet detected: %s for item %s", imagePath, itemName)
		}

		if isWall || isConduit || isSpriteSheet {
			// TODO: Fix neighbor detection - for now just use index 13
			spriteToRender = utils.ExtractSpriteFromSheet(tileImg, 13)
		} else {
			// Floors and non-sheet items use the full image
			spriteToRender = tileImg
		}
	}

	// Calculate draw size based on item type
	var drawWidth, drawHeight float32
	if (isInstallable || isLoose) && spriteToRender != nil {
		// For installable/loose items, use natural image size at 0.5x scale (then apply zoom)
		drawWidth = float32(spriteToRender.Bounds().Dx()) * r.scale * 0.5
		drawHeight = float32(spriteToRender.Bounds().Dy()) * r.scale * 0.5
	} else {
		// For floors, walls, conduits: use grid-based sizing
		drawWidth = drawSize * float32(sizeX)
		drawHeight = drawSize * float32(sizeY)
	}

	// Check if mouse is hovering over this tile (use actual width/height)
	var actualDrawX, actualDrawY float32
	if isInstallable || isLoose {
		// For centered items, adjust hover box
		tileCenterOffsetX := drawSize / 2
		tileCenterOffsetY := drawSize / 2
		actualDrawX = drawX + tileCenterOffsetX - drawWidth/2
		actualDrawY = drawY + tileCenterOffsetY - drawHeight/2
	} else {
		actualDrawX = drawX
		actualDrawY = drawY
	}

	isHovered := float32(mx) >= actualDrawX && float32(mx) < actualDrawX+drawWidth &&
		float32(my) >= actualDrawY && float32(my) < actualDrawY+drawHeight
	if isHovered {
		r.hoveredTile = item
		// Store the grid position for cursor tile placement
		r.mouseGridX = tileX
		r.mouseGridY = tileY
		r.mouseInGrid = true
	}

	// Draw the tile image
	if spriteToRender != nil {
		// Draw the actual tile image with rotation
		opts := &ebiten.DrawImageOptions{}

		// Scale based on item type
		if isInstallable || isLoose {
			// Rotate around sprite center first (at original size)
			imgW := float32(spriteToRender.Bounds().Dx())
			imgH := float32(spriteToRender.Bounds().Dy())
			opts.GeoM.Translate(-float64(imgW)/2, -float64(imgH)/2)
			theta := (360 - rotation) * math.Pi / 180.0
			opts.GeoM.Rotate(theta)
			opts.GeoM.Translate(float64(imgW)/2, float64(imgH)/2)

			// Then apply 0.5x scale and zoom scale
			opts.GeoM.Scale(float64(r.scale)*0.5, float64(r.scale)*0.5)
		} else {
			// Scale to fit grid tiles
			scaleX := drawWidth / float32(spriteToRender.Bounds().Dx())
			scaleY := drawHeight / float32(spriteToRender.Bounds().Dy())
			opts.GeoM.Scale(float64(scaleX), float64(scaleY))

			// Rotate around center
			opts.GeoM.Translate(-float64(drawWidth)/2, -float64(drawHeight)/2)
			theta := (360 - rotation) * math.Pi / 180.0
			opts.GeoM.Rotate(theta)
			opts.GeoM.Translate(float64(drawWidth)/2, float64(drawHeight)/2)
		}

		// Translate to position
		if isInstallable || isLoose {
			// Center the item on the grid tile center
			tileCenterOffsetX := drawSize / 2
			tileCenterOffsetY := drawSize / 2
			opts.GeoM.Translate(float64(drawX+tileCenterOffsetX-drawWidth/2), float64(drawY+tileCenterOffsetY-drawHeight/2))
		} else {
			opts.GeoM.Translate(float64(drawX), float64(drawY))
		}
		clippedScreen.DrawImage(spriteToRender, opts)
	} else {
		// Fallback to colored rectangle if image not found (use actual width/height)
		tileColor := color.NRGBA{80, 120, 160, 255}
		vector.DrawFilledRect(clippedScreen, drawX, drawY, drawWidth, drawHeight, tileColor, false)
		borderColor := color.NRGBA{50, 50, 60, 255}
		vector.StrokeRect(clippedScreen, drawX, drawY, drawWidth, drawHeight, 1, borderColor, false)
	}

	// Check if strCODef matches strName
	tileID = ""
	if strID, ok := item["strID"].(string); ok {
		tileID = strID
	}

	mismatchDetected := false
	if tileID != "" {
		if co, ok := coMap[tileID]; ok {
			strCODef := ""
			if coDefVal, ok := co["strCODef"].(string); ok {
				strCODef = coDefVal
			}
			// If strCODef exists and doesn't match strName, mark as mismatched
			if strCODef != "" && strCODef != itemName {
				mismatchDetected = true
			}
		}
	}

	// Draw red transparent overlay if CO definition doesn't match tile name
	if mismatchDetected {
		redOverlay := color.NRGBA{255, 0, 0, 64}
		vector.DrawFilledRect(clippedScreen, actualDrawX, actualDrawY, drawWidth, drawHeight, redOverlay, false)
	}

	// Highlight hovered tile
	if isHovered {
		highlightColor := color.NRGBA{255, 255, 0, 128}

		if (isInstallable || isLoose) && rotation != 0 {
			// For rotated items, draw rotated rectangle
			centerX := actualDrawX + drawWidth/2
			centerY := actualDrawY + drawHeight/2
			halfW := drawWidth / 2
			halfH := drawHeight / 2

			// Calculate rotation angle (negate to match item rotation)
			theta := (360 - rotation) * math.Pi / 180.0
			cosTheta := float32(math.Cos(theta))
			sinTheta := float32(math.Sin(theta))

			// Calculate the four corners rotated around center
			corners := [4][2]float32{
				{-halfW, -halfH}, // top-left
				{halfW, -halfH},  // top-right
				{halfW, halfH},   // bottom-right
				{-halfW, halfH},  // bottom-left
			}

			// Rotate and translate each corner
			for i := range corners {
				x := corners[i][0]
				y := corners[i][1]
				corners[i][0] = x*cosTheta - y*sinTheta + centerX
				corners[i][1] = x*sinTheta + y*cosTheta + centerY
			}

			// Draw lines between corners
			for i := 0; i < 4; i++ {
				next := (i + 1) % 4
				vector.StrokeLine(clippedScreen,
					corners[i][0], corners[i][1],
					corners[next][0], corners[next][1],
					2, highlightColor, false)
			}
		} else {
			// For non-rotated items, use simple rectangle
			vector.StrokeRect(clippedScreen, actualDrawX, actualDrawY, drawWidth, drawHeight, 2, highlightColor, false)
		}
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
		// Check if cursor tile is a wall or conduit
		isCursorWall := len(cursorName) >= 7 && cursorName[:7] == "ItmWall"
		isCursorConduit := len(cursorName) >= 10 && cursorName[:10] == "ItmConduit"

		// For walls and conduits, extract sprite based on what neighbors would be after placement
		var spriteToRender *ebiten.Image
		if isCursorWall || isCursorConduit {
			// Use index 13 for cursor preview (standard segment)
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

			// Apply rotation (use same calculation as floor tiles)
			if cursorRotation != 0 {
				opts.GeoM.Translate(-float64(drawSize)/2, -float64(drawSize)/2)
				theta := (360 - cursorRotation) * math.Pi / 180.0
				opts.GeoM.Rotate(theta)
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

func (r *ShipRenderer) SetShowConduit(show bool) {
	r.showConduit = show
}

func (r *ShipRenderer) SetShowInstallable(show bool) {
	r.showInstallable = show
}

func (r *ShipRenderer) SetShowLoose(show bool) {
	r.showLoose = show
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
