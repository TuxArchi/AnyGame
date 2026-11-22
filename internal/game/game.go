// SPDX-License-Identifier: GPL-2.0-only

// Package game discovers the layout of an extracted PS5 title.
package game

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Kind is the container format of the title executable.
type Kind string

const (
	KindELF   Kind = "elf"
	KindSELF  Kind = "self"
	KindOther Kind = "unknown"
)

// Info describes a discovered title.
type Info struct {
	Input           string
	Root            string
	Resources       string
	Eboot           string
	Kind            Kind
	EbootSize       int64
	ModuleDirs      []string
	ExtraModuleDirs []string
	ParamJSON       string
	ParamSFO        string
	TitleID         string
	TitleName       string
	ContentVersion  string
	Language        string
	Icon            string
	Warnings        []string
}

// Discover finds the executable, modules and metadata of a title.
func Discover(input string) (*Info, error) {
	absolute, err := filepath.Abs(input)
	if err != nil {
		return nil, err
	}
	info := &Info{Input: absolute}
	stat, err := os.Stat(absolute)
	if err != nil {
		return nil, err
	}
	if stat.IsDir() {
		info.Root = absolute
	} else {
		info.Root = filepath.Dir(absolute)
		info.Eboot = absolute
	}

	if info.Eboot == "" {
		if found, resources := findEboot(info.Root); found != "" {
			info.Eboot = found
			info.Resources = resources
		}
	}
	if info.Eboot == "" {
		return nil, fmt.Errorf("no eboot found under %s (looked for eboot.bin, app0/eboot.bin and eboot.elf)", info.Root)
	}
	if info.Resources == "" {
		info.Resources = filepath.Dir(info.Eboot)
	}
	info.Kind, info.EbootSize, err = sniff(info.Eboot)
	if err != nil {
		return nil, err
	}
	if info.Kind == KindOther {
		return nil, fmt.Errorf("%s is neither an ELF executable nor a SELF container", info.Eboot)
	}

	for _, base := range uniquePaths(info.Resources, info.Root) {
		for _, name := range []string{"sce_module", "sce_modules", "prx"} {
			candidate := filepath.Join(base, name)
			if directoryExists(candidate) {
				info.ModuleDirs = append(info.ModuleDirs, candidate)
			}
		}
	}
	info.ModuleDirs = uniqueStrings(info.ModuleDirs)
	info.ExtraModuleDirs = findExtraModuleDirs(info.Resources)

	for _, base := range uniquePaths(info.Resources, info.Root) {
		if info.ParamJSON == "" {
			candidate := filepath.Join(base, "sce_sys", "param.json")
			if fileExists(candidate) {
				info.ParamJSON = candidate
			}
		}
		if info.ParamSFO == "" {
			candidate := filepath.Join(base, "sce_sys", "param.sfo")
			if fileExists(candidate) {
				info.ParamSFO = candidate
			}
		}
		if info.Icon == "" {
			candidate := filepath.Join(base, "sce_sys", "icon0.png")
			if fileExists(candidate) {
				info.Icon = candidate
			}
		}
	}

	if info.ParamJSON != "" {
		if err := applyParamJSON(info, info.ParamJSON); err != nil {
			info.Warnings = append(info.Warnings, fmt.Sprintf("%s: %v", info.ParamJSON, err))
		}
	}
	if info.TitleID == "" && info.ParamSFO != "" {
		if err := applyParamSFO(info, info.ParamSFO); err != nil {
			info.Warnings = append(info.Warnings, fmt.Sprintf("%s: %v", info.ParamSFO, err))
		}
	}
	if info.TitleID == "" {
		info.Warnings = append(info.Warnings, "no titleId found in sce_sys/param.json or param.sfo")
	}
	if info.TitleName == "" {
		info.TitleName = strings.TrimSuffix(filepath.Base(info.Eboot), filepath.Ext(info.Eboot))
	}
	if info.ContentVersion == "" && info.ParamJSON == "" && info.ParamSFO == "" {
		info.Warnings = append(info.Warnings, "no sce_sys metadata; the converted title may fail to start")
	}
	return info, nil
}

