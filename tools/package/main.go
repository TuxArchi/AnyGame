// SPDX-License-Identifier: GPL-2.0-only

// Command package builds the AnyGame desktop app as a Linux AppImage and a
// Windows ZIP with AnyGame.exe. It downloads the official Electron runtime.
package main

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"time"
)

const electronRepo = "https://github.com/electron/electron/releases/download"

var (
	version      = flag.String("version", "0.1.0", "AnyGame version")
	electron     = flag.String("electron", "45.4.1", "Electron version")
	distDir      = flag.String("dist", "dist", "output directory")
	cacheDir     = flag.String("cache", ".work-tmp/ui-downloads", "download and extraction cache")
	only         = flag.String("only", "", "build only linux or windows")
	skipDownload = flag.Bool("skip-download", false, "fail instead of downloading Electron")
)

func main() {
	flag.Parse()
	root, err := repositoryRoot()
	must(err)
	cache, err := filepath.Abs(*cacheDir)
	must(err)
	dist, err := filepath.Abs(*distDir)
	must(err)
	*cacheDir = cache
	*distDir = dist
	must(os.MkdirAll(cache, 0o755))
	must(os.MkdirAll(dist, 0o755))

	targets := []string{"linux", "windows"}
	if *only != "" {
		targets = []string{*only}
	}

	linuxZip := filepath.Join(cache, "electron-v"+*electron+"-linux-x64.zip")
	windowsZip := filepath.Join(cache, "electron-v"+*electron+"-win32-x64.zip")
	if contains(targets, "linux") {
		must(ensureDownload(linuxZip, electronRepo+"/v"+*electron+"/electron-v"+*electron+"-linux-x64.zip"))
	}
	if contains(targets, "windows") {
		must(ensureDownload(windowsZip, electronRepo+"/v"+*electron+"/electron-v"+*electron+"-win32-x64.zip"))
	}

	sidecars := filepath.Join(cache, "sidecars-"+*version)
	must(os.RemoveAll(sidecars))
	must(os.MkdirAll(sidecars, 0o755))
	fmt.Println("Building the anygame CLI sidecars…")
	must(buildSidecar(root, sidecars, "linux", "anygame"))
	must(buildSidecar(root, sidecars, "windows", "anygame.exe"))

	for _, target := range targets {
		switch target {
		case "linux":
			archive := filepath.Join(cache, "electron-linux-"+*electron)
			must(extractZip(linuxZip, archive))
			output := filepath.Join(dist, "AnyGame-"+*version+"-x86_64.AppImage")
			must(buildLinux(root, archive, sidecars, output))
			report(output)
		case "windows":
			archive := filepath.Join(cache, "electron-windows-"+*electron)
			must(extractZip(windowsZip, archive))
			output := filepath.Join(dist, "AnyGame-"+*version+"-windows-x64.zip")
			must(buildWindows(root, archive, sidecars, output))
			report(output)
		default:
			must(fmt.Errorf("unknown target %q", target))
		}
	}
}

func repositoryRoot() (string, error) {
	current, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(current, "go.mod")); err == nil {
			if _, err := os.Stat(filepath.Join(current, "ui", "package.json")); err == nil {
				return current, nil
			}
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", fmt.Errorf("run this tool from the AnyGame repository")
		}
		current = parent
	}
}

func ensureDownload(destination, url string) error {
	if info, err := os.Stat(destination); err == nil && info.Size() > 0 {
		fmt.Printf("Using cached %s (%s)\n", filepath.Base(destination), human(info.Size()))
		return nil
	}
	if *skipDownload {
		return fmt.Errorf("%s is missing and --skip-download is set", destination)
	}
	fmt.Printf("Downloading %s\n", url)
	request, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	request.Header.Set("User-Agent", "anygame-packager")
	client := &http.Client{Timeout: 30 * time.Minute}
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: %s", url, response.Status)
	}
	temporary := destination + ".part"
	file, err := os.Create(temporary)
	if err != nil {
		return err
	}
	written, err := io.Copy(file, response.Body)
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		os.Remove(temporary)
		return err
	}
	if response.ContentLength > 0 && written != response.ContentLength {
		os.Remove(temporary)
		return fmt.Errorf("%s: got %d of %d bytes", filepath.Base(destination), written, response.ContentLength)
	}
	return os.Rename(temporary, destination)
}

