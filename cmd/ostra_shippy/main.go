package main

import (
	"flag"
	"log"
	"os"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/user/ostra_shippy/internal/config"
	"github.com/user/ostra_shippy/internal/ui"
)

var verbose bool

const (
	screenWidth  = 1280
	screenHeight = 720
)

type Game struct {
	editor *ui.Editor
}

func (g *Game) Update() error {
	g.editor.Update()
	return nil
}

func (g *Game) Draw(screen *ebiten.Image) {
	g.editor.Draw(screen)
}

func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	return screenWidth, screenHeight
}

func main() {
	// Parse command-line flags
	flag.BoolVar(&verbose, "v", false, "Enable verbose logging")
	flag.Parse()

	config.Verbose = verbose

	// Log current working directory
	cwd, _ := os.Getwd()
	if verbose {
		log.Printf("Starting Ostra Shippy from: %s", cwd)
	}

	// Load config
	if err := config.Load("conf.yaml"); err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	if verbose {
		log.Printf("Loaded config - Images path: %s", config.GlobalConfig.Images)
	}

	editor, err := ui.NewEditor()
	if err != nil {
		log.Fatalf("Failed to create editor: %v", err)
	}

	game := &Game{
		editor: editor,
	}

	ebiten.SetWindowSize(screenWidth, screenHeight)
	ebiten.SetWindowTitle("Ostra Shippy Editor")
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)

	if err := ebiten.RunGame(game); err != nil {
		log.Fatal(err)
	}
}