func findEboot(root string) (string, string) {
	type candidate struct {
		path      string
		resources string
		priority  int
	}
	var candidates []candidate
	add := func(path, resources string, priority int) {
		if fileExists(path) {
			candidates = append(candidates, candidate{path: path, resources: resources, priority: priority})
		}
	}
	add(filepath.Join(root, "eboot.bin"), root, 0)
	add(filepath.Join(root, "eboot.elf"), root, 1)
	add(filepath.Join(root, "app0", "eboot.bin"), filepath.Join(root, "app0"), 2)
	add(filepath.Join(root, "app0", "eboot.elf"), filepath.Join(root, "app0"), 3)
	if filepath.Base(root) == "app0" {
		add(filepath.Join(root, "eboot.bin"), root, 4)
		add(filepath.Join(root, "eboot.elf"), root, 5)
	}
	sort.SliceStable(candidates, func(i, j int) bool { return candidates[i].priority < candidates[j].priority })
	if len(candidates) == 0 {
		// Last resort: a single *.bin beside the root that is not a module.
		entries, err := os.ReadDir(root)
		if err == nil {
			for _, entry := range entries {
				if entry.IsDir() {
					continue
				}
				name := strings.ToLower(entry.Name())
				if strings.Contains(name, "eboot") && (strings.HasSuffix(name, ".bin") || strings.HasSuffix(name, ".elf")) {
					return filepath.Join(root, entry.Name()), root
				}
			}
		}
		return "", ""
	}
	return candidates[0].path, candidates[0].resources
}

func sniff(path string) (Kind, int64, error) {
	file, err := os.Open(path)
	if err != nil {
		return KindOther, 0, err
	}
	defer file.Close()
	stat, err := file.Stat()
	if err != nil {
		return KindOther, 0, err
	}
	header := make([]byte, 4)
	if _, err := file.Read(header); err != nil {
		return KindOther, stat.Size(), err
	}
	switch {
	case bytes.Equal(header, []byte{0x7f, 'E', 'L', 'F'}):
		return KindELF, stat.Size(), nil
	case binary.LittleEndian.Uint32(header) == 0x1D3D154F || binary.LittleEndian.Uint32(header) == 0xEEF51454:
		return KindSELF, stat.Size(), nil
	default:
		return KindOther, stat.Size(), nil
	}
}

type paramJSON struct {
	TitleID             string                     `json:"titleId"`
	ContentVersion      string                     `json:"contentVersion"`
	DefaultLanguage     string                     `json:"defaultLanguage"`
	LocalizedParameters map[string]json.RawMessage `json:"localizedParameters"`
}

func applyParamJSON(info *Info, path string) error {
	body, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var param paramJSON
	if err := json.Unmarshal(body, &param); err != nil {
		return err
	}
	if param.TitleID != "" {
		info.TitleID = param.TitleID
	}
	if param.ContentVersion != "" {
		info.ContentVersion = param.ContentVersion
	}
	language := param.DefaultLanguage
	if language == "" {
		if raw, ok := param.LocalizedParameters["defaultLanguage"]; ok {
			_ = json.Unmarshal(raw, &language)
		}
	}
	info.Language = language
	if name := localizedName(param.LocalizedParameters, language); name != "" {
		info.TitleName = name
	}
	return nil
}

