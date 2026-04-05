package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"goauthorllm/internal/app"
	"goauthorllm/internal/config"
	"goauthorllm/internal/llm"
)

func main() {
	cfg, err := config.Load(os.Args[1:])
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
