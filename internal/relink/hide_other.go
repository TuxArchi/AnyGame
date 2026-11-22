// SPDX-License-Identifier: GPL-2.0-only

//go:build !windows

package relink

import "os/exec"

func hideWindow(_ *exec.Cmd) {}