func extractZip(archive, destination string) error {
	marker := filepath.Join(destination, ".extracted")
	if _, err := os.Stat(marker); err == nil {
		return nil
	}
	if err := os.RemoveAll(destination); err != nil {
		return err
	}
	reader, err := zip.OpenReader(archive)
	if err != nil {
		return err
	}
	defer reader.Close()
	for _, entry := range reader.File {
		cleaned := path.Clean(strings.ReplaceAll(entry.Name, "\\", "/"))
		if cleaned == "." || strings.HasPrefix(cleaned, "../") || path.IsAbs(cleaned) {
			return fmt.Errorf("archive entry escapes the destination: %q", entry.Name)
		}
		target := filepath.Join(destination, filepath.FromSlash(cleaned))
		if entry.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		mode := entry.Mode().Perm()
		if mode == 0 {
			mode = 0o644
		}
		in, err := entry.Open()
		if err != nil {
			return err
		}
		out, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
		if err != nil {
			in.Close()
			return err
		}
		_, copyErr := io.Copy(out, in)
		in.Close()
		closeErr := out.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
	}
	return os.WriteFile(marker, []byte("ok\n"), 0o644)
}

func buildSidecar(root, destination, target, name string) error {
	command := exec.Command("go", "build", "-trimpath",
		"-ldflags", "-s -w -X anygame/internal/cli.Version="+*version,
		"-o", filepath.Join(destination, name), "./cmd/anygame")
	command.Dir = root
	command.Env = append(os.Environ(), "CGO_ENABLED=0", "GOOS="+target, "GOARCH=amd64")
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	return command.Run()
}

func buildLinux(root, electronDir, sidecars, output string) error {
	appDir := filepath.Join(filepath.Dir(output), "AppDir-"+*version)
	must(os.RemoveAll(appDir))
	if err := copyDir(electronDir, appDir); err != nil {
		return err
	}
	_ = os.Remove(filepath.Join(appDir, ".extracted"))
	must(os.Remove(filepath.Join(appDir, "resources", "default_app.asar")))
	electronBinary := filepath.Join(appDir, "electron")
	if err := os.Rename(electronBinary, filepath.Join(appDir, "anygame")); err != nil {
		return err
	}
	if err := copyApp(root, filepath.Join(appDir, "resources", "app")); err != nil {
		return err
	}
	if err := copyFile(filepath.Join(sidecars, "anygame"), filepath.Join(appDir, "resources", "bin", "anygame"), 0o755); err != nil {
		return err
	}
	if err := copyFile(filepath.Join(root, "ui", "assets", "icon.png"), filepath.Join(appDir, "anygame.png"), 0o644); err != nil {
		return err
	}
	iconDir := filepath.Join(appDir, "usr", "share", "icons", "hicolor", "512x512", "apps")
	if err := os.MkdirAll(iconDir, 0o755); err != nil {
		return err
	}
	if err := copyFile(filepath.Join(root, "ui", "assets", "icon.png"), filepath.Join(iconDir, "anygame.png"), 0o644); err != nil {
		return err
	}
	desktop := desktopEntry()
	if err := os.WriteFile(filepath.Join(appDir, "anygame.desktop"), []byte(desktop), 0o644); err != nil {
		return err
	}
	applications := filepath.Join(appDir, "usr", "share", "applications")
	must(os.MkdirAll(applications, 0o755))
	if err := os.WriteFile(filepath.Join(applications, "anygame.desktop"), []byte(desktop), 0o644); err != nil {
		return err
	}
	appRun := "#!/bin/sh\n" +
		"HERE=\"$(dirname \"$(readlink -f \"$0\")\")\"\n" +
		"export ANYGAME_APPIMAGE=1\n" +
		"exec \"$HERE/anygame\" --no-sandbox \"$@\"\n"
	if err := os.WriteFile(filepath.Join(appDir, "AppRun"), []byte(appRun), 0o755); err != nil {
		return err
	}
	if err := copyFile(filepath.Join(root, "LICENSE"), filepath.Join(appDir, "LICENSE"), 0o644); err != nil {
		return err
	}

	tool, err := appImageTool()
	if err != nil {
		return err
	}
	_ = os.Remove(output)
	fmt.Println("Building the AppImage…")
	command := exec.Command(tool, "--no-appstream", appDir, output)
	command.Env = append(os.Environ(), "ARCH=x86_64", "APPIMAGE_EXTRACT_AND_RUN=1")
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	if err := command.Run(); err != nil {
		return fmt.Errorf("appimagetool: %w", err)
	}
	return os.RemoveAll(appDir)
}

