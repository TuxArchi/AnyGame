// SPDX-License-Identifier: GPL-2.0-only

// Package patches reads AnyPS5 patch catalogs (index.json + TOML files) and
// checks entries against a local game dump.
package patches

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"anygame/internal/self"
)

// Tested is one contributor test record.
type Tested struct {
	Stage       string `json:"stage"`
	Result      string `json:"result"`
	Evidence    string `json:"evidence"`
	Environment string `json:"environment,omitempty"`
	Date        string `json:"date,omitempty"`
}

// Entry is one catalog entry.
type Entry struct {
	TitleID        string            `json:"titleId"`
	ContentVersion string            `json:"contentVersion"`
	Patch          string            `json:"patch"`
	Targets        map[string]string `json:"targets"`
	ID             string            `json:"id,omitempty"`
	Name           string            `json:"name,omitempty"`
	Category       string            `json:"category,omitempty"`
	Description    string            `json:"description,omitempty"`
	Tested         []Tested          `json:"tested,omitempty"`
	Notes          string            `json:"notes,omitempty"`
	Contributors   []string          `json:"contributors,omitempty"`
	Source         string            `json:"source,omitempty"`
	Revision       int               `json:"revision,omitempty"`
}

// Catalog is index.json.
type Catalog struct {
	Format  int     `json:"format"`
	Entries []Entry `json:"entries"`
}

// Match is one entry that matches a title and version.
type Match struct {
	ID           string            `json:"id,omitempty"`
	Name         string            `json:"name,omitempty"`
	Category     string            `json:"category,omitempty"`
	Description  string            `json:"description,omitempty"`
	Notes        string            `json:"notes,omitempty"`
	Source       string            `json:"source,omitempty"`
	Contributors []string          `json:"contributors,omitempty"`
	Tested       []Tested          `json:"tested,omitempty"`
	Revision     int               `json:"revision,omitempty"`
	Patch        string            `json:"patch"`
	Path         string            `json:"path"`
	Compatible   bool              `json:"compatible"`
	Reason       string            `json:"reason,omitempty"`
	Targets      map[string]string `json:"targets"`
}

// Result is the response of a catalog lookup.
type Result struct {
	Source         string  `json:"source"`
	TitleID        string  `json:"titleId"`
	ContentVersion string  `json:"contentVersion"`
	Matches        []Match `json:"matches"`
}

// Load reads a catalog from a local index.json, a local directory, an HTTP
// index, a GitHub repository URL or a GitHub tree URL.
func Load(ctx context.Context, source, cacheDir string) (*Catalog, string, error) {
	if strings.TrimSpace(source) == "" {
		return nil, "", errors.New("no patch catalog configured")
	}
	index, base, err := resolveSource(source)
	if err != nil {
		return nil, "", err
	}
	var body []byte
	if strings.HasPrefix(index, "http://") || strings.HasPrefix(index, "https://") {
		body, err = fetch(ctx, index, cacheDir, "catalogs")
	} else {
		body, err = os.ReadFile(index)
	}
	if err != nil {
		return nil, "", err
	}
	var catalog Catalog
	if err := json.Unmarshal(body, &catalog); err != nil {
		return nil, "", fmt.Errorf("%s: %w", index, err)
	}
	if catalog.Format != 1 {
		return nil, "", fmt.Errorf("%s: unsupported catalog format %d", index, catalog.Format)
	}
	for number := range catalog.Entries {
		entry := &catalog.Entries[number]
		if entry.TitleID == "" || entry.ContentVersion == "" || entry.Patch == "" {
			return nil, "", fmt.Errorf("%s: entry %d is missing titleId, contentVersion or patch", index, number+1)
		}
		if err := safeRelative(entry.Patch); err != nil {
			return nil, "", fmt.Errorf("%s: entry %d: %w", index, number+1, err)
		}
		for target := range entry.Targets {
			if err := safeRelative(target); err != nil {
				return nil, "", fmt.Errorf("%s: entry %d target: %w", index, number+1, err)
			}
		}
	}
	return &catalog, base, nil
}

// Find returns the catalog entries for one title and version, with the patch
// file downloaded and the dump checked against the catalog hashes.
func Find(ctx context.Context, source, cacheDir, gameRoot, titleID, version string) (*Result, error) {
	catalog, base, err := Load(ctx, source, cacheDir)
	if err != nil {
		return nil, err
	}
	result := &Result{Source: source, TitleID: titleID, ContentVersion: version, Matches: []Match{}}
	for _, entry := range catalog.Entries {
		if entry.TitleID != titleID || entry.ContentVersion != version {
			continue
		}
		local, err := obtainPatch(ctx, base, entry.Patch, cacheDir)
		if err != nil {
			return nil, err
		}
		match := Match{
			ID: entry.ID, Name: entry.Name, Category: entry.Category, Description: entry.Description,
			Notes: entry.Notes, Source: entry.Source, Contributors: entry.Contributors, Tested: entry.Tested,
			Revision: entry.Revision, Patch: entry.Patch, Path: local, Targets: entry.Targets,
		}
		match.Compatible, match.Reason = checkTargets(gameRoot, entry.Targets)
		result.Matches = append(result.Matches, match)
	}
	sort.SliceStable(result.Matches, func(i, j int) bool {
		left, right := result.Matches[i], result.Matches[j]
		if left.Compatible != right.Compatible {
			return left.Compatible
		}
		return strings.ToLower(left.Name+" "+left.ID) < strings.ToLower(right.Name+" "+right.ID)
	})
	return result, nil
}

