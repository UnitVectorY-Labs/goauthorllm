package main

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"runtime"
	"runtime/debug"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/UnitVectorY-Labs/goauthorllm/internal/app"
	"github.com/UnitVectorY-Labs/goauthorllm/internal/config"
	"github.com/UnitVectorY-Labs/goauthorllm/internal/llm"
)

var Version = "dev"
var semverRe = regexp.MustCompile(`^\d+\.\d+\.\d+`)

const (
	exitSuccess   = 0
	exitRuntime   = 1
	exitUsage     = 2
	exitNoChanges = 3
)

func buildVersionOutput(version string) string {
	normalized := version
	if semverRe.MatchString(normalized) && !strings.HasPrefix(normalized, "v") {
		normalized = "v" + normalized
	}
	return fmt.Sprintf("%s (%s, %s/%s)", normalized, runtime.Version(), runtime.GOOS, runtime.GOARCH)
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if Version == "dev" || Version == "" {
		if bi, ok := debug.ReadBuildInfo(); ok {
			if bi.Main.Version != "" && bi.Main.Version != "(devel)" {
				Version = bi.Main.Version
			}
		}
	}

	if len(args) == 1 && (args[0] == "--version" || args[0] == "-version") {
		fmt.Fprintf(stdout, "goauthorllm version %s\n", buildVersionOutput(Version))
		return exitSuccess
	}

	cfg, err := config.Load(args)
	if err != nil {
		fmt.Fprintf(stderr, "config error: %v\n", err)
		return exitUsage
	}

	if cfg.NonInteractive {
		result, err := app.RunNonInteractive(cfg, stdout)
		if err != nil {
			fmt.Fprintf(stderr, "operation failed: %v\n", err)
			return exitRuntime
		}
		if result.Changed == 0 {
			fmt.Fprintln(stderr, "operation completed without changing the document")
			return exitNoChanges
		}
		return exitSuccess
	}

	client := llm.NewClient(cfg.BaseURL, cfg.Model, cfg.APIKey, cfg.Timeout)
	model, err := app.NewModel(cfg, client)
	if err != nil {
		fmt.Fprintf(stderr, "startup error: %v\n", err)
		return exitRuntime
	}

	program := tea.NewProgram(&model, tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := program.Run(); err != nil {
		fmt.Fprintf(stderr, "runtime error: %v\n", err)
		return exitRuntime
	}
	return exitSuccess
}