func localizedName(parameters map[string]json.RawMessage, language string) string {
	if parameters == nil {
		return ""
	}
	try := func(key string) string {
		raw, ok := parameters[key]
		if !ok {
			return ""
		}
		var entry struct {
			TitleName string `json:"titleName"`
		}
		if err := json.Unmarshal(raw, &entry); err != nil {
			return ""
		}
		return strings.TrimSpace(entry.TitleName)
	}
	if language != "" {
		if name := try(language); name != "" {
			return name
		}
	}
	if name := try("en-US"); name != "" {
		return name
	}
	keys := make([]string, 0, len(parameters))
	for key := range parameters {
		if key == "defaultLanguage" {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if name := try(key); name != "" {
			return name
		}
	}
	return ""
}

func applyParamSFO(info *Info, path string) error {
	body, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	values, err := ParseSFO(body)
	if err != nil {
		return err
	}
	if value, ok := values["TITLE_ID"].(string); ok && value != "" {
		info.TitleID = value
	}
	if value, ok := values["TITLE"].(string); ok && value != "" {
		info.TitleName = value
	}
	if value, ok := values["VERSION"].(string); ok && value != "" {
		info.ContentVersion = value
	}
	return nil
}

// ParseSFO reads the keys of a Sony param.sfo file.
func ParseSFO(data []byte) (map[string]any, error) {
	if len(data) < 20 || !bytes.Equal(data[:4], []byte{0x00, 'P', 'S', 'F'}) {
		return nil, errors.New("not a param.sfo file")
	}
	version := binary.LittleEndian.Uint32(data[4:])
	if version != 0x101 {
		return nil, fmt.Errorf("unsupported param.sfo version %#x", version)
	}
	keyTable := binary.LittleEndian.Uint32(data[8:])
	dataTable := binary.LittleEndian.Uint32(data[12:])
	count := binary.LittleEndian.Uint32(data[16:])
	if int(20+count*16) > len(data) {
		return nil, errors.New("param.sfo index is truncated")
	}
	result := map[string]any{}
	for index := uint32(0); index < count; index++ {
		entry := 20 + int(index)*16
		keyOffset := binary.LittleEndian.Uint16(data[entry:])
		format := binary.LittleEndian.Uint16(data[entry+2:])
		length := binary.LittleEndian.Uint32(data[entry+4:])
		valueOffset := binary.LittleEndian.Uint32(data[entry+12:])
		keyStart := int(keyTable) + int(keyOffset)
		if keyStart >= len(data) {
			return nil, errors.New("param.sfo key escapes the file")
		}
		keyEnd := keyStart
		for keyEnd < len(data) && data[keyEnd] != 0 {
			keyEnd++
		}
		key := string(data[keyStart:keyEnd])
		valueStart := int(dataTable) + int(valueOffset)
		switch format & 0x0F {
		case 0x02, 0x04:
			if valueStart < 0 || int(length) < 0 || valueStart+int(length) > len(data) {
				return nil, fmt.Errorf("param.sfo value of %q escapes the file", key)
			}
			value := data[valueStart : valueStart+int(length)]
			if zero := bytes.IndexByte(value, 0); zero >= 0 {
				value = value[:zero]
			}
			result[key] = strings.TrimSpace(string(value))
		case 0x00:
			if length == 4 {
				result[key] = binary.LittleEndian.Uint32(data[valueStart:])
			}
		}
	}
	return result, nil
}

func findExtraModuleDirs(resources string) []string {
	var result []string
	rootDepth := strings.Count(filepath.Clean(resources), string(os.PathSeparator))
	_ = filepath.WalkDir(resources, func(current string, entry os.DirEntry, err error) error {
		if err != nil || !entry.IsDir() {
			return nil
		}
		if depth := strings.Count(filepath.Clean(current), string(os.PathSeparator)) - rootDepth; depth > 4 {
			return filepath.SkipDir
		}
		name := strings.ToLower(entry.Name())
		if name == "sce_module" || name == "sce_modules" || name == "prx" || name == "fakelib" || strings.HasPrefix(name, ".") {
			if current != resources {
				return filepath.SkipDir
			}
			return nil
		}
		if name != "modules" && name != "plugins" {
			return nil
		}
		entries, err := os.ReadDir(current)
		if err != nil {
			return nil
		}
		for _, child := range entries {
			if !child.IsDir() && strings.HasSuffix(strings.ToLower(child.Name()), ".prx") {
				relative, err := filepath.Rel(resources, current)
				if err == nil && relative != "." {
					result = append(result, filepath.ToSlash(relative))
				}
				break
			}
		}
		return nil
	})
	sort.Strings(result)
	return uniqueStrings(result)
}

func fileExists(path string) bool {
	stat, err := os.Stat(path)
	return err == nil && !stat.IsDir()
}

func directoryExists(path string) bool {
	stat, err := os.Stat(path)
	return err == nil && stat.IsDir()
}

func uniquePaths(values ...string) []string {
	seen := map[string]bool{}
	var result []string
	for _, value := range values {
		if value == "" {
			continue
		}
		cleaned := filepath.Clean(value)
		if !seen[cleaned] {
			seen[cleaned] = true
			result = append(result, cleaned)
		}
	}
	return result
}

func uniqueStrings(values []string) []string {
	seen := map[string]bool{}
	var result []string
	for _, value := range values {
		if !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	return result
}
