package utils

import (
	"image"

	"github.com/hajimehoshi/ebiten/v2"
)

// ExtractSpriteFromSheet extracts a single sprite from a 4x4 sprite sheet
// index is 0-15, representing position in the sheet (0-indexed):
//
//	0  1  2  3
//	4  5  6  7
//	8  9 10 11
//
// 12 13 14 15
func ExtractSpriteFromSheet(sheet *ebiten.Image, index int) *ebiten.Image {
	if sheet == nil {
		return nil
	}

	sheetWidth := sheet.Bounds().Dx()
	sheetHeight := sheet.Bounds().Dy()

	// Calculate tile size (4x4 grid)
	tileWidth := sheetWidth / 4
	tileHeight := sheetHeight / 4

	// Calculate position in grid (0-indexed)
	row := index / 4
	col := index % 4

	// Extract the sub-image
	x := col * tileWidth
	y := row * tileHeight
	rect := image.Rect(x, y, x+tileWidth, y+tileHeight)

	return sheet.SubImage(rect).(*ebiten.Image)
}
