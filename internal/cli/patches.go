// SPDX-License-Identifier: GPL-2.0-only

package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"anygame/internal/anyps5"
	"anygame/internal/game"
	"anygame/internal/patches"
)

func runPatches(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("patches", flag.ContinueOnError)
	flags.SetOutput(stderr)
	catalog := flags.String("catalog", os.Getenv("ANYGAME_PATCH_CATALOG"), "catalog URL, GitHub repository or local directory")
	jsonOut := flags.Bool("json", false, "print JSON")
	positionals, err := parsePositionals(flags, args, "json")
	if err != nil {
		return 2
	}
	if len(positionals) != 1 {
		return fail(stderr, fmt.Errorf("usage: anygame patches <game folder> --catalog <url|directory> [--json]"))
	}
	source := *catalog
	if source == "" {
		return fail(stderr, fmt.Errorf("no patch catalog configured: pass --catalog or set ANYGAME_PATCH_CATALOG"))
	}
	info, err := game.Discover(positionals[0])
	if err != nil {
		return fail(stderr, err)
	}
	if info.TitleID == "" || info.ContentVersion == "" {
		return fail(stderr, fmt.Errorf("%s has no titleId or contentVersion; the catalog cannot match it", info.Resources))
	}
	cache, err := anyps5.CacheDir()
	if err != nil {
		return fail(stderr, err)
	}
	result, err := patches.Find(ctx, source, filepath.Join(cache, "patch-catalog"), info.Resources, info.TitleID, info.ContentVersion)
	if err != nil {
		return fail(stderr, err)
	}
	if *jsonOut {
		writeJSON(stdout, result)
		return 0
	}
	fmt.Fprintf(stdout, "%s %s\nCatalog: %s\n\n", info.TitleName, info.TitleID, result.Source)
	if len(result.Matches) == 0 {
		fmt.Fprintf(stdout, "No patches for %s %s.\n", info.TitleID, info.ContentVersion)
		return 0
	}
	for _, match := range result.Matches {
		state := "compatible"
		if !match.Compatible {
			state = "incompatible: " + match.Reason
		}
		name := match.Name
		if name == "" {
			name = match.ID
		}
		fmt.Fprintf(stdout, "[%s] %s\n", state, name)
		if match.Category != "" || match.ID != "" {
			fmt.Fprintf(stdout, "  id: %s  category: %s\n", match.ID, match.Category)
		}
		if match.Description != "" {
			fmt.Fprintf(stdout, "  %s\n", match.Description)
		}
		if match.Source != "" {
			fmt.Fprintf(stdout, "  source: %s\n", match.Source)
		}
		fmt.Fprintf(stdout, "  patch: %s\n\n", match.Path)
	}
	return 0
}
