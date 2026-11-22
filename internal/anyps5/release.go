// SPDX-License-Identifier: GPL-2.0-only

package anyps5

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"sort"
	"strings"
	"time"
)

const (
	apiBase   = "https://api.github.com"
	repoOwner = "boykopovar"
	repoName  = "AnyPS5"
)

// Asset is one file attached to a release.
type Asset struct {
	Name   string `json:"name"`
	Size   int64  `json:"size"`
	Digest string `json:"digest"`
	URL    string `json:"browser_download_url"`
}

// Release is one AnyPS5 release.
type Release struct {
	Tag         string    `json:"tag_name"`
	Name        string    `json:"name"`
	Draft       bool      `json:"draft"`
	Prerelease  bool      `json:"prerelease"`
	PublishedAt time.Time `json:"published_at"`
	HTMLURL     string    `json:"html_url"`
	Assets      []Asset   `json:"assets"`
}

// Version is the tag without its leading "v".
func (r *Release) Version() string {
	return strings.TrimPrefix(r.Tag, "v")
}

// AssetNamed returns the asset with the exact name.
func (r *Release) AssetNamed(name string) *Asset {
	for index := range r.Assets {
		if r.Assets[index].Name == name {
			return &r.Assets[index]
		}
	}
	return nil
}

// Client talks to the GitHub releases API for AnyPS5.
type Client struct {
	HTTP    *http.Client
	Token   string
	APIBase string
}

// NewClient returns a client with a sane timeout.
func NewClient() *Client {
	return &Client{
		HTTP:  &http.Client{Timeout: 60 * time.Second},
		Token: tokenFromEnv(),
	}
}

func tokenFromEnv() string {
	for _, name := range []string{"GITHUB_TOKEN", "GH_TOKEN"} {
		if value := os.Getenv(name); value != "" {
			return value
		}
	}
	return ""
}

func (c *Client) get(ctx context.Context, path string) (*Release, error) {
	body, err := c.getBytes(ctx, path)
	if err != nil {
		return nil, err
	}
	var release Release
	if err := json.Unmarshal(body, &release); err != nil {
		return nil, fmt.Errorf("decode release: %w", err)
	}
	return &release, nil
}

func (c *Client) getBytes(ctx context.Context, path string) ([]byte, error) {
	endpoint := path
	if !strings.HasPrefix(endpoint, "http://") && !strings.HasPrefix(endpoint, "https://") {
		base := c.APIBase
		if base == "" {
			base = apiBase
		}
		endpoint = strings.TrimRight(base, "/") + path
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	request.Header.Set("User-Agent", "anygame/"+runtime.GOOS)
	if c.Token != "" {
		request.Header.Set("Authorization", "Bearer "+c.Token)
	}
	client := c.HTTP
	if client == nil {
		client = http.DefaultClient
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("request %s: %w", endpoint, err)
	}
	defer response.Body.Close()
	body, readErr := io.ReadAll(io.LimitReader(response.Body, 16<<20))
	if readErr != nil {
		return nil, fmt.Errorf("read %s: %w", endpoint, readErr)
	}
	switch response.StatusCode {
	case http.StatusOK:
		return body, nil
	case http.StatusNotFound:
		return nil, fmt.Errorf("release not found: %s", endpoint)
	case http.StatusForbidden, http.StatusTooManyRequests:
		hint := "set GITHUB_TOKEN to raise the GitHub API rate limit"
		if c.Token != "" {
			hint = "the configured GITHUB_TOKEN may be invalid or out of quota"
		}
		return nil, fmt.Errorf("GitHub API refused the request (%s): %s", response.Status, hint)
	default:
		return nil, fmt.Errorf("GitHub API returned %s for %s", response.Status, endpoint)
	}
}

// Latest returns the newest non-draft, non-prerelease release.
func (c *Client) Latest(ctx context.Context) (*Release, error) {
	return c.get(ctx, "/repos/"+repoOwner+"/"+repoName+"/releases/latest")
}

// ByVersion returns a release by tag, with or without the leading "v".
func (c *Client) ByVersion(ctx context.Context, version string) (*Release, error) {
	version = strings.TrimPrefix(strings.TrimSpace(version), "v")
	if version == "" {
		return nil, errors.New("empty version")
	}
	release, err := c.get(ctx, "/repos/"+repoOwner+"/"+repoName+"/releases/tags/v"+url.PathEscape(version))
	if err == nil {
		return release, nil
	}
	// A tag without the "v" prefix is unusual but accepted.
	return c.get(ctx, "/repos/"+repoOwner+"/"+repoName+"/releases/tags/"+url.PathEscape(version))
}

// List returns up to limit releases, newest first.
func (c *Client) List(ctx context.Context, limit int) ([]Release, error) {
	if limit <= 0 || limit > 100 {
		limit = 30
	}
	body, err := c.getBytes(ctx, fmt.Sprintf("/repos/%s/%s/releases?per_page=%d", repoOwner, repoName, limit))
	if err != nil {
		return nil, err
	}
	var releases []Release
	if err := json.Unmarshal(body, &releases); err != nil {
		return nil, fmt.Errorf("decode releases: %w", err)
	}
	sort.SliceStable(releases, func(i, j int) bool {
		return releases[i].PublishedAt.After(releases[j].PublishedAt)
	})
	return releases, nil
}

// Bundle is the pair of release assets AnyGame needs for one target.
type Bundle struct {
	Release  *Release
	Relinker Asset
	Libs     Asset
	Docs     []Asset
}

// SelectBundle picks the relinker build for the host and the system libraries
// for target.
func SelectBundle(release *Release, target string) (*Bundle, error) {
	if release == nil {
		return nil, errors.New("nil release")
	}
	version := release.Version()
	host := runtime.GOOS

	relinkerName := "relinker-v" + version
	if host == "windows" {
		relinkerName = "relinker-v" + version + ".exe"
	}
	relinker := release.AssetNamed(relinkerName)
	if relinker == nil {
		// Fall back to any matching relinker asset for this host.
		for index := range release.Assets {
			asset := &release.Assets[index]
			if !strings.HasPrefix(asset.Name, "relinker-") {
				continue
			}
			isWindows := strings.HasSuffix(strings.ToLower(asset.Name), ".exe")
			if (host == "windows") == isWindows {
				relinker = asset
				break
			}
		}
	}
	if relinker == nil {
		return nil, fmt.Errorf("release %s has no relinker build for %s", release.Tag, host)
	}

	libs := release.AssetNamed("prx-" + target + "-v" + version + ".tar.gz")
	if libs == nil {
		libs = release.AssetNamed("prx-" + target + "-v" + version + ".zip")
	}
	if libs == nil {
		return nil, fmt.Errorf("release %s has no %s system libraries", release.Tag, target)
	}

	bundle := &Bundle{Release: release, Relinker: *relinker, Libs: *libs}
	for _, name := range []string{"USAGE.md", "INPUT_MAPPING.md", "README.es.md", "run-report.md"} {
		if asset := release.AssetNamed(name); asset != nil {
			bundle.Docs = append(bundle.Docs, *asset)
		}
	}
	return bundle, nil
}
