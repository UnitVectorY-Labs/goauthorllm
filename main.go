package main

import (
	"fmt"
	"os"
	"runtime/debug"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/UnitVectorY-Labs/goauthorllm/internal/app"
	"github.com/UnitVectorY-Labs/goauthorllm/internal/config"
	"github.com/UnitVectorY-Labs/goauthorllm/internal/llm"
)

var Version = "dev"

func main() {
	if Version == "dev" || Version == "" {
		if bi, ok := debug.ReadBuildInfo(); ok {
			if bi.Main.Version != "" && bi.Main.Version != "(devel)" {
				Version = bi.Main.Version
			}
		}
	}

	args := os.Args[1:]
	if len(args) == 1 && (args[0] == "--version" || args[0] == "-version") {
		fmt.Println(Version)
		return
	}

	cfg, err := config.Load(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "config error: %v\n", err)
		os.Exit(1)
	}

	client := llm.NewClient(cfg.BaseURL, cfg.Model, cfg.APIKey, cfg.Timeout)
	model, err := app.NewModel(cfg, client)
	if err != nil {
		fmt.Fprintf(os.Stderr, "startup error: %v\n", err)
		os.Exit(1)
	}

	program := tea.NewProgram(&model, tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := program.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "runtime error: %v\n", err)
		os.Exit(1)
	}
}