func buildWindows(root, electronDir, sidecars, output string) error {
	appDir := filepath.Join(filepath.Dir(output), "AnyGame-"+*version+"-windows-x64")
	must(os.RemoveAll(appDir))
	if err := copyDir(electronDir, appDir); err != nil {
		return err
	}
	_ = os.Remove(filepath.Join(appDir, ".extracted"))
	must(os.Remove(filepath.Join(appDir, "resources", "default_app.asar")))
	must(os.Rename(filepath.Join(appDir, "electron.exe"), filepath.Join(appDir, "AnyGame.exe")))
	if err := copyApp(root, filepath.Join(appDir, "resources", "app")); err != nil {
		return err
	}
	if err := copyFile(filepath.Join(sidecars, "anygame.exe"), filepath.Join(appDir, "resources", "bin", "anygame.exe"), 0o755); err != nil {
		return err
	}
	if err := copyFile(filepath.Join(root, "LICENSE"), filepath.Join(appDir, "LICENSE.txt"), 0o644); err != nil {
		return err
	}
	readme := fmt.Sprintf("AnyGame %s\r\n\r\n"+
		"1. Extract this folder anywhere.\r\n"+
		"2. Run AnyGame.exe.\r\n"+
		"3. Open Runtimes and install the AnyPS5 runtime.\r\n"+
		"4. Add an extracted, decrypted PS5 title and convert it.\r\n\r\n"+
		"Runtimes and the library live in %%LOCALAPPDATA%%\\AnyGame.\r\n"+
		"AnyGame does not include game data, firmware or keys.\r\n", *version)
	if err := os.WriteFile(filepath.Join(appDir, "README.txt"), []byte(readme), 0o644); err != nil {
		return err
	}
	fmt.Println("Packing the Windows ZIP…")
	if err := zipDirectory(appDir, output); err != nil {
		return err
	}
	return os.RemoveAll(appDir)
}

func copyApp(root, destination string) error {
	source := filepath.Join(root, "ui")
	for _, name := range []string{"package.json", "main.js", "preload.js"} {
		if err := copyFile(filepath.Join(source, name), filepath.Join(destination, name), 0o644); err != nil {
			return err
		}
	}
	if err := copyDir(filepath.Join(source, "renderer"), filepath.Join(destination, "renderer")); err != nil {
		return err
	}
	if err := copyDir(filepath.Join(source, "assets"), filepath.Join(destination, "assets")); err != nil {
		return err
	}
	return copyDir(filepath.Join(source, "patches"), filepath.Join(destination, "patches"))
}

func desktopEntry() string {
	return "[Desktop Entry]\n" +
		"Type=Application\n" +
		"Name=AnyGame\n" +
		"Comment=PS5 game library and converter built on AnyPS5\n" +
		"Exec=anygame %U\n" +
		"Icon=anygame\n" +
		"Terminal=false\n" +
		"Categories=Game;Emulator;\n" +
		"X-AppImage-Version=" + *version + "\n"
}

