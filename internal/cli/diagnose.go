// SPDX-License-Identifier: GPL-2.0-only

package cli

import (
	"flag"
	"fmt"
	"io"

	"anygame/internal/diagnose"
	"anygame/internal/verify"
)

func runDiagnose(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("diagnose", flag.ContinueOnError)
	flags.SetOutput(stderr)
	seconds := flags.Int("seconds", 15, "seconds to let the game run before stopping it")
	jsonOut := flags.Bool("json", false, "print JSON")
	positionals, err := parsePositionals(flags, args, "json")
	if err != nil {
		return 2
	}
	if len(positionals) != 1 {
		return fail(stderr, fmt.Errorf("usage: anygame diagnose <package directory> [--seconds 15] [--json]"))
	}
	report, verifyErr := verify.Run(positionals[0])
	if verifyErr != nil {
		return fail(stderr, verifyErr)
	}
	if !*jsonOut {
		if report.Errors > 0 {
			fmt.Fprintf(stderr, "Preflight: %d errors, %d warnings\n", report.Errors, report.Warnings)
			for _, check := range report.Checks {
				if check.Status == verify.StatusFail {
					fmt.Fprintf(stderr, "  FAIL %s: %s\n", check.Name, check.Detail)
				}
			}
		}
		fmt.Fprintf(stderr, "Running %s for %d seconds...\n", report.Executable, *seconds)
	}
	findings, err := diagnose.Run(positionals[0], *seconds)
	if err != nil {
		return fail(stderr, err)
	}
	if *jsonOut {
		writeJSON(stdout, struct {
			Verify    *verify.Report     `json:"verify"`
			Diagnosis *diagnose.Findings `json:"diagnosis"`
		}{Verify: report, Diagnosis: findings})
	} else {
		printDiagnosis(stdout, findings)
	}
	switch findings.Verdict {
	case "running", "started", "exited":
		return 0
	default:
		return 1
	}
}

func printDiagnosis(w io.Writer, findings *diagnose.Findings) {
	fmt.Fprintln(w)
	fmt.Fprintf(w, "Verdict:  %s\n", findings.Verdict)
	fmt.Fprintf(w, "Window:   %v\n", findings.Window)
	fmt.Fprintf(w, "Started:  %v\n", findings.Started)
	if findings.TimedOut {
		fmt.Fprintf(w, "Stopped:  after %d s\n", findings.Seconds)
	} else {
		fmt.Fprintf(w, "Exit:     %d\n", findings.ExitCode)
	}
	fmt.Fprintf(w, "Log:      %s\n", findings.LogPath)
	if findings.Report != nil {
		if stage, ok := findings.Report["stage"]; ok {
			fmt.Fprintf(w, "Stage:    %v\n", stage)
		}
		if frames, ok := findings.Report["framesPresented"]; ok {
			fmt.Fprintf(w, "Frames:   %v\n", frames)
		}
	}
	if len(findings.MissingLibraries) > 0 {
		fmt.Fprintln(w, "\nMissing libraries:")
		for _, name := range findings.MissingLibraries {
			fmt.Fprintf(w, "  - %s\n", name)
		}
	}
	if len(findings.MissingFiles) > 0 {
		fmt.Fprintln(w, "\nMissing files:")
		for _, name := range findings.MissingFiles {
			fmt.Fprintf(w, "  - %s\n", name)
		}
	}
	if len(findings.NotImplemented) > 0 {
		fmt.Fprintln(w, "\nNot implemented:")
		for _, line := range findings.NotImplemented {
			fmt.Fprintf(w, "  - %s\n", line)
		}
	}
	if len(findings.Fatal) > 0 {
		fmt.Fprintln(w, "\nFatal:")
		for _, line := range findings.Fatal {
			fmt.Fprintf(w, "  - %s\n", line)
		}
	}
	if len(findings.Warnings) > 0 {
		fmt.Fprintln(w, "\nWarnings:")
		for _, line := range findings.Warnings {
			fmt.Fprintf(w, "  - %s\n", line)
		}
	}
	if findings.Verdict == "no-window" || findings.Verdict == "missing-libraries" || findings.Verdict == "crashed" {
		fmt.Fprintln(w, "\nLast log lines:")
		for _, line := range findings.Tail {
			fmt.Fprintf(w, "  %s\n", line)
		}
	}
}
