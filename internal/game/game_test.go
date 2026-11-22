// SPDX-License-Identifier: GPL-2.0-only

package game

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

func buildSFO(values map[string]string) []byte {
	type entry struct {
		key      string
		value    string
		keyStart int
		dataOff  int
	}
	keys := []string{"TITLE", "TITLE_ID", "VERSION"}
	entries := make([]entry, 0, len(keys))
	keyTable := make([]byte, 0)
	dataTable := make([]byte, 0)
	for _, key := range keys {
		value, ok := values[key]
		if !ok {
			continue
		}
		item := entry{key: key, value: value, keyStart: len(keyTable), dataOff: len(dataTable)}
		keyTable = append(keyTable, key...)
		keyTable = append(keyTable, 0)
		dataTable = append(dataTable, value...)
		dataTable = append(dataTable, 0)
		entries = append(entries, item)
	}
	headerSize := 20
	indexSize := len(entries) * 16
	keyBase := headerSize + indexSize
	dataBase := keyBase + len(keyTable)
	if dataBase%4 != 0 {
		dataBase += 4 - dataBase%4
	}
	image := make([]byte, dataBase+len(dataTable))
	copy(image[0:4], []byte{0x00, 'P', 'S', 'F'})
	binary.LittleEndian.PutUint32(image[4:], 0x101)
	binary.LittleEndian.PutUint32(image[8:], uint32(keyBase))
	binary.LittleEndian.PutUint32(image[12:], uint32(dataBase))
	binary.LittleEndian.PutUint32(image[16:], uint32(len(entries)))
	for index, item := range entries {
		offset := headerSize + index*16
		binary.LittleEndian.PutUint16(image[offset:], uint16(item.keyStart))
		binary.LittleEndian.PutUint16(image[offset+2:], 0x0204)
		binary.LittleEndian.PutUint32(image[offset+4:], uint32(len(item.value)+1))
		binary.LittleEndian.PutUint32(image[offset+8:], uint32(len(item.value)+1))
		binary.LittleEndian.PutUint32(image[offset+12:], uint32(item.dataOff))
	}
	copy(image[keyBase:], keyTable)
	copy(image[dataBase:], dataTable)
	return image
}

func TestParseSFO(t *testing.T) {
	values, err := ParseSFO(buildSFO(map[string]string{
		"TITLE": "AnyGame Test", "TITLE_ID": "PPSA00001", "VERSION": "01.000.000",
	}))
	if err != nil {
		t.Fatalf("ParseSFO: %v", err)
	}
	if values["TITLE"] != "AnyGame Test" || values["TITLE_ID"] != "PPSA00001" || values["VERSION"] != "01.000.000" {
		t.Fatalf("unexpected values: %#v", values)
	}
}

func TestDiscoverParamJSON(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "eboot.bin"), []byte{0x7f, 'E', 'L', 'F', 2, 1, 1})
	write(t, filepath.Join(root, "sce_sys", "param.json"),
		[]byte(`{"titleId":"PPSA01234","contentVersion":"01.002.000","defaultLanguage":"es-ES","localizedParameters":{"en-US":{"titleName":"English"},"es-ES":{"titleName":"Español"}}}`))
	write(t, filepath.Join(root, "sce_sys", "icon0.png"), []byte("png"))
	if err := os.MkdirAll(filepath.Join(root, "sce_module"), 0o755); err != nil {
		t.Fatal(err)
	}
	info, err := Discover(root)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if info.TitleID != "PPSA01234" || info.TitleName != "Español" || info.ContentVersion != "01.002.000" {
		t.Fatalf("unexpected metadata: %#v", info)
	}
	if info.Kind != KindELF {
		t.Fatalf("kind = %s", info.Kind)
	}
	if len(info.ModuleDirs) != 1 || filepath.Base(info.ModuleDirs[0]) != "sce_module" {
		t.Fatalf("module dirs = %#v", info.ModuleDirs)
	}
	if info.Icon == "" {
		t.Fatal("icon was not discovered")
	}
}

func TestDiscoverParamSFO(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "eboot.bin"), []byte{0x4f, 0x15, 0x3d, 0x1d})
	write(t, filepath.Join(root, "sce_sys", "param.sfo"), buildSFO(map[string]string{
		"TITLE": "SFO Title", "TITLE_ID": "PPSA99999", "VERSION": "01.000.000",
	}))
	info, err := Discover(filepath.Join(root, "eboot.bin"))
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if info.Kind != KindSELF {
		t.Fatalf("kind = %s", info.Kind)
	}
	if info.TitleName != "SFO Title" || info.TitleID != "PPSA99999" {
		t.Fatalf("unexpected metadata: %#v", info)
	}
}

func TestDiscoverApp0Layout(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "app0", "eboot.bin"), []byte{0x7f, 'E', 'L', 'F'})
	info, err := Discover(root)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if filepath.Base(info.Resources) != "app0" {
		t.Fatalf("resources = %s", info.Resources)
	}
}

func write(t *testing.T, path string, body []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestDiscoverNestedDefaultLanguage(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "eboot.bin"), []byte{0x54, 0x14, 0xf5, 0xee})
	write(t, filepath.Join(root, "sce_sys", "param.json"),
		[]byte(`{"titleId":"PPSA12345","contentVersion":"01.000.003","localizedParameters":{"defaultLanguage":"en-US","en-US":{"titleName":"Demo Title"},"es-ES":{"titleName":"Demo Title ES"}}}`))
	info, err := Discover(root)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if info.TitleID != "PPSA12345" || info.TitleName != "Demo Title" || info.Language != "en-US" {
		t.Fatalf("unexpected metadata: %#v", info)
	}
}

func TestDiscoverExtraModuleDirs(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "eboot.bin"), []byte{0x7f, 'E', 'L', 'F'})
	write(t, filepath.Join(root, "sce_module", "libc.prx"), []byte("module"))
	write(t, filepath.Join(root, "Media", "Modules", "PS5Util.prx"), []byte("module"))
	write(t, filepath.Join(root, "Media", "Plugins", "libfmod.prx"), []byte("module"))
	write(t, filepath.Join(root, "fakelib", "libSceAgc.sprx"), []byte("module"))
	info, err := Discover(root)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	want := []string{"Media/Modules", "Media/Plugins"}
	if len(info.ExtraModuleDirs) != len(want) {
		t.Fatalf("extra modules = %#v, want %#v", info.ExtraModuleDirs, want)
	}
	for index, value := range want {
		if info.ExtraModuleDirs[index] != value {
			t.Fatalf("extra modules = %#v, want %#v", info.ExtraModuleDirs, want)
		}
	}
}
