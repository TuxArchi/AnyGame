// SPDX-License-Identifier: GPL-2.0-only

// Package elfpatch applies small, well-defined edits to ELF64 executables.
package elfpatch

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const (
	ptLoad    = 1
	ptDynamic = 2

	dtNull           = 0
	dtNeeded         = 1
	dtStrtab         = 5
	dtStrsz          = 10
	dtSoname         = 14
	dynamicEntrySize = 16
)

// ErrNotELF reports that the file is not a 64-bit little-endian ELF.
var ErrNotELF = errors.New("not a 64-bit little-endian ELF")

type segment struct {
	offset uint64
	vaddr  uint64
	filesz uint64
}

// DropNeeded retags the DT_NEEDED entries whose library name is in names as
// DT_SONAME. The dynamic loader ignores DT_SONAME in an executable, so the
// dependency disappears without moving the string table or the dynamic array.
func DropNeeded(path string, names map[string]bool) ([]string, error) {
	if len(names) == 0 {
		return nil, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(data) < 64 || !bytes.Equal(data[:4], []byte{0x7f, 'E', 'L', 'F'}) {
		return nil, ErrNotELF
	}
	if data[4] != 2 || data[5] != 1 {
		return nil, ErrNotELF
	}
	le := binary.LittleEndian
	phoff := le.Uint64(data[0x20:])
	phentsize := le.Uint16(data[0x36:])
	phnum := le.Uint16(data[0x38:])
	if phentsize < 56 || phoff+uint64(phentsize)*uint64(phnum) > uint64(len(data)) {
		return nil, errors.New("invalid program header table")
	}
	var loads []segment
	var dynamicOffset, dynamicSize uint64
	for index := uint16(0); index < phnum; index++ {
		base := phoff + uint64(index)*uint64(phentsize)
		kind := le.Uint32(data[base:])
		switch kind {
		case ptLoad:
			loads = append(loads, segment{
				offset: le.Uint64(data[base+8:]),
				vaddr:  le.Uint64(data[base+16:]),
				filesz: le.Uint64(data[base+32:]),
			})
		case ptDynamic:
			dynamicOffset = le.Uint64(data[base+8:])
			dynamicSize = le.Uint64(data[base+32:])
		}
	}
	if dynamicOffset == 0 || dynamicSize == 0 || dynamicOffset+dynamicSize > uint64(len(data)) {
		return nil, errors.New("no PT_DYNAMIC segment")
	}
	var stringTableAddress uint64
	for offset := dynamicOffset; offset+dynamicEntrySize <= dynamicOffset+dynamicSize; offset += dynamicEntrySize {
		tag := int64(le.Uint64(data[offset:]))
		if tag == dtNull {
			break
		}
		if tag == dtStrtab {
			stringTableAddress = le.Uint64(data[offset+8:])
		}
	}
	if stringTableAddress == 0 {
		return nil, errors.New("no DT_STRTAB")
	}
	stringTableOffset, ok := virtualToOffset(stringTableAddress, loads)
	if !ok || stringTableOffset > uint64(len(data)) {
		return nil, errors.New("DT_STRTAB is outside the loaded image")
	}
	// DT_STRTAB points at the first byte, which is the empty string.
	var dropped []string
	for offset := dynamicOffset; offset+dynamicEntrySize <= dynamicOffset+dynamicSize; offset += dynamicEntrySize {
		tag := int64(le.Uint64(data[offset:]))
		if tag == dtNull {
			break
		}
		value := le.Uint64(data[offset+8:])
		if tag != dtNeeded {
			continue
		}
		name, err := readString(data, stringTableOffset+value)
		if err != nil {
			return nil, err
		}
		if !names[name] {
			continue
		}
		le.PutUint64(data[offset:], dtSoname)
		dropped = append(dropped, name)
	}
	if len(dropped) == 0 {
		return nil, nil
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".elfpatch-*")
	if err != nil {
		return nil, err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if _, err := temporary.Write(data); err != nil {
		temporary.Close()
		return nil, err
	}
	info, err := os.Stat(path)
	if err == nil {
		_ = temporary.Chmod(info.Mode().Perm())
	}
	if err := temporary.Close(); err != nil {
		return nil, err
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return nil, err
	}
	return dropped, nil
}

func virtualToOffset(address uint64, loads []segment) (uint64, bool) {
	for _, load := range loads {
		if address >= load.vaddr && address < load.vaddr+load.filesz {
			return load.offset + (address - load.vaddr), true
		}
	}
	return 0, false
}

func readString(data []byte, offset uint64) (string, error) {
	if offset >= uint64(len(data)) {
		return "", fmt.Errorf("string offset %#x is outside the file", offset)
	}
	end := offset
	for end < uint64(len(data)) && data[end] != 0 {
		end++
	}
	if end == uint64(len(data)) {
		return "", errors.New("unterminated dynamic string")
	}
	return string(data[offset:end]), nil
}

// Needed lists the DT_NEEDED strings of an ELF64 image. It reads the dynamic
// segment through PT_DYNAMIC, so it also works for modules without section
// headers.
func Needed(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(data) < 64 || !bytes.Equal(data[:4], []byte{0x7f, 'E', 'L', 'F'}) || data[4] != 2 || data[5] != 1 {
		return nil, ErrNotELF
	}
	le := binary.LittleEndian
	phoff := le.Uint64(data[0x20:])
	phentsize := le.Uint16(data[0x36:])
	phnum := le.Uint16(data[0x38:])
	if phentsize < 56 || phoff+uint64(phentsize)*uint64(phnum) > uint64(len(data)) {
		return nil, errors.New("invalid program header table")
	}
	var loads []segment
	var dynamicOffset, dynamicSize uint64
	for index := uint16(0); index < phnum; index++ {
		base := phoff + uint64(index)*uint64(phentsize)
		switch le.Uint32(data[base:]) {
		case ptLoad:
			loads = append(loads, segment{
				offset: le.Uint64(data[base+8:]),
				vaddr:  le.Uint64(data[base+16:]),
				filesz: le.Uint64(data[base+32:]),
			})
		case ptDynamic:
			dynamicOffset = le.Uint64(data[base+8:])
			dynamicSize = le.Uint64(data[base+32:])
		}
	}
	if dynamicOffset == 0 || dynamicSize == 0 || dynamicOffset+dynamicSize > uint64(len(data)) {
		return nil, errors.New("no PT_DYNAMIC segment")
	}
	var stringTableAddress uint64
	for offset := dynamicOffset; offset+dynamicEntrySize <= dynamicOffset+dynamicSize; offset += dynamicEntrySize {
		tag := int64(le.Uint64(data[offset:]))
		if tag == dtNull {
			break
		}
		if tag == dtStrtab {
			stringTableAddress = le.Uint64(data[offset+8:])
		}
	}
	if stringTableAddress == 0 {
		return nil, errors.New("no DT_STRTAB")
	}
	stringTableOffset, ok := virtualToOffset(stringTableAddress, loads)
	if !ok || stringTableOffset > uint64(len(data)) {
		return nil, errors.New("DT_STRTAB is outside the loaded image")
	}
	var names []string
	for offset := dynamicOffset; offset+dynamicEntrySize <= dynamicOffset+dynamicSize; offset += dynamicEntrySize {
		tag := int64(le.Uint64(data[offset:]))
		if tag == dtNull {
			break
		}
		if tag != dtNeeded {
			continue
		}
		name, err := readString(data, stringTableOffset+le.Uint64(data[offset+8:]))
		if err != nil {
			return nil, err
		}
		names = append(names, name)
	}
	return names, nil
}
