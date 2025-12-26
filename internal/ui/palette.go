package ui

import (
	"fmt"
	"image/color"
	"log"
	"math"
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

func (e *Editor) buildConduitPalette() {
	// Build conduit tile palette from cooverlays.json in the data directory
	dataPath := ""
	if config.GlobalConfig != nil {
		dataPath = config.GlobalConfig.Data
	}

	if dataPath == "" {
		log.Printf("Warning: Data path not set, cannot build conduit palette")
		return
	}

	jsonPath := filepath.Join(dataPath, "cooverlays", "cooverlays.json")

	// Load JSON file
	jsonData, err := loader.LoadJSON(jsonPath)
	if err != nil {
		log.Printf("Error loading cooverlays.json: %v", err)
		return
	}

	// Parse the JSON array
	conduitDefinitions, ok := jsonData.([]interface{})
	if !ok {
		log.Printf("Error: cooverlays.json is not an array")
		return
	}

	e.availableConduitTiles = make([]map[string]interface{}, 0)

	for _, conduitDefInterface := range conduitDefinitions {
		conduitDef, ok := conduitDefInterface.(map[string]interface{})
		if !ok {
			continue
		}

		strName := utils.GetStringValue(conduitDef, "strName")

		// Only include conduit tiles (check prefix with proper length validation)
		if !strings.HasPrefix(strName, "ItmConduit") {
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
		tile["_coDefinition"] = conduitDef

		// Store additional info for later use (like image path)
		if strImg, ok := conduitDef["strImg"].(string); ok {
			tile["strImg"] = strImg
		}
		if strNameFriendly, ok := conduitDef["strNameFriendly"].(string); ok {
			tile["strNameFriendly"] = strNameFriendly
		}

		e.availableConduitTiles = append(e.availableConduitTiles, tile)
	}

	// Sort tiles by name
	sort.Slice(e.availableConduitTiles, func(i, j int) bool {
		return utils.GetStringValue(e.availableConduitTiles[i], "strName") < utils.GetStringValue(e.availableConduitTiles[j], "strName")
	})

	if config.Verbose {
		log.Printf("Built conduit palette with %d conduit tiles from %s", len(e.availableConduitTiles), jsonPath)
	}

	// Initialize filtered conduit tiles
	e.filteredConduitTiles = e.availableConduitTiles
}

func (e *Editor) buildInstallablePalette() {
	// Build installable items palette from installables.json in the data directory
	dataPath := ""
	if config.GlobalConfig != nil {
		dataPath = config.GlobalConfig.Data
	}

	if dataPath == "" {
		log.Printf("Warning: Data path not set, cannot build installable palette")
		return
	}

	installablesPath := filepath.Join(dataPath, "installables", "installables.json")

	// Load installables JSON file
	installablesData, err := loader.LoadJSON(installablesPath)
	if err != nil {
		log.Printf("Error loading installables.json: %v", err)
		return
	}

	// Parse the JSON array
	installableDefinitions, ok := installablesData.([]interface{})
	if !ok {
		log.Printf("Error: installables.json is not an array")
		return
	}

	// Load items.json to get correct strImg values
	itemsPath := filepath.Join(dataPath, "items", "items.json")
	itemsData, err := loader.LoadJSON(itemsPath)
	if err != nil {
		log.Printf("Error loading items.json: %v", err)
		return
	}

	itemsList, ok := itemsData.([]interface{})
	if !ok {
		log.Printf("Error: items.json is not an array")
		return
	}

	// Build a map of item name -> item definition (for strImg)
	itemsMap := make(map[string]map[string]interface{})
	for _, itemInterface := range itemsList {
		if item, ok := itemInterface.(map[string]interface{}); ok {
			itemName := utils.GetStringValue(item, "strName")
			if itemName != "" {
				itemsMap[itemName] = item
			}
		}
	}

	// Also load cooverlays.json to get item details
	cooverlaysPath := filepath.Join(dataPath, "cooverlays", "cooverlays.json")
	cooverlaysData, err := loader.LoadJSON(cooverlaysPath)
	if err != nil {
		log.Printf("Error loading cooverlays.json: %v", err)
		return
	}

	cooverlaysList, ok := cooverlaysData.([]interface{})
	if !ok {
		log.Printf("Error: cooverlays.json is not an array")
		return
	}

	// Build a map of item name -> CO definition
	coMap := make(map[string]map[string]interface{})
	for _, coInterface := range cooverlaysList {
		if co, ok := coInterface.(map[string]interface{}); ok {
			coName := utils.GetStringValue(co, "strName")
			if coName != "" {
				coMap[coName] = co
			}
		}
	}

	e.availableInstallableTiles = make([]map[string]interface{}, 0)
	seenItems := make(map[string]bool) // Track unique items

	for _, installableDefInterface := range installableDefinitions {
		installableDef, ok := installableDefInterface.(map[string]interface{})
		if !ok {
			continue
		}

		strName := utils.GetStringValue(installableDef, "strName")

		var itemName string

		// Check for Install variant - get item from strStartInstall
		if strings.Contains(strName, "Install") && !strings.Contains(strName, "Uninstall") {
			itemName = utils.GetStringValue(installableDef, "strStartInstall")
		} else if strings.Contains(strName, "Uninstall") {
			// Check for Uninstall variant - get item from strActionCO
			itemName = utils.GetStringValue(installableDef, "strActionCO")
		} else {
			// Not an install/uninstall action
			continue
		}

		// Skip if no item name found
		if itemName == "" {
			continue
		}

		// Skip if we've already seen this item
		if seenItems[itemName] {
			continue
		}
		seenItems[itemName] = true

		// Get the CO definition for this item
		coDef, hasCO := coMap[itemName]

		// Create a tile template
		tile := make(map[string]interface{})
		tile["strName"] = itemName
		tile["fX"] = 0.0
		tile["fY"] = 0.0
		tile["fRotation"] = 0.0
		tile["strID"] = e.generateGUID() // Generate a temporary ID

		// Store the CO definition if available
		if hasCO {
			tile["_coDefinition"] = coDef
		}

		// Get strImg from items.json (most accurate source)
		if itemDef, hasItem := itemsMap[itemName]; hasItem {
			if strImg, ok := itemDef["strImg"].(string); ok && strImg != "" {
				tile["strImg"] = strImg
			}
		}

		// Fallback to CO definition for strImg if not found in items.json
		if tile["strImg"] == nil && hasCO {
			if strImg, ok := coDef["strImg"].(string); ok && strImg != "" {
				tile["strImg"] = strImg
			}
		}

		// Get friendly name from CO definition
		if hasCO {
			if strNameFriendly, ok := coDef["strNameFriendly"].(string); ok {
				tile["strNameFriendly"] = strNameFriendly
			}
		}

		// Store installable definition as fallback
		if !hasCO {
			tile["_installableDefinition"] = installableDef
		}

		e.availableInstallableTiles = append(e.availableInstallableTiles, tile)
	}

	// Sort tiles by name
	sort.Slice(e.availableInstallableTiles, func(i, j int) bool {
		return utils.GetStringValue(e.availableInstallableTiles[i], "strName") < utils.GetStringValue(e.availableInstallableTiles[j], "strName")
	})

	if config.Verbose {
		log.Printf("Built installable palette with %d installable items from %s", len(e.availableInstallableTiles), installablesPath)
	}

	// Initialize filtered installable tiles
	e.filteredInstallableTiles = e.availableInstallableTiles
}

func (e *Editor) buildLoosePalette() {
	// Build loose items palette from items.json in the data directory
	dataPath := ""
	if config.GlobalConfig != nil {
		dataPath = config.GlobalConfig.Data
	}

	if dataPath == "" {
		log.Printf("Warning: Data path not set, cannot build loose palette")
		return
	}

	itemsPath := filepath.Join(dataPath, "items", "items.json")

	// Load items JSON file
	itemsData, err := loader.LoadJSON(itemsPath)
	if err != nil {
		log.Printf("Error loading items.json: %v", err)
		return
	}

	// Parse the JSON array
	itemsList, ok := itemsData.([]interface{})
	if !ok {
		log.Printf("Error: items.json is not an array")
		return
	}

	// Also load cooverlays.json to get CO definitions with *Loose suffix
	cooverlaysPath := filepath.Join(dataPath, "cooverlays", "cooverlays.json")
	cooverlaysData, err := loader.LoadJSON(cooverlaysPath)
	if err != nil {
		log.Printf("Error loading cooverlays.json: %v", err)
		return
	}

	cooverlaysList, ok := cooverlaysData.([]interface{})
	if !ok {
		log.Printf("Error: cooverlays.json is not an array")
		return
	}

	// Build a map of item name -> CO definition
	coMap := make(map[string]map[string]interface{})
	for _, coInterface := range cooverlaysList {
		if co, ok := coInterface.(map[string]interface{}); ok {
			coName := utils.GetStringValue(co, "strName")
			if coName != "" {
				coMap[coName] = co
			}
		}
	}

	e.availableLooseTiles = make([]map[string]interface{}, 0)
	seenItems := make(map[string]bool) // Track unique items

	// First, add all items from items.json
	for _, itemInterface := range itemsList {
		itemDef, ok := itemInterface.(map[string]interface{})
		if !ok {
			continue
		}

		strName := utils.GetStringValue(itemDef, "strName")
		if strName == "" {
			continue
		}

		// Skip ItmBG* items (background items)
		if len(strName) >= 5 && strName[:5] == "ItmBG" {
			continue
		}

		// Skip if we've already seen this item
		if seenItems[strName] {
			continue
		}
		seenItems[strName] = true

		// Create a tile template
		tile := make(map[string]interface{})
		tile["strName"] = strName
		tile["fX"] = 0.0
		tile["fY"] = 0.0
		tile["fRotation"] = 0.0
		tile["strID"] = e.generateGUID() // Generate a temporary ID

		// Get strImg from items.json
		if strImg, ok := itemDef["strImg"].(string); ok && strImg != "" {
			tile["strImg"] = strImg
		}

		// Try to get CO definition for this item
		if coDef, hasCO := coMap[strName]; hasCO {
			tile["_coDefinition"] = coDef

			// Get friendly name from CO definition
			if strNameFriendly, ok := coDef["strNameFriendly"].(string); ok {
				tile["strNameFriendly"] = strNameFriendly
			}

			// Debug: log size info for items with size
			if config.Verbose {
				if fSizeX, ok := coDef["fSizeX"].(float64); ok && fSizeX > 1 {
					if fSizeY, ok := coDef["fSizeY"].(float64); ok {
						log.Printf("Loaded loose item %s with size %.1fx%.1f", strName, fSizeX, fSizeY)
					}
				}
			}
		} else if config.Verbose {
			log.Printf("Warning: Loose item %s has no CO definition in cooverlays.json", strName)
		}

		e.availableLooseTiles = append(e.availableLooseTiles, tile)
	}

	// Second, add all *Loose objects from cooverlays.json
	for _, coInterface := range cooverlaysList {
		coDef, ok := coInterface.(map[string]interface{})
		if !ok {
			continue
		}

		strName := utils.GetStringValue(coDef, "strName")

		// Only include items with "Loose" suffix
		if !strings.HasSuffix(strName, "Loose") {
			continue
		}

		// Skip ItmBG* items (background items)
		if len(strName) >= 5 && strName[:5] == "ItmBG" {
			continue
		}

		// Skip if we've already seen this item
		if seenItems[strName] {
			continue
		}
		seenItems[strName] = true

		// Create a tile template
		tile := make(map[string]interface{})
		tile["strName"] = strName
		tile["fX"] = 0.0
		tile["fY"] = 0.0
		tile["fRotation"] = 0.0
		tile["strID"] = e.generateGUID() // Generate a temporary ID

		// Store the CO definition
		tile["_coDefinition"] = coDef

		// Get strImg from CO definition
		if strImg, ok := coDef["strImg"].(string); ok && strImg != "" {
			tile["strImg"] = strImg
		}

		// Get friendly name from CO definition
		if strNameFriendly, ok := coDef["strNameFriendly"].(string); ok {
			tile["strNameFriendly"] = strNameFriendly
		}

		e.availableLooseTiles = append(e.availableLooseTiles, tile)
	}

	// Sort tiles by name
	sort.Slice(e.availableLooseTiles, func(i, j int) bool {
		return utils.GetStringValue(e.availableLooseTiles[i], "strName") < utils.GetStringValue(e.availableLooseTiles[j], "strName")
	})

	if config.Verbose {
		log.Printf("Built loose palette with %d loose items from %s", len(e.availableLooseTiles), itemsPath)
	}

	// Initialize filtered loose tiles
	e.filteredLooseTiles = e.availableLooseTiles
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

	// Filter conduit tiles
	if e.availableConduitTiles != nil {
		if searchText == "" {
			e.filteredConduitTiles = e.availableConduitTiles
		} else {
			e.filteredConduitTiles = make([]map[string]interface{}, 0)
			for _, tile := range e.availableConduitTiles {
				tileName := strings.ToLower(utils.GetStringValue(tile, "strName"))
				if strings.Contains(tileName, searchText) {
					e.filteredConduitTiles = append(e.filteredConduitTiles, tile)
				}
			}
		}
	}

	// Filter installable tiles
	if e.availableInstallableTiles != nil {
		if searchText == "" {
			e.filteredInstallableTiles = e.availableInstallableTiles
		} else {
			e.filteredInstallableTiles = make([]map[string]interface{}, 0)
			for _, tile := range e.availableInstallableTiles {
				tileName := strings.ToLower(utils.GetStringValue(tile, "strName"))
				if strings.Contains(tileName, searchText) {
					e.filteredInstallableTiles = append(e.filteredInstallableTiles, tile)
				}
			}
		}
	}

	// Filter loose tiles
	if e.availableLooseTiles != nil {
		if searchText == "" {
			e.filteredLooseTiles = e.availableLooseTiles
		} else {
			e.filteredLooseTiles = make([]map[string]interface{}, 0)
			for _, tile := range e.availableLooseTiles {
				tileName := strings.ToLower(utils.GetStringValue(tile, "strName"))
				if strings.Contains(tileName, searchText) {
					e.filteredLooseTiles = append(e.filteredLooseTiles, tile)
				}
			}
		}
	}

	// Rebuild the tile palette UI
	e.rebuildTilePaletteUI()
}

func (e *Editor) buildPaletteScrollContent(numColumns int) {
	if e.tilePaletteScrollWrapper == nil {
		return
	}

	// Clear existing scroll content
	e.tilePaletteScrollWrapper.RemoveChildren()

	// Create new palette container
	e.tilePaletteContainer = widget.NewContainer(
		widget.ContainerOpts.Layout(widget.NewGridLayout(
			widget.GridLayoutOpts.Columns(numColumns),
			widget.GridLayoutOpts.Spacing(2, 2),
			widget.GridLayoutOpts.Padding(widget.NewInsetsSimple(5)),
		)),
	)

	// Store the current column count
	e.paletteCurrentColumns = numColumns

	// ScrollContainer
	scrollContainer := widget.NewScrollContainer(
		widget.ScrollContainerOpts.Content(e.tilePaletteContainer),
		widget.ScrollContainerOpts.StretchContentWidth(),
		widget.ScrollContainerOpts.Image(&widget.ScrollContainerImage{
			Idle: ebitenui_image.NewNineSliceColor(color.NRGBA{25, 25, 35, 255}),
			Mask: ebitenui_image.NewNineSliceColor(color.NRGBA{25, 25, 35, 255}),
		}),
	)
	e.tilePaletteScrollWrapper.AddChild(scrollContainer)

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

	e.tilePaletteScrollWrapper.AddChild(vSlider)
}

func (e *Editor) rebuildTilePaletteUI() {
	if e.tilePaletteContainer == nil {
		return
	}

	// Update layer choice visibility
	e.updateLayerChoiceVisibility()

	// Determine which tiles to show based on current layer
	var tilesToShow []map[string]interface{}
	var numColumns int
	if e.currentLayer == "wall" {
		tilesToShow = e.filteredWallTiles
		numColumns = 7
	} else if e.currentLayer == "conduit" {
		tilesToShow = e.filteredConduitTiles
		numColumns = 3
	} else if e.currentLayer == "installable" {
		tilesToShow = e.filteredInstallableTiles
		numColumns = 3
	} else if e.currentLayer == "loose" {
		tilesToShow = e.filteredLooseTiles
		numColumns = 3
	} else {
		tilesToShow = e.filteredTiles
		numColumns = 7
	}

	// Rebuild scroll content if column count changed
	if e.paletteCurrentColumns != numColumns {
		e.buildPaletteScrollContent(numColumns)
	} else {
		// Just clear existing buttons
		e.tilePaletteContainer.RemoveChildren()
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

		// For walls and conduits, extract sprite at index 13 for palette preview
		// Installed items use single sprites, no extraction needed
		isWall := len(tileName) >= 7 && tileName[:7] == "ItmWall"
		isConduit := len(tileName) >= 10 && tileName[:10] == "ItmConduit"
		if (isWall || isConduit) && tileImg != nil {
			tileImg = utils.ExtractSpriteFromSheet(tileImg, 13)
		}

		// Create button with tile image
		var btn *widget.Button
		if tileImg != nil {
			// Calculate button size based on image size (with padding)
			imgBounds := tileImg.Bounds()
			btnWidth := imgBounds.Dx() + 8 // 4px padding on each side
			btnHeight := imgBounds.Dy() + 8

			// Cap maximum size to 64x64 to prevent oversized buttons
			if btnWidth > 64 {
				btnWidth = 64
			}
			if btnHeight > 64 {
				btnHeight = 64
			}

			// Ensure minimum size of 20x20
			if btnWidth < 20 {
				btnWidth = 20
			}
			if btnHeight < 20 {
				btnHeight = 20
			}

			// Button with graphic image
			btn = widget.NewButton(
				widget.ButtonOpts.WidgetOpts(
					widget.WidgetOpts.MinSize(btnWidth, btnHeight),
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
		} else {
			// Button without graphic (image failed to load) - show text label instead
			btn = widget.NewButton(
				widget.ButtonOpts.WidgetOpts(
					widget.WidgetOpts.MinSize(20, 20),
				),
				widget.ButtonOpts.Image(&widget.ButtonImage{
					Idle:    ebitenui_image.NewNineSliceColor(color.NRGBA{50, 50, 60, 255}),
					Hover:   ebitenui_image.NewNineSliceColor(color.NRGBA{70, 100, 150, 255}),
					Pressed: ebitenui_image.NewNineSliceColor(color.NRGBA{60, 90, 130, 255}),
				}),
				widget.ButtonOpts.Text("?", e.smallFontFace, &widget.ButtonTextColor{
					Idle: color.NRGBA{255, 255, 255, 255},
				}),
				widget.ButtonOpts.ClickedHandler(func(args *widget.ButtonClickedEventArgs) {
					e.selectTileFromPalette(tile)
				}),
			)
		}

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
