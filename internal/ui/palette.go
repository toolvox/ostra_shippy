package ui

import (
	"fmt"
	"image/color"
	"log"
	"path/filepath"
	"sort"
	"strings"

	ebitenui_image "github.com/ebitenui/ebitenui/image"
	"github.com/ebitenui/ebitenui/widget"
	"github.com/user/ostra_shippy/internal/config"
	"github.com/user/ostra_shippy/internal/loader"
	"github.com/user/ostra_shippy/internal/utils"
)

func (e *Editor) buildFloorPalette() {
	// Build floor tile palette from cooverlays_floors.json in the data directory
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

		strName := utils.GetStringValue(tileDef, "strName")

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
		return utils.GetStringValue(e.availableTiles[i], "strName") < utils.GetStringValue(e.availableTiles[j], "strName")
	})

	if config.Verbose {
		log.Printf("Built tile palette with %d floor tiles from %s", len(e.availableTiles), jsonPath)
	}

	// Initialize filtered tiles
	e.filterTilePalette("")
}

func (e *Editor) buildWallPalette() {
	// Build wall tile palette from cooverlays_walls.json in the data directory
	dataPath := ""
	if config.GlobalConfig != nil {
		dataPath = config.GlobalConfig.Data
	}

	if dataPath == "" {
		log.Printf("Warning: Data path not set, cannot build wall palette")
		return
	}

	jsonPath := filepath.Join(dataPath, "cooverlays", "cooverlays_walls.json")

	// Load JSON file
	jsonData, err := loader.LoadJSON(jsonPath)
	if err != nil {
		log.Printf("Error loading cooverlays_walls.json: %v", err)
		return
	}

	// Parse the JSON array
	wallDefinitions, ok := jsonData.([]interface{})
	if !ok {
		log.Printf("Error: cooverlays_walls.json is not an array")
		return
	}

	e.availableWallTiles = make([]map[string]interface{}, 0)

	for _, wallDefInterface := range wallDefinitions {
		wallDef, ok := wallDefInterface.(map[string]interface{})
		if !ok {
			continue
		}

		strName := utils.GetStringValue(wallDef, "strName")

		// Only include wall tiles (but exclude ItmWall*Loose which are items, not walls)
		if len(strName) < 7 || strName[:7] != "ItmWall" {
			continue
		}

		// Filter out damaged, patched, and loose variants
		// Note: Loose walls are considered items, not wall tiles
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
		tile["_coDefinition"] = wallDef

		// Store additional info for later use (like image path)
		if strImg, ok := wallDef["strImg"].(string); ok {
			tile["strImg"] = strImg
		}
		if strNameFriendly, ok := wallDef["strNameFriendly"].(string); ok {
			tile["strNameFriendly"] = strNameFriendly
		}

		e.availableWallTiles = append(e.availableWallTiles, tile)
	}

	// Sort tiles by name
	sort.Slice(e.availableWallTiles, func(i, j int) bool {
		return utils.GetStringValue(e.availableWallTiles[i], "strName") < utils.GetStringValue(e.availableWallTiles[j], "strName")
	})

	if config.Verbose {
		log.Printf("Built wall palette with %d wall tiles from %s", len(e.availableWallTiles), jsonPath)
	}

	// Initialize filtered wall tiles
	e.filteredWallTiles = e.availableWallTiles
}

func (e *Editor) filterTilePalette(searchText string) {
	searchText = strings.ToLower(searchText)

	// Filter floor tiles
	if e.availableTiles != nil {
		if searchText == "" {
			e.filteredTiles = e.availableTiles
		} else {
			e.filteredTiles = make([]map[string]interface{}, 0)
			for _, tile := range e.availableTiles {
				tileName := strings.ToLower(utils.GetStringValue(tile, "strName"))
				if strings.Contains(tileName, searchText) {
					e.filteredTiles = append(e.filteredTiles, tile)
				}
			}
		}
	}

	// Filter wall tiles
	if e.availableWallTiles != nil {
		if searchText == "" {
			e.filteredWallTiles = e.availableWallTiles
		} else {
			e.filteredWallTiles = make([]map[string]interface{}, 0)
			for _, tile := range e.availableWallTiles {
				tileName := strings.ToLower(utils.GetStringValue(tile, "strName"))
				if strings.Contains(tileName, searchText) {
					e.filteredWallTiles = append(e.filteredWallTiles, tile)
				}
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

	// Update layer choice visibility
	e.updateLayerChoiceVisibility()

	// Determine which tiles to show based on current layer
	var tilesToShow []map[string]interface{}
	if e.currentLayer == "wall" {
		tilesToShow = e.filteredWallTiles
	} else {
		tilesToShow = e.filteredTiles
	}

	// Add all filtered tiles as buttons in a 5-column grid
	for _, tile := range tilesToShow {
		tileName := utils.GetStringValue(tile, "strName")
		// Use the strImg field from the preloaded tile data
		imagePath := utils.GetStringValue(tile, "strImg")
		if imagePath == "" {
			imagePath = tileName
		}
		tileImg := e.renderer.LoadTileImage(imagePath)

		// For walls, extract sprite at index 13 for palette preview
		isWall := len(tileName) >= 7 && tileName[:7] == "ItmWall"
		if isWall && tileImg != nil {
			tileImg = utils.ExtractSpriteFromSheet(tileImg, 13)
		}

		// Create button with tile image
		btn := widget.NewButton(
			widget.ButtonOpts.WidgetOpts(
				widget.WidgetOpts.MinSize(40, 40),
			),
			widget.ButtonOpts.Image(&widget.ButtonImage{
				Idle:    ebitenui_image.NewNineSliceColor(color.NRGBA{50, 50, 60, 255}),
				Hover:   ebitenui_image.NewNineSliceColor(color.NRGBA{70, 100, 150, 255}),
				Pressed: ebitenui_image.NewNineSliceColor(color.NRGBA{60, 90, 130, 255}),
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
	e.setStatus(fmt.Sprintf("Selected from palette: %s", utils.GetStringValue(tile, "strName")))
}
