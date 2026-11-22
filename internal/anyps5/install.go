// SPDX-License-Identifier: GPL-2.0-only

package anyps5

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
)

// ErrNotInstalled reports that no AnyPS5 runtime satisfies a request.
var ErrNotInstalled = errors.New("anyps5 runtime is not installed")

// AssetRef records one downloaded asset.
type AssetRef struct {
	Name   string `json:"name"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}

// Manifest describes one installed runtime.
type Manifest struct {
	Format      string    `json:"format"`
	Version     string    `json:"version"`
	Tag         string    `json:"tag"`
	Target      string    `json:"target"`
	Host        string    `json:"host"`
	InstalledAt time.Time `json:"installedAt"`
	Relinker    AssetRef  `json:"relinker"`
	Libs        AssetRef  `json:"libs"`
	Source      string    `json:"source"`
}

// Runtime is an installed AnyPS5 conversion toolchain and library set.
type Runtime struct {
	Version   string
	Target    string
	Dir       string
	Relinker  string
	LibsDir   string
	Manifest  Manifest
	Installed bool
}

// InstallResult reports what Install did.
type InstallResult struct {
	Version          string
	Target           string
	Dir              string
	Relinker         string
	AlreadyInstalled bool
}

// ProgressFunc receives download progress. total is -1 when unknown.
type ProgressFunc func(done, total int64)

// RuntimeDir is the directory of one version and target.
func RuntimeDir(version, target string) (string, error) {
	versions, err := VersionsDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(versions, strings.TrimPrefix(version, "v"), target), nil
}

// Install downloads and installs one release for target.
func Install(ctx context.Context, client *Client, target, version string, force bool, progress ProgressFunc) (*InstallResult, error) {
	target, err := NormalizeTarget(target)
	if err != nil {
		return nil, err
	}
	release, err := resolveRelease(ctx, client, version)
	if err != nil {
		return nil, err
	}
	bundle, err := SelectBundle(release, target)
	if err != nil {
		return nil, err
	}
	finalDir, err := RuntimeDir(release.Version(), target)
	if err != nil {
		return nil, err
	}
	relinkerName := "relinker"
	if runtime.GOOS == "windows" {
		relinkerName += ".exe"
	}
	if !force {
		if existing, err := openRuntime(finalDir, release.Version(), target); err == nil {
			_ = setActive(target, release.Version())
			return &InstallResult{
				Version: release.Version(), Target: target, Dir: finalDir,
				Relinker: existing.Relinker, AlreadyInstalled: true,
			}, nil
		}
	}

	cache, err := CacheDir()
	if err != nil {
		return nil, err
	}
	downloads := filepath.Join(cache, "downloads")
	if err := os.MkdirAll(downloads, 0o755); err != nil {
		return nil, err
	}
	relinkerFile, relinkerSum, err := fetch(ctx, client, bundle.Relinker, downloads, progress)
	if err != nil {
		return nil, err
	}
	libsFile, libsSum, err := fetch(ctx, client, bundle.Libs, downloads, progress)
	if err != nil {
		return nil, err
	}

	versions, err := VersionsDir()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(versions, 0o755); err != nil {
		return nil, err
	}
	stage, err := os.MkdirTemp(versions, ".stage-"+release.Version()+"-"+target+"-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(stage)

	if err := copyFile(relinkerFile, filepath.Join(stage, "bin", relinkerName), 0o755); err != nil {
		return nil, err
	}
	if err := extractLibs(libsFile, filepath.Join(stage, "libs")); err != nil {
		return nil, fmt.Errorf("extract %s: %w", bundle.Libs.Name, err)
	}
	if err := verifyLibs(filepath.Join(stage, "libs")); err != nil {
		return nil, err
	}
	if len(bundle.Docs) > 0 {
		docs := filepath.Join(stage, "docs")
		for _, asset := range bundle.Docs {
			file, sum, err := fetch(ctx, client, asset, downloads, nil)
			if err != nil {
				return nil, err
			}
			if err := copyFile(file, filepath.Join(docs, asset.Name), 0o644); err != nil {
				os.Remove(file)
				return nil, err
			}
			_ = sum
			os.Remove(file)
		}
	}

	manifest := Manifest{
		Format:      "anygame.runtime/1",
		Version:     release.Version(),
		Tag:         release.Tag,
		Target:      target,
		Host:        runtime.GOOS,
		InstalledAt: time.Now().UTC(),
		Relinker:    AssetRef{Name: bundle.Relinker.Name, Size: bundle.Relinker.Size, SHA256: relinkerSum},
		Libs:        AssetRef{Name: bundle.Libs.Name, Size: bundle.Libs.Size, SHA256: libsSum},
		Source:      release.HTMLURL,
	}
	if err := writeJSON(filepath.Join(stage, "runtime.json"), manifest); err != nil {
		return nil, err
	}

	if err := os.MkdirAll(filepath.Dir(finalDir), 0o755); err != nil {
		return nil, err
	}
	if force {
		backup := finalDir + ".old-" + fmt.Sprint(time.Now().UnixNano())
		if _, statErr := os.Stat(finalDir); statErr == nil {
			if err := os.Rename(finalDir, backup); err != nil {
				return nil, fmt.Errorf("replace %s: %w", finalDir, err)
			}
			defer os.RemoveAll(backup)
		}
	}
	if err := os.Rename(stage, finalDir); err != nil {
		return nil, fmt.Errorf("install to %s: %w", finalDir, err)
	}
	if err := setActive(target, release.Version()); err != nil {
		return nil, err
	}
	return &InstallResult{
		Version: release.Version(), Target: target, Dir: finalDir,
		Relinker: filepath.Join(finalDir, "bin", relinkerName),
	}, nil
}

func resolveRelease(ctx context.Context, client *Client, version string) (*Release, error) {
	version = strings.TrimSpace(version)
	if version == "" || strings.EqualFold(version, "latest") {
		return client.Latest(ctx)
	}
	return client.ByVersion(ctx, version)
}

func fetch(ctx context.Context, client *Client, asset Asset, dir string, progress ProgressFunc) (string, string, error) {
	destination := filepath.Join(dir, asset.Name)
	if ok, sum := fileMatches(destination, asset.Digest); ok {
		return destination, sum, nil
	}
	partial := destination + ".part"
	_ = os.Remove(partial)
	out, err := os.Create(partial)
	if err != nil {
		return "", "", err
	}
	cleanup := func() {
		out.Close()
		os.Remove(partial)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, asset.URL, nil)
	if err != nil {
		cleanup()
		return "", "", err
	}
	request.Header.Set("User-Agent", "anygame")
	if client.Token != "" && strings.HasPrefix(asset.URL, apiBase) {
		request.Header.Set("Authorization", "Bearer "+client.Token)
	}
	httpClient := client.HTTP
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	response, err := httpClient.Do(request)
	if err != nil {
		cleanup()
		return "", "", err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		cleanup()
		return "", "", fmt.Errorf("download %s: %s", asset.URL, response.Status)
	}
	total := response.ContentLength
	if total <= 0 {
		total = asset.Size
	}
	hash := sha256.New()
	writer := io.MultiWriter(out, hash)
	buffer := make([]byte, 1<<20)
	var done int64
	lastReport := time.Now()
	for {
		count, readErr := response.Body.Read(buffer)
		if count > 0 {
			if _, err := writer.Write(buffer[:count]); err != nil {
				cleanup()
				return "", "", err
			}
			done += int64(count)
			if progress != nil && time.Since(lastReport) > 150*time.Millisecond {
				progress(done, total)
				lastReport = time.Now()
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			cleanup()
			return "", "", readErr
		}
	}
	if progress != nil {
		progress(done, total)
	}
	if err := out.Close(); err != nil {
		os.Remove(partial)
		return "", "", err
	}
	sum := hex.EncodeToString(hash.Sum(nil))
	if expected := digestSHA(asset.Digest); expected != "" && !strings.EqualFold(expected, sum) {
		os.Remove(partial)
		return "", "", fmt.Errorf("checksum mismatch for %s: expected %s, got %s", asset.Name, expected, sum)
	}
	if asset.Size > 0 && done != asset.Size {
		os.Remove(partial)
		return "", "", fmt.Errorf("%s: downloaded %d bytes, release lists %d", asset.Name, done, asset.Size)
	}
	if err := os.Rename(partial, destination); err != nil {
		os.Remove(partial)
		return "", "", err
	}
	return destination, sum, nil
}

func digestSHA(digest string) string {
	digest = strings.TrimSpace(digest)
	digest = strings.TrimPrefix(digest, "sha256:")
	if len(digest) != 64 {
		return ""
	}
	return digest
}

func fileMatches(path, digest string) (bool, string) {
	expected := digestSHA(digest)
	if expected == "" {
		return false, ""
	}
	file, err := os.Open(path)
	if err != nil {
		return false, ""
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return false, ""
	}
	sum := hex.EncodeToString(hash.Sum(nil))
	return strings.EqualFold(sum, expected), sum
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
	return out.Close()
}

func extractLibs(archive, destination string) error {
	if strings.HasSuffix(strings.ToLower(archive), ".zip") {
		return extractZipLibs(archive, destination)
	}
	file, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer file.Close()
	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gzipReader.Close()
	reader := tar.NewReader(gzipReader)
	for {
		header, err := reader.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		name := path.Clean(strings.ReplaceAll(header.Name, "\\", "/"))
		if name == "." || strings.HasPrefix(name, "../") || path.IsAbs(name) {
			return fmt.Errorf("archive entry escapes the destination: %q", header.Name)
		}
		if name != "libs" && !strings.HasPrefix(name, "libs/") {
			continue
		}
		target, err := safeJoin(destination, strings.TrimPrefix(name, "libs/"))
		if err != nil {
			return err
		}
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			mode := os.FileMode(header.Mode).Perm()
			if mode == 0 {
				mode = 0o644
			}
			out, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
			if err != nil {
				return err
			}
			if _, err := io.Copy(out, reader); err != nil {
				out.Close()
				return err
			}
			if err := out.Close(); err != nil {
				return err
			}
		case tar.TypeSymlink:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			if err := os.Symlink(header.Linkname, target); err != nil {
				return err
			}
		}
	}
}

func extractZipLibs(archive, destination string) error {
	reader, err := zip.OpenReader(archive)
	if err != nil {
		return err
	}
	defer reader.Close()
	for _, entry := range reader.File {
		name := path.Clean(strings.ReplaceAll(entry.Name, "\\", "/"))
		if name == "." || strings.HasPrefix(name, "../") || path.IsAbs(name) {
			return fmt.Errorf("archive entry escapes the destination: %q", entry.Name)
		}
		if name != "libs" && !strings.HasPrefix(name, "libs/") {
			continue
		}
		target, err := safeJoin(destination, strings.TrimPrefix(name, "libs/"))
		if err != nil {
			return err
		}
		if entry.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		}
		if entry.Mode()&os.ModeSymlink != 0 {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		in, err := entry.Open()
		if err != nil {
			return err
		}
		out, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, entry.Mode().Perm())
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
	return nil
}

func safeJoin(base, relative string) (string, error) {
	joined := filepath.Join(base, filepath.FromSlash(relative))
	absoluteBase, err := filepath.Abs(base)
	if err != nil {
		return "", err
	}
	absoluteJoined, err := filepath.Abs(joined)
	if err != nil {
		return "", err
	}
	if absoluteJoined != absoluteBase && !strings.HasPrefix(absoluteJoined, absoluteBase+string(os.PathSeparator)) {
		return "", fmt.Errorf("path escapes the destination: %q", relative)
	}
	return absoluteJoined, nil
}

func verifyLibs(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	count := 0
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(strings.ToLower(entry.Name()), ".prx") {
			count++
		}
	}
	if count == 0 {
		return fmt.Errorf("%s contains no .prx libraries", dir)
	}
	return nil
}

// RuntimeAt returns an installed runtime at a directory.
func RuntimeAt(dir, version, target string) (*Runtime, error) {
	return openRuntime(dir, version, target)
}

func openRuntime(dir, version, target string) (*Runtime, error) {
	manifestPath := filepath.Join(dir, "runtime.json")
	body, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, ErrNotInstalled
	}
	var manifest Manifest
	if err := json.Unmarshal(body, &manifest); err != nil {
		return nil, fmt.Errorf("%s: %w", manifestPath, err)
	}
	if manifest.Host != "" && manifest.Host != runtime.GOOS {
		return nil, ErrNotInstalled
	}
	name := "relinker"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	relinker := filepath.Join(dir, "bin", name)
	if _, err := os.Stat(relinker); err != nil {
		return nil, ErrNotInstalled
	}
	libs := filepath.Join(dir, "libs")
	if _, err := os.Stat(libs); err != nil {
		return nil, ErrNotInstalled
	}
	return &Runtime{
		Version: version, Target: target, Dir: dir, Relinker: relinker,
		LibsDir: libs, Manifest: manifest, Installed: true,
	}, nil
}

// Ensure resolves an installed runtime, preferring version then the active
// version then the newest installed one for target.
func Ensure(target, version string) (*Runtime, error) {
	var err error
	target, err = NormalizeTarget(target)
	if err != nil {
		return nil, err
	}
	if version != "" {
		version = strings.TrimPrefix(strings.TrimSpace(version), "v")
		dir, err := RuntimeDir(version, target)
		if err != nil {
			return nil, err
		}
		if runtime, err := openRuntime(dir, version, target); err == nil {
			return runtime, nil
		}
		return nil, fmt.Errorf("%w: AnyPS5 %s for %s; run `anygame update --version %s --target %s`", ErrNotInstalled, version, target, version, target)
	}
	state, err := loadState()
	if err != nil {
		return nil, err
	}
	if active := state.Active[target]; active != "" {
		dir, err := RuntimeDir(active, target)
		if err != nil {
			return nil, err
		}
		if runtime, err := openRuntime(dir, active, target); err == nil {
			return runtime, nil
		}
	}
	installed, err := List(false)
	if err != nil {
		return nil, err
	}
	for _, runtime := range installed {
		if runtime.Target == target {
			return runtime, nil
		}
	}
	return nil, fmt.Errorf("%w for %s; run `anygame update --target %s`", ErrNotInstalled, target, target)
}

// List returns installed runtimes, newest version first.
func List(withSummary bool) ([]*Runtime, error) {
	versions, err := VersionsDir()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(versions)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var result []*Runtime
	for _, versionEntry := range entries {
		if !versionEntry.IsDir() || strings.HasPrefix(versionEntry.Name(), ".") {
			continue
		}
		targets, err := os.ReadDir(filepath.Join(versions, versionEntry.Name()))
		if err != nil {
			continue
		}
		for _, targetEntry := range targets {
			if !targetEntry.IsDir() || strings.HasPrefix(targetEntry.Name(), ".") {
				continue
			}
			dir := filepath.Join(versions, versionEntry.Name(), targetEntry.Name())
			runtime, err := openRuntime(dir, versionEntry.Name(), targetEntry.Name())
			if err != nil {
				continue
			}
			result = append(result, runtime)
		}
	}
	sort.SliceStable(result, func(i, j int) bool {
		if result[i].Version != result[j].Version {
			return compareVersions(result[i].Version, result[j].Version) > 0
		}
		return result[i].Target < result[j].Target
	})
	return result, nil
}

func compareVersions(a, b string) int {
	as := strings.Split(strings.TrimPrefix(a, "v"), ".")
	bs := strings.Split(strings.TrimPrefix(b, "v"), ".")
	for index := 0; index < len(as) || index < len(bs); index++ {
		var av, bv int
		if index < len(as) {
			fmt.Sscanf(as[index], "%d", &av)
		}
		if index < len(bs) {
			fmt.Sscanf(bs[index], "%d", &bv)
		}
		if av != bv {
			if av < bv {
				return -1
			}
			return 1
		}
	}
	return 0
}

type stateFile struct {
	Format string            `json:"format"`
	Active map[string]string `json:"active"`
}

func statePath() (string, error) {
	home, err := Home()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "state.json"), nil
}

func loadState() (*stateFile, error) {
	state := &stateFile{Format: "anygame.state/1", Active: map[string]string{}}
	path, err := statePath()
	if err != nil {
		return nil, err
	}
	body, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return state, nil
		}
		return nil, err
	}
	if err := json.Unmarshal(body, state); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if state.Active == nil {
		state.Active = map[string]string{}
	}
	return state, nil
}

func setActive(target, version string) error {
	state, err := loadState()
	if err != nil {
		return err
	}
	state.Format = "anygame.state/1"
	state.Active[target] = strings.TrimPrefix(version, "v")
	path, err := statePath()
	if err != nil {
		return err
	}
	return writeJSON(path, state)
}

// ActiveVersion returns the active installed version for target, if any.
func ActiveVersion(target string) string {
	state, err := loadState()
	if err != nil {
		return ""
	}
	return state.Active[target]
}

func writeJSON(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	body, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	body = append(body, '\n')
	temporary := path + ".tmp"
	if err := os.WriteFile(temporary, body, 0o644); err != nil {
		return err
	}
	return os.Rename(temporary, path)
}