func checkTargets(root string, targets map[string]string) (bool, string) {
	if len(targets) == 0 {
		return false, "the catalog entry has no target hashes"
	}
	for target, expected := range targets {
		sum, err := hashTarget(root, target)
		if err != nil {
			return false, fmt.Sprintf("%s: %v", target, err)
		}
		if !strings.EqualFold(sum, expected) {
			return false, fmt.Sprintf("%s has a different SHA-256 (another dump or version)", target)
		}
	}
	return true, ""
}

func resolveSource(source string) (index, base string, err error) {
	source = strings.TrimSpace(source)
	if strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://") {
		parsed, err := url.Parse(source)
		if err != nil {
			return "", "", err
		}
		host := strings.ToLower(parsed.Host)
		segments := strings.Split(strings.Trim(parsed.Path, "/"), "/")
		if host == "github.com" && len(segments) >= 2 {
			owner, repo := segments[0], strings.TrimSuffix(segments[1], ".git")
			branch := "HEAD"
			sub := ""
			if len(segments) >= 4 && segments[2] == "tree" {
				branch = segments[3]
				sub = strings.Join(segments[4:], "/")
			}
			rawPath := "index.json"
			if sub != "" {
				rawPath = sub + "/index.json"
			}
			index = "https://raw.githubusercontent.com/" + owner + "/" + repo + "/" + branch + "/" + rawPath
		} else if strings.HasSuffix(parsed.Path, ".json") {
			index = source
		} else {
			parsed.Path = strings.TrimSuffix(parsed.Path, "/") + "/index.json"
			index = parsed.String()
		}
		parsedIndex, err := url.Parse(index)
		if err != nil {
			return "", "", err
		}
		parsedIndex.Path = strings.TrimSuffix(parsedIndex.Path, "/index.json")
		base = strings.TrimSuffix(parsedIndex.String(), "/")
		return index, base, nil
	}
	info, err := os.Stat(source)
	if err != nil {
		return "", "", err
	}
	if info.IsDir() {
		return filepath.Join(source, "index.json"), source, nil
	}
	return source, filepath.Dir(source), nil
}

func obtainPatch(ctx context.Context, base, relative, cacheDir string) (string, error) {
	if strings.HasPrefix(base, "http://") || strings.HasPrefix(base, "https://") {
		address := strings.TrimSuffix(base, "/") + "/" + strings.TrimPrefix(relative, "/")
		body, err := fetch(ctx, address, cacheDir, "patches")
		if err != nil {
			return "", err
		}
		sum := sha256.Sum256([]byte(address))
		directory := filepath.Join(cacheDir, "patches", hex.EncodeToString(sum[:12]))
		if err := os.MkdirAll(directory, 0o755); err != nil {
			return "", err
		}
		target := filepath.Join(directory, filepath.Base(relative))
		if err := os.WriteFile(target, body, 0o644); err != nil {
			return "", err
		}
		return target, nil
	}
	resolved, err := safeJoin(base, relative)
	if err != nil {
		return "", err
	}
	return resolved, nil
}

func fetch(ctx context.Context, address, cacheDir, kind string) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("User-Agent", "anygame-patches")
	client := &http.Client{Timeout: 30 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: %s", address, response.Status)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, 8<<20))
	if err != nil {
		return nil, err
	}
	if cacheDir != "" {
		sum := sha256.Sum256([]byte(address))
		directory := filepath.Join(cacheDir, kind, hex.EncodeToString(sum[:12]))
		if os.MkdirAll(directory, 0o755) == nil {
			_ = os.WriteFile(filepath.Join(directory, filepath.Base(request.URL.Path)), body, 0o644)
		}
	}
	return body, nil
}

// hashTarget hashes a catalog target. SELF containers are unwrapped because
// the catalog stores the hash of the unwrapped file the relinker receives.
func hashTarget(root, target string) (string, error) {
	candidates := []string{target}
	switch {
	case strings.HasSuffix(target, ".elf"):
		candidates = append(candidates, strings.TrimSuffix(target, ".elf")+".bin")
	case strings.HasSuffix(target, ".bin"):
		candidates = append(candidates, strings.TrimSuffix(target, ".bin")+".elf")
	}
	for _, candidate := range candidates {
		resolved, err := safeJoin(root, candidate)
		if err != nil {
			return "", err
		}
		body, err := os.ReadFile(resolved)
		if err != nil {
			continue
		}
		if len(body) >= 8 {
			magic := binaryLittleEndian(body[:4])
			if magic == 0x1D3D154F || magic == 0xEEF51454 {
				output, err := self.Unwrap(body, nil)
				if err != nil {
					return "", fmt.Errorf("%s is a SELF container that cannot be unwrapped: %w", candidate, err)
				}
				sum := sha256.Sum256(output.Output)
				return hex.EncodeToString(sum[:]), nil
			}
		}
		sum := sha256.Sum256(body)
		return hex.EncodeToString(sum[:]), nil
	}
	return "", errors.New("file is missing")
}

func binaryLittleEndian(data []byte) uint32 {
	return uint32(data[0]) | uint32(data[1])<<8 | uint32(data[2])<<16 | uint32(data[3])<<24
}

func hashFile(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func safeRelative(name string) error {
	if name == "" || path.IsAbs(name) || strings.Contains(name, "\\") || strings.Contains(name, "\x00") {
		return fmt.Errorf("invalid relative path %q", name)
	}
	for _, part := range strings.Split(name, "/") {
		if part == "" || part == "." || part == ".." {
			return fmt.Errorf("invalid relative path %q", name)
		}
	}
	return nil
}

func safeJoin(base, relative string) (string, error) {
	if err := safeRelative(relative); err != nil {
		return "", err
	}
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
		return "", fmt.Errorf("path escapes the catalog: %q", relative)
	}
	return absoluteJoined, nil
}
