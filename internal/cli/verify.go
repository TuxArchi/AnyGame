// SPDX-License-Identifier: GPL-2.0-only

package cli

import (
	"flag"
	"fmt"
	"io"

	"anygame/internal/verify"
)

func runVerify(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("verify", flag.ContinueOnError)
	flags.SetOutput(stderr)
	jsonOut := flags.Bool("json", false, "print JSON")
	positionals, err := parsePositionals(flags, args, "json")
	if err != nil {
		return 2
	}
	if len(positionals) != 1 {
		return fail(stderr, fmt.Errorf("usage: anygame verify <package directory> [--json]"))
	}
	report, err := verify.Run(positionals[0])
	if err != nil {
		return fail(stderr, err)
	}
	if *jsonOut {
		writeJSON(stdout, report)
		if !report.OK {
			return 1
		}
		return 0
	}
	fmt.Fprintf(stdout, "Verifying %s (%s)\n\n", report.Package, report.Target)
	target := report.Executable
	if len(target) > 0 {
		fmt.Fprintf(stdout, "Executable: %s\n\n", target)
	}
	for _, check := range report.Checks {
		fmt.Fprintf(stdout, "%-5s %-22s %s\n", statusLabel(check.Status), check.Name, check.Detail)
	}
	if len(report.MissingLibs) > 0 {
		fmt.Fprintln(stdout)
		fmt.Fprintln(stdout, "Missing libraries:")
		for _, name := range report.MissingLibs {
			fmt.Fprintf(stdout, "  - %s\n", name)
		}
	}
	fmt.Fprintln(stdout)
	fmt.Fprintf(stdout, "%d errors, %d warnings\n", report.Errors, report.Warnings)
	if !report.OK {
		return 1
	}
	return 0
}

func statusLabel(status verify.Status) string {
	switch status {
	case verify.StatusOK:
		return "OK"
	case verify.StatusWarn:
		return "WARN"
	default:
		return "FAIL"
	}
}
