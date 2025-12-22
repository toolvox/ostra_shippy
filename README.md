# Ostra Shippy

A ship editor for the game Ostranauts built with Ebiten and Ebitenui.

## Features

- Load and edit Ostranauts ship JSON files
- Visual ship rendering with zoom and pan
- Edit ship name and registration ID
- Layer toggle for floor tiles
- Tile placement and replacement system
- Copy tiles with Q key
- Rotate tiles with R key
- Left-click to place copied tiles
- Automatic duplicate floor tile cleanup on save

## Requirements

- Go 1.18 or higher
- Ostranauts game installed (for image assets)

## Configuration

Create `config.json` in the executable directory:

```json
{
  "imagesPath": "C:/Program Files (x86)/Steam/steamapps/common/Ostranauts/Ostranauts_Data/StreamingAssets/images/"
}
```

## Usage

```bash
# Run normally
go run cmd/ostra_shippy/main.go

# Run with verbose logging
go run cmd/ostra_shippy/main.go -v
```

## Controls

- **Mouse wheel**: Zoom in/out
- **Middle mouse drag**: Pan view
- **Q**: Copy hovered tile to cursor
- **R**: Rotate hovered tile
- **Left-click**: Place cursor tile (replaces hovered tile)

## Project Structure

```
ostra_shippy/
├── cmd/ostra_shippy/     # Main entry point
├── internal/
│   ├── config/           # Configuration management
│   ├── loader/           # Ship JSON loading/saving
│   ├── models/           # Data structures
│   ├── renderer/         # Ship visualization
│   └── ui/               # UI and editor logic
└── README.md
```
