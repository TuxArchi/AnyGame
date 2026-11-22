// SPDX-License-Identifier: GPL-2.0-only

package main

import (
	"context"
	"os"

	"anygame/internal/cli"
)

func main() {
	context := context.Background()
	os.Exit(cli.Run(context, os.Args[1:], os.Stdout, os.Stderr))
}
