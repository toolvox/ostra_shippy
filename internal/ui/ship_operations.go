package ui

import (
	"fmt"
	"log"
	"path/filepath"
	"strings"

	"github.com/sqweek/dialog"
	"github.com/user/ostra_shippy/internal/config"
	"github.com/user/ostra_shippy/internal/loader"
	"github.com/user/ostra_shippy/internal/utils"
)

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

func (e *Editor) saveShip() {
	if e.currentShip == nil || e.currentPath == "" {
		e.setStatus("No ship loaded!")
		return
	}

	// Update ship with edited values
	e.currentShip.PublicName = e.nameInput.GetText()
	e.currentShip.StrRegID = e.regIDInput.GetText()

	// Clean up duplicate tiles before saving
	e.removeDuplicateTiles()

	if err := loader.SaveShip(e.currentPath, e.currentShip); err != nil {
		e.setStatus(fmt.Sprintf("Error: %v", err))
		return
	}

	e.setStatus(fmt.Sprintf("Saved: %s", filepath.Base(e.currentPath)))
}

func (e *Editor) removeDuplicateTiles() {
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

	// Track tiles by position and layer (x,y,layer) -> list of tile indices
	type posKey struct {
		x, y  float64
		layer string // "floor", "wall", "conduit", "installable", or "loose"
	}
	positionMap := make(map[posKey][]int)

	// Build map of positions to tile indices (floor, wall, conduit, installable, and loose items)
	for i, itemInterface := range aItems {
		if item, ok := itemInterface.(map[string]interface{}); ok {
			strName := utils.GetStringValue(item, "strName")

			// Determine layer using the getTileLayer helper
			layer := getTileLayer(strName)
			if layer == "unknown" {
				continue
			}

			// Installable and loose items can stack at the same position - skip duplicate removal for them
			if layer == "installable" || layer == "loose" {
				continue
			}

			x := utils.GetFloatValue(item, "fX")
			y := utils.GetFloatValue(item, "fY")
			key := posKey{x, y, layer}
			positionMap[key] = append(positionMap[key], i)
		}
	}

	// Find duplicates and collect indices to remove
	indicesToRemove := make(map[int]bool)
	removedCOIDs := make(map[string]bool)

	for pos, indices := range positionMap {
		if len(indices) > 1 {
			if config.Verbose {
				log.Printf("Found %d %s tiles at position (%.0f, %.0f), keeping last one", len(indices), pos.layer, pos.x, pos.y)
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
								utils.GetStringValue(item, "strName"), strID, idx)
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

	if hoveredTile != nil {
		// Check if hovered tile and cursor tile are the same layer type
		hoveredName := utils.GetStringValue(hoveredTile, "strName")
		cursorName := utils.GetStringValue(e.cursorTile, "strName")

		hoveredLayer := getTileLayer(hoveredName)
		cursorLayer := getTileLayer(cursorName)

		if config.Verbose {
			hoveredX := utils.GetFloatValue(hoveredTile, "fX")
			hoveredY := utils.GetFloatValue(hoveredTile, "fY")
			hoveredID := utils.GetStringValue(hoveredTile, "strID")
			log.Printf("Hovered tile: %s (ID: %s, layer: %s) at (%.0f, %.0f)", hoveredName, hoveredID, hoveredLayer, hoveredX, hoveredY)
			log.Printf("Cursor tile: %s (layer: %s)", cursorName, cursorLayer)
			log.Printf("Cursor grid position: (%.0f, %.0f)", gridX, gridY)
		}

		// Only replace if same layer type
		if hoveredLayer == cursorLayer {
			// Replace the specific hovered tile (use its ID to ensure we replace the right one)
			hoveredTileID := utils.GetStringValue(hoveredTile, "strID")
			e.replaceTileAt(gridX, gridY, hoveredTileID)
		} else {
			// Different layers - place new tile without removing the hovered one
			if config.Verbose {
				log.Printf("Hovered tile layer (%s) differs from cursor layer (%s), placing new tile", hoveredLayer, cursorLayer)
			}
			e.placeNewTileAt(gridX, gridY)
		}
	} else {
		// Placing on empty space
		if config.Verbose {
			log.Printf("Placing new tile at empty position: (%.0f, %.0f)", gridX, gridY)
		}
		e.placeNewTileAt(gridX, gridY)
	}
}

// getTileLayer determines the layer type of a tile based on its name
func getTileLayer(tileName string) string {
	if strings.HasSuffix(tileName, "Loose") {
		return "loose"
	}
	if len(tileName) >= 8 && tileName[:8] == "ItmFloor" {
		return "floor"
	} else if len(tileName) >= 7 && tileName[:7] == "ItmWall" {
		return "wall"
	} else if len(tileName) >= 10 && tileName[:10] == "ItmConduit" {
		return "conduit"
	} else if strings.HasPrefix(tileName, "Itm") {
		return "installable"
	}
	return "unknown"
}

func (e *Editor) placeNewTileAt(tileX, tileY float64) {
	if e.currentShip == nil || e.cursorTile == nil {
		if config.Verbose {
			log.Printf("placeNewTileAt failed: currentShip=%v, cursorTile=%v", e.currentShip != nil, e.cursorTile != nil)
		}
		return
	}

	// Check if we're painting on the same position as last frame - skip to avoid duplicate undo actions
	if e.lastPaintedValid && e.lastPaintedX == tileX && e.lastPaintedY == tileY {
		return
	}

	// Update last painted position
	e.lastPaintedX = tileX
	e.lastPaintedY = tileY
	e.lastPaintedValid = true

	if config.Verbose {
		log.Printf("=== New Tile Placement: %s at (%.0f, %.0f) ===",
			utils.GetStringValue(e.cursorTile, "strName"), tileX, tileY)
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

	// Create new CO for the tile
	newCO := make(map[string]interface{})
	newCO["strID"] = newGUID

	// Get cursor tile's CO definition
	cursorTileID := utils.GetStringValue(e.cursorTile, "strID")
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

	// Populate new CO
	if cursorCO != nil {
		// Copy all fields from cursor tile's CO except strID
		for k, v := range cursorCO {
			if k != "strID" {
				newCO[k] = v
			}
		}
		if config.Verbose {
			log.Printf("Created new CO from existing tile CO")
		}
	} else {
		// Cursor tile is from palette, use the stored CO definition from JSON
		if coDefinition, ok := e.cursorTile["_coDefinition"].(map[string]interface{}); ok {
			// Copy all relevant fields from the palette CO definition
			if strCODef, ok := coDefinition["strName"].(string); ok {
				newCO["strCODef"] = strCODef
			}
			if strImg, ok := coDefinition["strImg"].(string); ok {
				newCO["strIMGPreview"] = strImg
			}
			// Copy size information for multi-tile items
			if fSizeX, ok := coDefinition["fSizeX"].(float64); ok {
				newCO["fSizeX"] = fSizeX
			}
			if fSizeY, ok := coDefinition["fSizeY"].(float64); ok {
				newCO["fSizeY"] = fSizeY
			}
			// Copy other useful CO fields
			if strCOBase, ok := coDefinition["strCOBase"].(string); ok {
				newCO["strCOBase"] = strCOBase
			}
			if strNameFriendly, ok := coDefinition["strNameFriendly"].(string); ok {
				newCO["strNameFriendly"] = strNameFriendly
			}
			if config.Verbose {
				log.Printf("Created new CO from palette: strCODef=%s, fSizeX=%.1f, fSizeY=%.1f",
					utils.GetStringValue(newCO, "strCODef"),
					utils.GetFloatValue(newCO, "fSizeX"),
					utils.GetFloatValue(newCO, "fSizeY"))
			}
		} else {
			// Fallback: just set image
			if strImg, ok := e.cursorTile["strImg"].(string); ok && strImg != "" {
				newCO["strIMGPreview"] = strImg
			}
			if config.Verbose {
				log.Printf("Created new CO with image only: %s", utils.GetStringValue(e.cursorTile, "strImg"))
			}
		}
	}

	// Add tile to aItems
	newItemIndex := len(aItems)
	aItems = append(aItems, newTile)
	e.currentShip.RawData["aItems"] = aItems

	// Add CO to aCOs
	aCOs = append(aCOs, newCO)
	e.currentShip.RawData["aCOs"] = aCOs

	// Record undo action for new tile placement (to undo, we delete it)
	e.pushUndo(EditorAction{
		actionType: "delete",
		tileData:   deepCopyMap(newTile),
		coData:     deepCopyMap(newCO),
		index:      newItemIndex,
	})

	e.setStatus(fmt.Sprintf("Placed new tile at (%.0f, %.0f)", tileX, tileY))
	if config.Verbose {
		log.Printf("New tile placement complete: GUID=%s, total items=%d", newGUID, len(aItems))
	}
}

func (e *Editor) deleteTileAt(tile map[string]interface{}) {
	if e.currentShip == nil || tile == nil {
		return
	}

	targetTileID := getStringValue(tile, "strID")
	if targetTileID == "" {
		return
	}

	// Get aItems array
	aItems, ok := e.currentShip.RawData["aItems"].([]interface{})
	if !ok {
		e.setStatus("Error: aItems not found")
		return
	}

	// Get aCOs array
	aCOs, ok := e.currentShip.RawData["aCOs"].([]interface{})
	if !ok {
		e.setStatus("Error: aCOs not found")
		return
	}

	// Find the tile by ID
	tileIndex := -1
	var tileData map[string]interface{}
	for i, itemInterface := range aItems {
		if item, ok := itemInterface.(map[string]interface{}); ok {
			if strID, ok := item["strID"].(string); ok && strID == targetTileID {
				tileIndex = i
				// Deep copy tile data for undo
				tileData = make(map[string]interface{})
				for k, v := range item {
					tileData[k] = v
				}
				break
			}
		}
	}

	if tileIndex < 0 {
		e.setStatus("Error: Could not find tile to delete")
		return
	}

	// Find and save the corresponding CO
	var coData map[string]interface{}
	coIndex := -1
	for i, coInterface := range aCOs {
		if co, ok := coInterface.(map[string]interface{}); ok {
			if strID, ok := co["strID"].(string); ok && strID == targetTileID {
				coIndex = i
				// Deep copy CO data for undo
				coData = make(map[string]interface{})
				for k, v := range co {
					coData[k] = v
				}
				break
			}
		}
	}

	// Record undo action before deletion
	e.pushUndo(EditorAction{
		actionType: "delete",
		tileData:   tileData,
		coData:     coData,
		index:      tileIndex,
	})

	// Remove tile from aItems
	aItems = append(aItems[:tileIndex], aItems[tileIndex+1:]...)
	e.currentShip.RawData["aItems"] = aItems

	// Remove CO from aCOs
	if coIndex >= 0 {
		aCOs = append(aCOs[:coIndex], aCOs[coIndex+1:]...)
		e.currentShip.RawData["aCOs"] = aCOs
	}

	if config.Verbose {
		log.Printf("Deleted tile: %s (ID: %s)", getStringValue(tileData, "strName"), targetTileID)
	}

	e.setStatus(fmt.Sprintf("Deleted: %s", getStringValue(tileData, "strName")))
}

func (e *Editor) replaceTileAt(tileX, tileY float64, targetTileID string) {
	if e.currentShip == nil || e.cursorTile == nil {
		if config.Verbose {
			log.Printf("replaceTileAt failed: currentShip=%v, cursorTile=%v", e.currentShip != nil, e.cursorTile != nil)
		}
		return
	}

	// Check if we're painting on the same position as last frame - skip to avoid duplicate undo actions
	if e.lastPaintedValid && e.lastPaintedX == tileX && e.lastPaintedY == tileY {
		return
	}

	// Update last painted position
	e.lastPaintedX = tileX
	e.lastPaintedY = tileY
	e.lastPaintedValid = true

	if config.Verbose {
		log.Printf("=== Tile Placement: %s at (%.0f, %.0f) ===",
			utils.GetStringValue(e.cursorTile, "strName"), tileX, tileY)
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
	var oldCOData map[string]interface{}

	// Find the CO that belongs to the old tile we're replacing
	if oldTileID != "" {
		for _, coInterface := range aCOs {
			if co, ok := coInterface.(map[string]interface{}); ok {
				if strID, ok := co["strID"].(string); ok && strID == oldTileID {
					targetCO = co
					// Save old CO data for undo
					oldCOData = make(map[string]interface{})
					for k, v := range co {
						oldCOData[k] = v
					}
					if config.Verbose {
						log.Printf("Found old CO to update in place: strID=%s", oldTileID)
					}
					break
				}
			}
		}
	}

	// Record undo action before making changes
	if tileIndex >= 0 {
		oldTileData := make(map[string]interface{})
		if oldTile, ok := aItems[tileIndex].(map[string]interface{}); ok {
			for k, v := range oldTile {
				oldTileData[k] = v
			}
		}
		e.pushUndo(EditorAction{
			actionType: "place",
			tileData:   oldTileData,
			coData:     oldCOData,
			index:      tileIndex,
		})
	}

	// If we found the old CO, update it with the cursor tile's data
	if targetCO != nil {
		// Get the cursor tile's CO definition (from palette or existing tile)
		cursorTileID := utils.GetStringValue(e.cursorTile, "strID")
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
					log.Printf("Updated CO in place with palette tile: strCODef=%s", utils.GetStringValue(targetCO, "strCODef"))
				}
			} else {
				// Fallback: just update image if no CO definition available
				if strImg, ok := e.cursorTile["strImg"].(string); ok && strImg != "" {
					targetCO["strIMGPreview"] = strImg
				}
				if config.Verbose {
					log.Printf("Updated CO in place with palette tile image only: %s", utils.GetStringValue(e.cursorTile, "strImg"))
				}
			}
		}

		// Update the CO's ID to match the new tile
		targetCO["strID"] = newGUID

		if config.Verbose {
			log.Printf("Updated CO ID from %s to %s", oldTileID, newGUID)
		}

		// Write back the aCOs array to ensure changes are persisted
		e.currentShip.RawData["aCOs"] = aCOs
	} else {
		if config.Verbose {
			log.Printf("WARNING: Could not find old CO to update for tile: %s", oldTileID)
		}
	}

	// Replace or append tile in aItems
	if tileIndex >= 0 {
		aItems[tileIndex] = newTile
		e.currentShip.RawData["aItems"] = aItems
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