func appImageTool() (string, error) {
	if value := os.Getenv("APPIMAGETOOL"); value != "" {
		return value, nil
	}
	candidates := []string{
		filepath.Join(*cacheDir, "appimagetool-x86_64.AppImage"),
		filepath.Join(*cacheDir, "appimagetool.AppImage"),
	}
	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && info.Size() > 0 {
			if info.Mode().Perm()&0o111 == 0 {
				_ = os.Chmod(candidate, 0o755)
			}
			return candidate, nil
		}
	}
	if !*skipDownload {
		destination := candidates[0]
		if err := ensureDownload(destination, "https://github.com/AppImage/appimagetool/releases/download/continuous/appimagetool-x86_64.AppImage"); err != nil {
			return "", err
		}
		_ = os.Chmod(destination, 0o755)
		return destination, nil
	}
	return "", fmt.Errorf("appimagetool not found; set APPIMAGETOOL")
}

func copyDir(source, destination string) error {
	entries, err := os.ReadDir(source)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(destination, 0o755); err != nil {
		return err
	}
	for _, entry := range entries {
		sourcePath := filepath.Join(source, entry.Name())
		destinationPath := filepath.Join(destination, entry.Name())
		info, err := os.Lstat(sourcePath)
		if err != nil {
			return err
		}
		switch {
		case info.Mode()&os.ModeSymlink != 0:
			link, err := os.Readlink(sourcePath)
			if err != nil {
				return err
			}
			_ = os.Remove(destinationPath)
			if err := os.Symlink(link, destinationPath); err != nil {
				return err
			}
		case info.IsDir():
			if err := copyDir(sourcePath, destinationPath); err != nil {
				return err
			}
		case info.Mode().IsRegular():
			if err := copyFile(sourcePath, destinationPath, info.Mode().Perm()); err != nil {
				return err
			}
		}
	}
	return nil
}

func copyFile(source, destination string, mode os.FileMode) error {
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return err
	}
	out, err := os.OpenFile(destination, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	return os.Chmod(destination, mode)
}

func zipDirectory(source, output string) error {
	_ = os.Remove(output)
	file, err := os.Create(output)
	if err != nil {
		return err
	}
	writer := zip.NewWriter(file)
	base := filepath.Dir(source)
	err = filepath.Walk(source, func(current string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(base, current)
		if err != nil {
			return err
		}
		relative = filepath.ToSlash(relative)
		if info.IsDir() {
			if current == source {
				return nil
			}
			_, err := writer.Create(relative + "/")
			return err
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		header.Name = relative
		header.Method = zip.Deflate
		entry, err := writer.CreateHeader(header)
		if err != nil {
			return err
		}
		in, err := os.Open(current)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(entry, in)
		in.Close()
		return copyErr
	})
	if err != nil {
		writer.Close()
		file.Close()
		return err
	}
	if err := writer.Close(); err != nil {
		file.Close()
		return err
	}
	return file.Close()
}

func report(path string) {
	info, err := os.Stat(path)
	if err != nil {
		fmt.Printf("missing %s: %v\n", path, err)
		return
	}
	body, err := os.ReadFile(path)
	sum := ""
	if err == nil {
		hash := sha256.Sum256(body)
		sum = hex.EncodeToString(hash[:])
	}
	fmt.Printf("\n%s\n  size   %s\n  sha256 %s\n", path, human(info.Size()), sum)
}

func human(value int64) string {
	units := []string{"B", "KiB", "MiB", "GiB"}
	number := float64(value)
	index := 0
	for number >= 1024 && index < len(units)-1 {
		number /= 1024
		index++
	}
	return fmt.Sprintf("%.1f %s", number, units[index])
}

func contains(values []string, value string) bool {
	for _, item := range values {
		if item == value {
			return true
		}
	}
	return false
}

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
