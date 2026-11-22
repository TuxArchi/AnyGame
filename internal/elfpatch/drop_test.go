// SPDX-License-Identifier: GPL-2.0-only

package elfpatch

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

func buildELF(t *testing.T, needed []string) (string, int) {
	t.Helper()
	const (
		loadOffset = 0x1000
		loadVaddr  = 0x1000
		strtabOff  = 0x180
	)
	var stringTable []byte
	stringTable = append(stringTable, 0)
	offsets := make([]uint32, len(needed))
	for index, name := range needed {
		offsets[index] = uint32(len(stringTable))
		stringTable = append(stringTable, name...)
		stringTable = append(stringTable, 0)
	}
	payload := make([]byte, 0x200)
	for index, offset := range offsets {
		binary.LittleEndian.PutUint64(payload[index*16:], dtNeeded)
		binary.LittleEndian.PutUint64(payload[index*16+8:], uint64(offset))
	}
	next := len(needed)
	binary.LittleEndian.PutUint64(payload[next*16:], dtStrtab)
	binary.LittleEndian.PutUint64(payload[next*16+8:], loadVaddr+strtabOff)
	binary.LittleEndian.PutUint64(payload[(next+1)*16:], dtStrsz)
	binary.LittleEndian.PutUint64(payload[(next+1)*16+8:], uint64(len(stringTable)))
	binary.LittleEndian.PutUint64(payload[(next+2)*16:], dtNull)
	copy(payload[strtabOff:], stringTable)

	image := make([]byte, loadOffset+len(payload))
	copy(image, []byte{0x7f, 'E', 'L', 'F'})
	image[4] = 2
	image[5] = 1
	image[6] = 1
	binary.LittleEndian.PutUint16(image[0x10:], 3)
	binary.LittleEndian.PutUint16(image[0x12:], 62)
	binary.LittleEndian.PutUint32(image[0x14:], 1)
	binary.LittleEndian.PutUint64(image[0x20:], 0x40)
	binary.LittleEndian.PutUint16(image[0x34:], 0x40)
	binary.LittleEndian.PutUint16(image[0x36:], 56)
	binary.LittleEndian.PutUint16(image[0x38:], 2)
	load := image[0x40:]
	binary.LittleEndian.PutUint32(load, ptLoad)
	binary.LittleEndian.PutUint32(load[4:], 5)
	binary.LittleEndian.PutUint64(load[8:], loadOffset)
	binary.LittleEndian.PutUint64(load[16:], loadVaddr)
	binary.LittleEndian.PutUint64(load[24:], loadVaddr)
	binary.LittleEndian.PutUint64(load[32:], uint64(len(payload)))
	binary.LittleEndian.PutUint64(load[40:], uint64(len(payload)))
	binary.LittleEndian.PutUint64(load[48:], 0x1000)
	dynamic := image[0x40+56:]
	binary.LittleEndian.PutUint32(dynamic, ptDynamic)
	binary.LittleEndian.PutUint32(dynamic[4:], 6)
	binary.LittleEndian.PutUint64(dynamic[8:], loadOffset)
	binary.LittleEndian.PutUint64(dynamic[16:], loadVaddr)
	binary.LittleEndian.PutUint64(dynamic[24:], loadVaddr)
	binary.LittleEndian.PutUint64(dynamic[32:], uint64((next+3)*16))
	binary.LittleEndian.PutUint64(dynamic[40:], uint64((next+3)*16))
	binary.LittleEndian.PutUint64(dynamic[48:], 8)
	copy(image[loadOffset:], payload)

	path := filepath.Join(t.TempDir(), "app.elf")
	if err := os.WriteFile(path, image, 0o755); err != nil {
		t.Fatal(err)
	}
	return path, len(needed)
}

func TestDropNeededRetagsOnlyTheRequestedEntry(t *testing.T) {
	path, count := buildELF(t, []string{"UnityNpToolkit2.prx", "libfoo.prx"})
	dropped, err := DropNeeded(path, map[string]bool{"UnityNpToolkit2.prx": true})
	if err != nil {
		t.Fatalf("DropNeeded: %v", err)
	}
	if len(dropped) != 1 || dropped[0] != "UnityNpToolkit2.prx" {
		t.Fatalf("dropped = %#v", dropped)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	const dynamicOffset = 0x1000
	tags := make([]uint64, count)
	for index := 0; index < count; index++ {
		tags[index] = binary.LittleEndian.Uint64(body[dynamicOffset+index*16:])
	}
	if tags[0] != dtSoname || tags[1] != dtNeeded {
		t.Fatalf("tags = %#v, want [DT_SONAME, DT_NEEDED]", tags)
	}
}

func TestDropNeededIgnoresUnknownNames(t *testing.T) {
	path, _ := buildELF(t, []string{"libfoo.prx"})
	dropped, err := DropNeeded(path, map[string]bool{"other.prx": true})
	if err != nil {
		t.Fatalf("DropNeeded: %v", err)
	}
	if len(dropped) != 0 {
		t.Fatalf("dropped = %#v", dropped)
	}
}

func TestDropNeededRejectsNonELF(t *testing.T) {
	path := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(path, []byte("not an elf"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := DropNeeded(path, map[string]bool{"x": true}); err == nil {
		t.Fatal("expected an error")
	}
}

func TestNeededReadsDynamicStrings(t *testing.T) {
	path, _ := buildELF(t, []string{"UnityNpToolkit2.prx", "libfoo.prx"})
	names, err := Needed(path)
	if err != nil {
		t.Fatalf("Needed: %v", err)
	}
	if len(names) != 2 || names[0] != "UnityNpToolkit2.prx" || names[1] != "libfoo.prx" {
		t.Fatalf("names = %#v", names)
	}
}
