package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/joho/godotenv"
	agentruntime "spire2mind/internal/agent"
	"spire2mind/internal/analyst"
	"spire2mind/internal/config"
	"spire2mind/internal/tui"
)

func main() {
	// Load .env.local if it exists (silently ignore if not found)
	_ = godotenv.Overload(".env.local")

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
	case "analyze":
		if err := runAnalyze(ctx, cfg, args[1:]); err != nil {
			fmt.Fprintf(os.Stderr, "analyze failed: %v\n", err)
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
	case "evolve":
		if err := runEvolve(ctx, cfg, args[1:]); err != nil {
			fmt.Fprintf(os.Stderr, "evolve failed: %v\n", err)
			os.Exit(1)
		}
	default:
		usage()
		os.Exit(1)
	}
}

func usage() {
	exe := filepath.Base(os.Args[0])
	fmt.Fprintf(os.Stderr, "usage: %s <command>\n\n", exe)
	fmt.Fprintln(os.Stderr, "commands:")
	fmt.Fprintln(os.Stderr, "  doctor                         Check system health")
	fmt.Fprintln(os.Stderr, "  play [--headless] [--attempts N] [--max-cycles N]")
	fmt.Fprintln(os.Stderr, "                                 Play the game with AI agent")
	fmt.Fprintln(os.Stderr, "  analyze <target>               Offline knowledge analysis")
	fmt.Fprintln(os.Stderr, "    targets: cards, enemies, archetypes, all")
	fmt.Fprintln(os.Stderr, "  evolve [--cycles N] [--games N] [--baseline N] [--checkpoint N]")
	fmt.Fprintln(os.Stderr, "                                 Run self-evolution loop")
}

func runAnalyze(ctx context.Context, cfg config.Config, args []string) error {
	a, err := analyst.New(cfg)
	if err != nil {
		return err
	}

	target := "all"
	if len(args) > 0 {
		target = strings.ToLower(args[0])
	}

	switch target {
	case "cards":
		return a.AnalyzeCards(ctx)
	case "enemies":
		return a.AnalyzeEnemies(ctx)
	case "archetypes":
		return a.AnalyzeArchetypes(ctx)
	case "run":
		// analyze run [run_id_or_dir]  — defaults to latest run
		hint := ""
		if len(args) > 1 {
			hint = args[1]
		}
		_, err := a.ReviewRun(ctx, hint)
		return err
	case "all":
		return a.RunAll(ctx)
	default:
		return fmt.Errorf("unknown analyze target %q (use: cards, enemies, archetypes, run [id], all)", target)
	}
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

func parseEvolveArgs(args []string) (cycles, gamesPerExp, baselineWindow, checkpointEvery int, err error) {
	cycles = 5
	gamesPerExp = 10
	baselineWindow = 10
	checkpointEvery = 10

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--cycles":
			if i+1 >= len(args) {
				return 0, 0, 0, 0, fmt.Errorf("--cycles requires a value")
			}
			cycles, err = strconv.Atoi(args[i+1])
			if err != nil || cycles <= 0 {
				return 0, 0, 0, 0, fmt.Errorf("invalid --cycles value %q", args[i+1])
			}
			i++
		case "--games":
			if i+1 >= len(args) {
				return 0, 0, 0, 0, fmt.Errorf("--games requires a value")
			}
			gamesPerExp, err = strconv.Atoi(args[i+1])
			if err != nil || gamesPerExp <= 0 {
				return 0, 0, 0, 0, fmt.Errorf("invalid --games value %q", args[i+1])
			}
			i++
		case "--baseline":
			if i+1 >= len(args) {
				return 0, 0, 0, 0, fmt.Errorf("--baseline requires a value")
			}
			baselineWindow, err = strconv.Atoi(args[i+1])
			if err != nil || baselineWindow <= 0 {
				return 0, 0, 0, 0, fmt.Errorf("invalid --baseline value %q", args[i+1])
			}
			i++
		case "--checkpoint":
			if i+1 >= len(args) {
				return 0, 0, 0, 0, fmt.Errorf("--checkpoint requires a value")
			}
			checkpointEvery, err = strconv.Atoi(args[i+1])
			if err != nil || checkpointEvery <= 0 {
				return 0, 0, 0, 0, fmt.Errorf("invalid --checkpoint value %q", args[i+1])
			}
			i++
		default:
			return 0, 0, 0, 0, fmt.Errorf("unknown argument %q", args[i])
		}
	}

	return cycles, gamesPerExp, baselineWindow, checkpointEvery, nil
}

func runEvolve(ctx context.Context, cfg config.Config, args []string) error {
	cycles, gamesPerExp, baselineWindow, checkpointEvery, err := parseEvolveArgs(args)
	if err != nil {
		return fmt.Errorf("parse evolve args: %w", err)
	}

	fmt.Printf("Spire2Mind Self-Evolution\n")
	fmt.Printf("  Cycles: %d | Games/experiment: %d | Baseline window: %d | Checkpoint every: %d\n",
		cycles, gamesPerExp, baselineWindow, checkpointEvery)

	evCfg := agentruntime.DefaultEvolutionConfig(cfg.RepoRoot)
	evCfg.Cycles = cycles
	evCfg.GamesPerExperiment = gamesPerExp
	evCfg.BaselineWindow = baselineWindow
	evCfg.CheckpointEvery = checkpointEvery

	// Initialize LLM provider for evolution edits
	if cfg.HasModelConfig() {
		llmProvider, err := analyst.NewLLMProvider(cfg)
		if err != nil {
			fmt.Printf("  Warning: LLM provider init failed: %v (using template-based edits)\n", err)
		} else {
			evCfg.LLMProvider = llmProvider
			fmt.Printf("  LLM Provider: %s\n", cfg.Model)
		}
	} else {
		fmt.Printf("  No LLM configured (using template-based edits)\n")
	}

	loop, err := agentruntime.NewEvolutionLoop(evCfg)
	if err != nil {
		return fmt.Errorf("create evolution loop: %w", err)
	}
	defer loop.Close()
	loop.SetStdout(os.Stdout)

	binary, err := agentruntime.BuildEvolverBinary(cfg.RepoRoot)
	if err != nil {
		return fmt.Errorf("find binary: %w", err)
	}

	return loop.Run(ctx, cycles, binary)
}
