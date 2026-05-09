package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
	"github.com/tetratelabs/wazero/sys"
)

// GameRegistry represents the structure of your github-hosted registry.json
type GameRegistry struct {
	Games map[string]GameInfo `json:"games"`
}

type GameInfo struct {
	Name        string `json:"name"`
	Author      string `json:"author"`
	Description string `json:"description"`
	WasmURL     string `json:"wasm_url"`
}

// For local testing, we fallback to a local file if the URL isn't set.
const DefaultRegistryURL = "http://localhost:8080/registry.json"

func main() {
	if len(os.Args) < 2 {
		interactiveSelect()
		return
	}

	command := os.Args[1]

	switch command {
	case "list":
		listGames()
	case "play":
		if len(os.Args) < 3 {
			fmt.Println("Usage: cli-games play <game_name>")
			return
		}
		playGame(os.Args[2])
	default:
		printHelp()
	}
}

func printHelp() {
	fmt.Println("🎮 CLI Games Launcher")
	fmt.Println("Usage:")
	fmt.Println("  cli-games list         - List available games")
	fmt.Println("  cli-games play <name>  - Play a game")
}

func fetchRegistry() (*GameRegistry, error) {
	// In production, this would be your raw.githubusercontent.com URL
	registryURL := os.Getenv("REGISTRY_URL")
	if registryURL == "" {
		registryURL = DefaultRegistryURL
	}

	var registry GameRegistry

	// Fallback to local file for easy prototyping
	if strings.HasPrefix(registryURL, "http://localhost") {
		ex, err := os.Executable()
		if err != nil {
			return nil, err
		}
		regPath := filepath.Join(filepath.Dir(ex), "..", "registry.json")
		data, err := os.ReadFile(regPath)
		if err != nil {
			return nil, fmt.Errorf("local registry.json not found: %v", err)
		}
		err = json.Unmarshal(data, &registry)
		return &registry, err
	}

	resp, err := http.Get(registryURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if err := json.NewDecoder(resp.Body).Decode(&registry); err != nil {
		return nil, err
	}

	return &registry, nil
}

func listGames() {
	registry, err := fetchRegistry()
	if err != nil {
		fmt.Printf("Error fetching registry: %v\n", err)
		return
	}

	fmt.Println("🎮 Available Games:")
	fmt.Println("---------------------------------------------------")
	for id, game := range registry.Games {
		fmt.Printf("📦 %s (by %s)\n", id, game.Author)
		fmt.Printf("   %s\n\n", game.Description)
	}
}

func interactiveSelect() {
	registry, err := fetchRegistry()
	if err != nil {
		fmt.Printf("Error fetching registry: %v\n", err)
		return
	}

	fmt.Println("🎮 PLAY CLI - Game Menu")
	fmt.Println("---------------------------------------------------")
	
	var gameIDs []string
	for id := range registry.Games {
		gameIDs = append(gameIDs, id)
	}

	for i, id := range gameIDs {
		game := registry.Games[id]
		fmt.Printf("[%d] %s (by %s) - %s\n", i+1, game.Name, game.Author, game.Description)
	}
	fmt.Println("---------------------------------------------------")
	fmt.Print("Select a game to play (or 'q' to quit): ")

	var input string
	fmt.Scanln(&input)

	if input == "q" || input == "Q" {
		return
	}

	var choice int
	_, err = fmt.Sscanf(input, "%d", &choice)
	if err != nil || choice < 1 || choice > len(gameIDs) {
		fmt.Println("Invalid selection.")
		return
	}

	playGame(gameIDs[choice-1])
}

func playGame(gameID string) {
	registry, err := fetchRegistry()
	if err != nil {
		fmt.Printf("Error fetching registry: %v\n", err)
		return
	}

	game, exists := registry.Games[gameID]
	if !exists {
		fmt.Printf("Game '%s' not found in registry.\n", gameID)
		return
	}

	// 1. Setup local cache directory (~/.cli-games/cache)
	homeDir, _ := os.UserHomeDir()
	cacheDir := filepath.Join(homeDir, ".cli-games", "cache")
	os.MkdirAll(cacheDir, 0755)

	wasmPath := filepath.Join(cacheDir, gameID+".wasm")

	// 2. Download if not cached
	if _, err := os.Stat(wasmPath); os.IsNotExist(err) {
		fmt.Printf("Downloading %s...\n", game.Name)
		err := downloadFile(wasmPath, game.WasmURL)
		if err != nil {
			fmt.Printf("Failed to download game: %v\n", err)
			return
		}
	}

	// 3. Read the WASM binary
	wasmBytes, err := os.ReadFile(wasmPath)
	if err != nil {
		fmt.Printf("Failed to read cached game: %v\n", err)
		return
	}

	// 4. Initialize WASM Runtime (Wazero)
	ctx := context.Background()
	r := wazero.NewRuntime(ctx)
	defer r.Close(ctx)

	// Mount WASI to allow the game to access Stdin/Stdout/Stderr, but NOT the file system!
	wasi_snapshot_preview1.MustInstantiate(ctx, r)

	// Configure the module to connect to the user's terminal
	config := wazero.NewModuleConfig().
		WithStdin(os.Stdin).
		WithStdout(os.Stdout).
		WithStderr(os.Stderr)

	fmt.Printf("\n--- Starting %s ---\n\n", game.Name)

	// 5. Execute the game!
	_, err = r.InstantiateWithConfig(ctx, wasmBytes, config)
	if err != nil {
		// WASI exits trigger an error in wazero, we filter out normal exits (code 0)
		if exitErr, ok := err.(*sys.ExitError); ok && exitErr.ExitCode() == 0 {
			// Clean exit
		} else {
			fmt.Printf("\nGame exited with error: %v\n", err)
		}
	}
	fmt.Println("\n--- Game Over ---")
}

func downloadFile(destPath string, url string) error {
	// For local file simulation
	if strings.HasPrefix(url, "file://") {
		localPath := strings.TrimPrefix(url, "file://")
		ex, err := os.Executable()
		if err != nil {
			return err
		}
		absPath := filepath.Join(filepath.Dir(ex), localPath)
		input, err := os.ReadFile(absPath)
		if err != nil { return err }
		return os.WriteFile(destPath, input, 0644)
	}

	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	out, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}
