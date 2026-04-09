package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"spire2mind/internal/kbharvest"
)

func main() {
	var (
		outputPath      string
		inventoryPath   string
		wikiLimit       int
		includeSTS1Core bool
	)

	flag.StringVar(&outputPath, "output", filepath.Join("data", "kb", "external-docs.json"), "path to normalized external document corpus")
	flag.StringVar(&inventoryPath, "inventory", filepath.Join("data", "kb", "source-inventory.json"), "path to source inventory report")
	flag.IntVar(&wikiLimit, "wiki-limit", 40, "maximum number of Slay the Spire 2 wiki pages to harvest")
	flag.BoolVar(&includeSTS1Core, "include-sts1-core", true, "include curated Slay the Spire 1 core overview pages")
	flag.Parse()

	result, err := kbharvest.Harvest(context.Background(), kbharvest.Options{
		OutputPath:      outputPath,
		InventoryPath:   inventoryPath,
		WikiLimit:       wikiLimit,
		IncludeSTS1Core: includeSTS1Core,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "harvest failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("wrote %d docs to %s\n", len(result.Documents), result.OutputPath)
	fmt.Printf("wrote inventory to %s\n", result.InventoryPath)
}
