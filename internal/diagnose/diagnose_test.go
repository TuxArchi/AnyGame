// SPDX-License-Identifier: GPL-2.0-only

package diagnose

import (
	"strings"
	"testing"
)

func TestAnalyzeRunningWithOptionalMissingLibraries(t *testing.T) {
	log := strings.Join([]string{
		"[src/VulkanDevice.cpp:2656 VulkanDevice] Vulkan device ready",
		"[src/VulkanDevice.cpp:2747 VulkanDevice] Swapchain created swapchain=0x1 extent=1280x720 images=3",
		"[module] dlopen: cannot load /app0/Media/Plugins/discord_game_sdk.prx (discord_game_sdk.prx: cannot open shared object file: No such file or directory)",
		"[PublishPlatformPlayStation.HasNetworkConnectivity] IsNetworkReachable is false",
		"\"RobustLogNotImplementedInput\": false,",
		"running normally",
	}, "\n")
	findings := Analyze(log)
	findings.TimedOut = true
	findings.finalize()
	if findings.Verdict != "running" {
		t.Fatalf("verdict = %s", findings.Verdict)
	}
	if !findings.Window || !findings.Started {
		t.Fatalf("window=%v started=%v", findings.Window, findings.Started)
	}
	if len(findings.MissingLibraries) != 1 || !strings.Contains(findings.MissingLibraries[0], "discord_game_sdk.prx") {
		t.Fatalf("missing libraries = %#v", findings.MissingLibraries)
	}
	if len(findings.NotImplemented) != 0 {
		t.Fatalf("configuration keys must not be reported as missing APIs: %#v", findings.NotImplemented)
	}
	if len(findings.Warnings) != 1 {
		t.Fatalf("warnings = %#v", findings.Warnings)
	}
}

func TestAnalyzeMissingLibraryWithoutWindow(t *testing.T) {
	log := "app.elf: error while loading shared libraries: libSceNpTrophy2.prx: cannot open shared object file: No such file or directory"
	findings := Analyze(log)
	findings.Exited = true
	findings.ExitCode = 127
	findings.finalize()
	if findings.Verdict != "missing-libraries" {
		t.Fatalf("verdict = %s", findings.Verdict)
	}
	if len(findings.MissingLibraries) != 1 || findings.MissingLibraries[0] != "libSceNpTrophy2.prx" {
		t.Fatalf("missing libraries = %#v", findings.MissingLibraries)
	}
}

func TestAnalyzeFatalCrash(t *testing.T) {
	log := "Swapchain created\nFATAL: unhandled exception 0xc0000005"
	findings := Analyze(log)
	findings.Exited = true
	findings.ExitCode = 1
	findings.finalize()
	if findings.Verdict != "crashed" {
		t.Fatalf("verdict = %s", findings.Verdict)
	}
	if len(findings.Fatal) != 1 {
		t.Fatalf("fatal = %#v", findings.Fatal)
	}
}

func TestAnalyzeMissingResources(t *testing.T) {
	log := "[src/Open.cpp:403 sceKernelOpen] open failed path=/app0/Media/Metadata/global-metadata.dat resolvedPath=/tmp/x result=0x80020002"
	findings := Analyze(log)
	if len(findings.MissingFiles) != 1 || findings.MissingFiles[0] != "/app0/Media/Metadata/global-metadata.dat" {
		t.Fatalf("missing files = %#v", findings.MissingFiles)
	}
}
