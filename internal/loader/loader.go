package loader

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/user/ostra_shippy/internal/models"
)

// FindShipFiles searches for all ship JSON files in the saves directory
func FindShipFiles(savesPath string) ([]string, error) {
	var shipFiles []string

	err := filepath.Walk(savesPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip files we can't access
		}
		if !info.IsDir() && strings.HasSuffix(path, ".json") && strings.Contains(path, "ships") {
			shipFiles = append(shipFiles, path)
		}
		return nil
	})

	return shipFiles, err
}

// LoadShip loads a ship from a JSON file
func LoadShip(path string) (*models.Ship, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// Try to parse as array first (common format)
	var arrayData []map[string]interface{}
	if err := json.Unmarshal(data, &arrayData); err == nil {
		if len(arrayData) == 0 {
			return nil, fmt.Errorf("empty ship array")
		}
		return parseShipData(arrayData[0])
	}

	// If not an array, try as single object
	var rawData map[string]interface{}
	if err := json.Unmarshal(data, &rawData); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	return parseShipData(rawData)
}

func parseShipData(rawData map[string]interface{}) (*models.Ship, error) {
	ship := &models.Ship{RawData: rawData}

	// Extract basic fields
	if publicName, ok := rawData["publicName"].(string); ok {
		ship.PublicName = publicName
	}
	if name, ok := rawData["strName"].(string); ok {
		ship.StrName = name
	}
	if regID, ok := rawData["strRegID"].(string); ok {
		ship.StrRegID = regID
	}
	if wp, ok := rawData["nCurrentWaypoint"].(float64); ok {
		ship.NCurrentWaypoint = int(wp)
	}
	if te, ok := rawData["fTimeEngaged"].(float64); ok {
		ship.FTimeEngaged = te
	}
	if wm, ok := rawData["fWearManeuver"].(float64); ok {
		ship.FWearManeuver = wm
	}
	if wa, ok := rawData["fWearAccrued"].(float64); ok {
		ship.FWearAccrued = wa
	}
	if cos, ok := rawData["aCOs"].([]interface{}); ok {
		ship.ACOs = cos
	}

	return ship, nil
}

// SaveShip saves a ship to a JSON file, preserving original JSON structure and order
func SaveShip(path string, ship *models.Ship) error {
	// Update raw data with current values
	ship.RawData["publicName"] = ship.PublicName
	ship.RawData["strName"] = ship.StrName
	ship.RawData["strRegID"] = ship.StrRegID
	ship.RawData["nCurrentWaypoint"] = ship.NCurrentWaypoint
	ship.RawData["fTimeEngaged"] = ship.FTimeEngaged
	ship.RawData["fWearManeuver"] = ship.FWearManeuver
	ship.RawData["fWearAccrued"] = ship.FWearAccrued

	// Read original file to preserve field order
	originalData, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read original file: %w", err)
	}

	// Unmarshal to get original structure (preserves order in Go 1.18+)
	var originalArray []json.RawMessage
	if err := json.Unmarshal(originalData, &originalArray); err != nil {
		return fmt.Errorf("failed to parse original JSON: %w", err)
	}

	// Marshal updated data
	updatedJSON, err := json.Marshal(ship.RawData)
	if err != nil {
		return fmt.Errorf("failed to encode ship data: %w", err)
	}

	// Replace first element with updated data
	if len(originalArray) > 0 {
		originalArray[0] = updatedJSON
	} else {
		originalArray = []json.RawMessage{updatedJSON}
	}

	// Marshal to JSON with indentation
	data, err := json.MarshalIndent(originalArray, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to encode JSON: %w", err)
	}

	// Write to file
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}
