// SPDX-License-Identifier: GPL-2.0-only

package self

import (
	"bytes"
	"encoding/binary"
	"testing"
)

// minimalELF builds a position-independent ELF64 with one PT_LOAD whose
// payload sits at file offset 0x1000 and virtual address 0x1000.
func minimalELF(payload []byte) []byte {
	const payloadOffset = 0x1000
	image := make([]byte, payloadOffset+len(payload))
	copy(image[0:4], []byte("\x7fELF"))
	image[4] = 2                                    // ELFCLASS64
	image[5] = 1                                    // ELFDATA2LSB
	image[6] = 1                                    // EV_CURRENT
	binary.LittleEndian.PutUint16(image[0x10:], 3)  // ET_DYN
	binary.LittleEndian.PutUint16(image[0x12:], 62) // EM_X86_64
	binary.LittleEndian.PutUint32(image[0x14:], 1)  // EV_CURRENT
	binary.LittleEndian.PutUint64(image[0x18:], 0x1000)
	binary.LittleEndian.PutUint64(image[0x20:], 0x40)
	binary.LittleEndian.PutUint16(image[0x34:], 0x40)
	binary.LittleEndian.PutUint16(image[0x36:], 56)
	binary.LittleEndian.PutUint16(image[0x38:], 1)
	header := image[0x40:]
	binary.LittleEndian.PutUint32(header[0:], 1) // PT_LOAD
	binary.LittleEndian.PutUint32(header[4:], 5) // R+X
	binary.LittleEndian.PutUint64(header[8:], payloadOffset)
	binary.LittleEndian.PutUint64(header[16:], 0x1000)
	binary.LittleEndian.PutUint64(header[24:], 0x1000)
	binary.LittleEndian.PutUint64(header[32:], uint64(len(payload)))
	binary.LittleEndian.PutUint64(header[40:], uint64(len(payload)))
	binary.LittleEndian.PutUint64(header[48:], 0x1000)
	copy(image[payloadOffset:], payload)
	return image
}

// wrapSELF embeds an ELF in a SELF container with one segment record.
func wrapSELF(elf []byte) []byte {
	const innerOffset = 0x400
	const recordPayload = 0x2000
	image := make([]byte, recordPayload+len(elf)-0x1000)
	binary.LittleEndian.PutUint32(image[0:], selfMagic)
	flags := uint64(recordHasSeg)
	binary.LittleEndian.PutUint64(image[recordOffset:], flags)
	binary.LittleEndian.PutUint64(image[recordOffset+8:], recordPayload)
	binary.LittleEndian.PutUint64(image[recordOffset+16:], uint64(len(elf)-0x1000))
	binary.LittleEndian.PutUint64(image[recordOffset+24:], uint64(len(elf)-0x1000))
	copy(image[innerOffset:], elf)
	copy(image[recordPayload:], elf[0x1000:])
	return image
}

func TestUnwrapMinimalSelf(t *testing.T) {
	payload := make([]byte, 0x200)
	for index := range payload {
		payload[index] = byte(index % 4)
	}
	elf := minimalELF(payload)
	container := wrapSELF(elf)
	var verbose bytes.Buffer
	output, err := Unwrap(container, &verbose)
	if err != nil {
		t.Fatalf("unwrap: %v", err)
	}
	if !bytes.Equal(output.Output[:4], []byte("\x7fELF")) {
		t.Fatalf("output does not start with an ELF header: %x", output.Output[:4])
	}
	if len(output.Output) != 0x1200 {
		t.Fatalf("output size = %#x, want %#x", len(output.Output), 0x1200)
	}
	if !bytes.Equal(output.Output[0x1000:0x1200], payload) {
		t.Fatal("payload was not copied to its virtual address")
	}
	programOffset := binary.LittleEndian.Uint64(output.Output[0x20:])
	entry := programOffset + 0
	offset := binary.LittleEndian.Uint64(output.Output[entry+8:])
	vaddr := binary.LittleEndian.Uint64(output.Output[entry+16:])
	filesz := binary.LittleEndian.Uint64(output.Output[entry+32:])
	if offset != 0x1000 || vaddr != 0x1000 || filesz != 0x200 {
		t.Fatalf("program header = offset %#x vaddr %#x size %#x", offset, vaddr, filesz)
	}
	if !bytes.Contains(verbose.Bytes(), []byte("1 program headers")) {
		t.Fatalf("verbose output = %q", verbose.String())
	}
}

func TestUnwrapRejectsOtherFiles(t *testing.T) {
	if _, err := Unwrap(make([]byte, firstRegion), nil); err == nil {
		t.Fatal("expected a refusal for a file without the SELF magic")
	}
	if _, err := Unwrap(make([]byte, 16), nil); err == nil {
		t.Fatal("expected a refusal for a truncated file")
	}
}

func TestUnwrapRejectsEncryptedSegments(t *testing.T) {
	payload := make([]byte, 0x2000)
	for index := range payload {
		payload[index] = byte((index * 131) % 256)
	}
	elf := minimalELF(payload)
	container := wrapSELF(elf)
	if _, err := Unwrap(container, nil); err == nil {
		t.Fatal("expected high-entropy segments to be refused")
	}
}
