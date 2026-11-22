// SPDX-License-Identifier: GPL-2.0-only

package verify

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

func writeELF(t *testing.T, path string, needed ...string) {
	t.Helper()
	const (
		loadOffset = 0x1000
		loadVaddr  = 0x1000
		strtabOff  = 0x180
	)
	var stringsTable []byte
	stringsTable = append(stringsTable, 0)
	offsets := make([]uint32, len(needed))
	for index, name := range needed {
		offsets[index] = uint32(len(stringsTable))
		stringsTable = append(stringsTable, name...)
		stringsTable = append(stringsTable, 0)
	}
	payload := make([]byte, 0x200)
	for index, offset := range offsets {
		binary.LittleEndian.PutUint64(payload[index*16:], 1)
		binary.LittleEndian.PutUint64(payload[index*16+8:], uint64(offset))
	}
	next := len(needed)
	binary.LittleEndian.PutUint64(payload[next*16:], 5)
	binary.LittleEndian.PutUint64(payload[next*16+8:], loadVaddr+strtabOff)
	binary.LittleEndian.PutUint64(payload[(next+1)*16:], 10)
	binary.LittleEndian.PutUint64(payload[(next+1)*16+8:], uint64(len(stringsTable)))
	binary.LittleEndian.PutUint64(payload[(next+2)*16:], 0)
	copy(payload[strtabOff:], stringsTable)

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
	binary.LittleEndian.PutUint32(load, 1)
	binary.LittleEndian.PutUint32(load[4:], 5)
	binary.LittleEndian.PutUint64(load[8:], loadOffset)
	binary.LittleEndian.PutUint64(load[16:], loadVaddr)
	binary.LittleEndian.PutUint64(load[24:], loadVaddr)
	binary.LittleEndian.PutUint64(load[32:], uint64(len(payload)))
	binary.LittleEndian.PutUint64(load[40:], uint64(len(payload)))
	binary.LittleEndian.PutUint64(load[48:], 0x1000)
	dynamic := image[0x40+56:]
	binary.LittleEndian.PutUint32(dynamic, 2)
	binary.LittleEndian.PutUint32(dynamic[4:], 6)
	binary.LittleEndian.PutUint64(dynamic[8:], loadOffset)
	binary.LittleEndian.PutUint64(dynamic[16:], loadVaddr)
	binary.LittleEndian.PutUint64(dynamic[24:], loadVaddr)
	binary.LittleEndian.PutUint64(dynamic[32:], uint64((next+3)*16))
	binary.LittleEndian.PutUint64(dynamic[40:], uint64((next+3)*16))
	binary.LittleEndian.PutUint64(dynamic[48:], 8)
	copy(image[loadOffset:], payload)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, image, 0o755); err != nil {
		t.Fatal(err)
	}
}

func makePackage(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	write := func(path string, body string, mode os.FileMode) {
		full := filepath.Join(dir, path)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), mode); err != nil {
			t.Fatal(err)
		}
	}
	write("anygame.json", `{"format":"anygame.package/1","target":"linux","executable":"app.elf"}`, 0o644)
	write("run.sh", "#!/bin/sh\n", 0o755)
	write("app0/eboot.bin", "eboot", 0o644)
	write("libs/libfoo.prx", "foo", 0o644)
	write("libs/libbar.prx", "bar", 0o644)
	writeELF(t, filepath.Join(dir, "app.elf"), "libfoo.prx")
	writeELF(t, filepath.Join(dir, "app0", "sce_module", "module.prx.guest.prx"), "libbar.prx")
	return dir
}

func TestVerifyCompletePackage(t *testing.T) {
	report, err := Run(makePackage(t))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !report.OK {
		t.Fatalf("package reported inconsistent: %#v", report.Checks)
	}
	if report.Errors != 0 {
		t.Fatalf("errors = %d", report.Errors)
	}
}

func TestVerifyFindsMissingModuleDependency(t *testing.T) {
	dir := makePackage(t)
	if err := os.Remove(filepath.Join(dir, "libs", "libbar.prx")); err != nil {
		t.Fatal(err)
	}
	report, err := Run(dir)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if report.OK {
		t.Fatal("missing module dependency was not detected")
	}
	found := false
	for _, name := range report.MissingLibs {
		if name == "libbar.prx" {
			found = true
		}
	}
	if !found {
		t.Fatalf("missing libraries = %#v", report.MissingLibs)
	}
}

func TestVerifyRejectsNonPackage(t *testing.T) {
	if _, err := Run(t.TempDir()); err == nil {
		t.Fatal("expected an error without anygame.json")
	}
}
