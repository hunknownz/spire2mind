package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"

	agentruntime "spire2mind/internal/agent"
	"spire2mind/internal/config"
	"spire2mind/internal/tui"
)

func main() {
	ctx := context.Background()
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "resolve cwd: %v\n", err)
		os.Exit(1)
	}

	cfg := config.Load(cwd)
	args := os.Args[1:]
	if len(args) == 0 {
		usage()
		os.Exit(1)
	}

	switch args[0] {
	case "doctor":
		if err := agentruntime.RunDoctor(ctx, cfg); err != nil {
			fmt.Fprintf(os.Stderr, "doctor failed: %v\n", err)
			os.Exit(1)
		}
	case "play":
		headless, attempts, maxCycles, err := parsePlayArgs(args[1:])
		if err != nil {
			fmt.Fprintf(os.Stderr, "parse play args: %v\n", err)
			os.Exit(1)
		}
		cfg.MaxAttempts = attempts
		cfg.MaxCycles = maxCycles
		if headless {
			if err := agentruntime.RunHeadless(ctx, cfg, os.Stdout); err != nil {
				fmt.Fprintf(os.Stderr, "play failed: %v\n", err)
				os.Exit(1)
			}
			return
		}

		model, err := tui.New(ctx, cfg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "init tui: %v\n", err)
			os.Exit(1)
		}
		defer model.Close()

		program := tea.NewProgram(model)
		if _, err := program.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "run tui: %v\n", err)
			os.Exit(1)
		}
	default:
		usage()
		os.Exit(1)
	}
}

func usage() {
	exe := filepath.Base(os.Args[0])
	fmt.Fprintf(os.Stderr, "usage: %s doctor | play [--headless] [--attempts N] [--max-cycles N]\n", exe)
	fmt.Fprintln(os.Stderr, "  --attempts 0 runs continuously until stopped")
	fmt.Fprintln(os.Stderr, "  --max-cycles 0 disables the autoplay cycle cap")
}

func parsePlayArgs(args []string) (bool, int, int, error) {
	headless := false
	attempts := 1
	maxCycles := -1

	for i := 0; i < len(args); i++ {
		switch arg := args[i]; {
		case arg == "--headless":
			headless = true
		case arg == "--attempts":
			if i+1 >= len(args) {
				return false, 0, 0, fmt.Errorf("--attempts requires a value")
			}

			parsed, err := strconv.Atoi(args[i+1])
			if err != nil || parsed < 0 {
				return false, 0, 0, fmt.Errorf("invalid --attempts value %q", args[i+1])
			}

			attempts = parsed
			i++
		case strings.HasPrefix(arg, "--attempts="):
			value := strings.TrimPrefix(arg, "--attempts=")
			parsed, err := strconv.Atoi(value)
			if err != nil || parsed < 0 {
				return false, 0, 0, fmt.Errorf("invalid --attempts value %q", value)
			}

			attempts = parsed
		case arg == "--max-cycles":
			if i+1 >= len(args) {
				return false, 0, 0, fmt.Errorf("--max-cycles requires a value")
			}

			parsed, err := strconv.Atoi(args[i+1])
			if err != nil || parsed < 0 {
				return false, 0, 0, fmt.Errorf("invalid --max-cycles value %q", args[i+1])
			}

			maxCycles = parsed
			i++
		case strings.HasPrefix(arg, "--max-cycles="):
			value := strings.TrimPrefix(arg, "--max-cycles=")
			parsed, err := strconv.Atoi(value)
			if err != nil || parsed < 0 {
				return false, 0, 0, fmt.Errorf("invalid --max-cycles value %q", value)
			}

			maxCycles = parsed
		default:
			return false, 0, 0, fmt.Errorf("unknown argument %q", arg)
		}
	}

	return headless, attempts, maxCycles, nil
}
