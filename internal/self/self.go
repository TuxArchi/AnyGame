// SPDX-License-Identifier: GPL-2.0-only

// Package self unwraps decrypted SELF and fSELF containers into clean ELF
// executables. It is a Go port of AnyPS5's tools/unwrap_self.py.
package self

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
)

const (
	selfMagic       = 0x1D3D154F
	selfMagicKernel = 0xEEF51454
	recordOffset    = 0x40
	recordSize      = 0x20
	recordHasSeg    = 0x800
	recordSegShift  = 20
	firstRegion     = 0x400

	maxAlignmentPadding = 64 << 20
	maxOutputFactor     = 4
	minOutputLimit      = 256 << 20

	ptNull        = 0
	ptLoad        = 1
	ptDynamic     = 2
	ptInterp      = 3
	ptNote        = 4
	ptPHDR        = 6
	ptTLS         = 7
	entropyLimit  = 7.5
	entropySample = 0x10000
	entropyLarge  = 0x1000
)

// ErrNotSelf reports that the input is not a SELF container.
var ErrNotSelf = errors.New("not a SELF container")

// Result describes a successful unwrap.
type Result struct {
	InputBytes  int64
	OutputBytes int64
	InnerOffset int64
	Headers     []HeaderInfo
	Records     int
}

// HeaderInfo is one program header of the unwrapped ELF.
type HeaderInfo struct {
	Index  int
	Type   uint32
	Flags  uint32
	Vaddr  uint64
	Size   uint64
	Offset uint64
}

type phdr struct {
	index     int
	kind      uint32
	flags     uint32
	offset    int64
	vaddr     int64
	size      int64
	memory    int64
	align     int64
	source    int64
	host      *phdr
	container *phdr
	newOffset int64
}

type record struct {
	flags   uint64
	segment uint64
	size    uint64
}

type region struct {
	old   int64
	size  int64
	vaddr int64
	align int64
	new   int64
}

// UnwrapFile unwraps input into output, atomically replacing output.
func UnwrapFile(input, output string, verbose io.Writer) (Result, error) {
	data, err := os.ReadFile(input)
	if err != nil {
		return Result{}, err
	}
	result, err := Unwrap(data, verbose)
	if err != nil {
		return Result{}, err
	}
	inputAbsolute, err := filepath.Abs(input)
	if err != nil {
		return Result{}, err
	}
	outputAbsolute, err := filepath.Abs(output)
	if err != nil {
		return Result{}, err
	}
	if inputAbsolute == outputAbsolute {
		return Result{}, errors.New("output aliases the input container")
	}
	if inputInfo, err := os.Stat(inputAbsolute); err == nil {
		if outputInfo, err := os.Stat(outputAbsolute); err == nil && os.SameFile(inputInfo, outputInfo) {
			return Result{}, errors.New("output aliases the input container")
		}
	}
	if err := os.MkdirAll(filepath.Dir(outputAbsolute), 0o755); err != nil {
		return Result{}, err
	}
	temporary, err := os.CreateTemp(filepath.Dir(outputAbsolute), ".unwrap-self-*")
	if err != nil {
		return Result{}, err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if _, err := temporary.Write(result.Output); err != nil {
		temporary.Close()
		return Result{}, err
	}
	if err := temporary.Chmod(0o755); err != nil {
		temporary.Close()
		return Result{}, err
	}
	if err := temporary.Close(); err != nil {
		return Result{}, err
	}
	if err := os.Rename(temporaryPath, outputAbsolute); err != nil {
		return Result{}, err
	}
	return result.Info, nil
}

// Output couples the unwrapped bytes with a summary.
type Output struct {
	Output []byte
	Info   Result
}

// Unwrap converts a SELF container in memory.
func Unwrap(data []byte, verbose io.Writer) (Output, error) {
	if len(data) < firstRegion {
		return Output{}, errors.New("file is smaller than the SELF header area")
	}
	magic := binary.LittleEndian.Uint32(data[0:4])
	if magic != selfMagic && magic != selfMagicKernel {
		return Output{}, fmt.Errorf("%w (magic %#x)", ErrNotSelf, magic)
	}
	records, err := readRecords(data)
	if err != nil {
		return Output{}, err
	}
	inner, elfHeader, headers, err := parseELF(data)
	if err != nil {
		return Output{}, err
	}
	var phdrSegment *phdr
	for _, header := range headers {
		if header.kind == ptPHDR {
			if phdrSegment != nil {
				return Output{}, errors.New("more than one PT_PHDR segment")
			}
			phdrSegment = header
		}
	}
	if phdrSegment != nil {
		tableSize := int64(len(headers) * 56)
		if phdrSegment.size < tableSize || phdrSegment.memory < tableSize {
			return Output{}, errors.New("PT_PHDR does not cover the program header table")
		}
		for _, header := range headers {
			if header.kind == ptLoad && header.index < phdrSegment.index {
				return Output{}, errors.New("PT_PHDR must precede the load segments")
			}
		}
	}
	zeroFilled, err := placeSegments(data, headers, records)
	if err != nil {
		return Output{}, err
	}
	if phdrSegment != nil {
		if phdrSegment.host == nil {
			return Output{}, errors.New("PT_PHDR is not in a file-backed load segment")
		}
		table := inner + int64(binary.LittleEndian.Uint64(elfHeader[0x20:0x28]))
		tableSize := int64(len(headers) * 56)
		if table < 0 || table+tableSize > int64(len(data)) || phdrSegment.source < 0 ||
			phdrSegment.source+tableSize > int64(len(data)) {
			return Output{}, errors.New("PT_PHDR does not map the program header table")
		}
		if !bytes.Equal(data[phdrSegment.source:phdrSegment.source+tableSize], data[table:table+tableSize]) {
			return Output{}, errors.New("PT_PHDR does not map the program header table")
		}
	}
	if err := checkDecrypted(data, headers); err != nil {
		return Output{}, err
	}

	var regions []*region
	loadRegions := map[int]*region{}
	for _, header := range headers {
		if header.kind == ptLoad && header.size > 0 {
			item := &region{old: header.source, size: header.size, vaddr: header.vaddr, align: header.align}
			regions = append(regions, item)
			loadRegions[header.index] = item
		}
	}
	regionFor := func(offset, size int64) *region {
		for _, item := range regions {
			if item.old <= offset && offset+size <= item.old+item.size {
				return item
			}
		}
		return nil
	}
	for _, header := range headers {
		if header.kind == ptLoad || header.size == 0 || header.container != nil {
			continue
		}
		if regionFor(header.source, header.size) == nil {
			regions = append(regions, &region{old: header.source, size: header.size, vaddr: header.vaddr, align: header.align})
		}
	}
	sortRegions(regions)

	outputRegion := func(header *phdr) *region {
		host := header
		if header.kind != ptLoad {
			host = header.host
		}
		if host != nil {
			if item, ok := loadRegions[host.index]; ok {
				return item
			}
		}
		return regionFor(header.source, header.size)
	}

	for _, header := range headers {
		if header.size == 0 && (header.kind == ptLoad || header.source == 0) {
			continue
		}
		item := outputRegion(header)
		alignment := header.align
		if item == nil || alignment <= 1 || alignment&(alignment-1) != 0 {
			continue
		}
		current := item.align
		if current == 0 || current&(current-1) != 0 {
			current = 1
		}
		address := header.vaddr - (header.source - item.old)
		step := current
		if alignment < step {
			step = alignment
		}
		if modPositive(address-item.vaddr, step) != 0 {
			return Output{}, fmt.Errorf("segment %d has incompatible alignment within its output region", header.index)
		}
		if alignment > current {
			item.align = alignment
			item.vaddr = address
		}
	}

	programOffset := int64(binary.LittleEndian.Uint64(elfHeader[0x20:0x28]))
	cursor := programOffset + int64(len(headers)*56)
	if cursor < firstRegion {
		cursor = firstRegion
	}
	var padding int64
	for _, item := range regions {
		alignment := item.align
		if alignment <= 0 || alignment&(alignment-1) != 0 {
			alignment = 1
		}
		placed := cursor + modPositive(item.vaddr-cursor, alignment)
		padding += placed - cursor
		item.new = placed
		cursor = placed + item.size
	}
	emptyEnd := cursor
	for _, header := range headers {
		if header.kind == ptLoad && header.size == 0 {
			align := header.align
			if align < 1 {
				align = 1
			}
			if value := header.vaddr % align; value > emptyEnd {
				emptyEnd = value
			}
		}
	}
	padding += emptyEnd - cursor
	if padding > maxAlignmentPadding {
		return Output{}, fmt.Errorf("segment alignment requires more than %d bytes of output padding", maxAlignmentPadding)
	}
	cursor = emptyEnd
	outputLimit := int64(maxOutputFactor * len(data))
	if outputLimit < minOutputLimit {
		outputLimit = minOutputLimit
	}
	if cursor > outputLimit || cursor > math.MaxInt32*8 {
		return Output{}, fmt.Errorf("the load segments need %d bytes of output, more than the %d allowed for a %d byte container", cursor, outputLimit, len(data))
	}
	if cursor < 0 {
		return Output{}, errors.New("negative output size")
	}
	result := make([]byte, cursor)

	for _, item := range regions {
		start := item.old
		end := start + item.size
		target := item.new
		if target < 0 || target+item.size > int64(len(result)) {
			return Output{}, errors.New("output region escapes the output")
		}
		destination := result[target : target+item.size]
		var filePart []byte
		if start < int64(len(data)) {
			fileEnd := end
			if fileEnd > int64(len(data)) {
				fileEnd = int64(len(data))
			}
			if fileEnd > start {
				filePart = data[start:fileEnd]
			}
		}
		copy(destination, filePart)
		if int64(len(filePart)) < int64(len(destination)) {
			tailStart := start - int64(len(data))
			if tailStart < 0 {
				tailStart = 0
			}
			tailEnd := end - int64(len(data))
			if tailEnd > int64(len(zeroFilled)) {
				tailEnd = int64(len(zeroFilled))
			}
			if tailStart < tailEnd {
				copy(destination[len(filePart):], zeroFilled[tailStart:tailEnd])
			}
		}
	}

	for _, header := range headers {
		if header.size == 0 && (header.kind == ptLoad || header.source == 0) {
			if header.kind == ptLoad {
				align := header.align
				if align < 1 {
					align = 1
				}
				header.newOffset = header.vaddr % align
			} else {
				header.newOffset = 0
			}
			continue
		}
		item := outputRegion(header)
		if item == nil {
			header.newOffset = header.source
		} else {
			header.newOffset = item.new + (header.source - item.old)
		}
		if header.newOffset < 0 || header.newOffset+header.size > int64(len(result)) {
			return Output{}, fmt.Errorf("segment %d escaped the output", header.index)
		}
	}

	if phdrSegment != nil {
		programOffset = phdrSegment.newOffset
	}
	if programOffset < 0 || programOffset+int64(len(headers)*56) > int64(len(result)) {
		return Output{}, errors.New("program header table escaped the output")
	}
	copy(result[0:0x40], elfHeader)
	putU64(result, 0x28, 0)
	putU16(result, 0x3C, 0)
	putU16(result, 0x3E, 0)
	for _, item := range headers {
		base := programOffset + int64(item.index*56)
		putU32(result, base+0, item.kind)
		putU32(result, base+4, item.flags)
		putU64(result, base+8, uint64(item.newOffset))
		putU64(result, base+16, uint64(item.vaddr))
		putU64(result, base+24, uint64(item.vaddr))
		putU64(result, base+32, uint64(item.size))
		putU64(result, base+40, uint64(item.memory))
		putU64(result, base+48, uint64(item.align))
	}

	info := Result{
		InputBytes:  int64(len(data)),
		OutputBytes: int64(len(result)),
		InnerOffset: inner,
		Records:     len(records),
	}
	for _, item := range headers {
		info.Headers = append(info.Headers, HeaderInfo{
			Index: item.index, Type: item.kind, Flags: item.flags,
			Vaddr: uint64(item.vaddr), Size: uint64(item.size), Offset: uint64(item.newOffset),
		})
	}
	if verbose != nil {
		fmt.Fprintf(verbose, "%d bytes, inner ELF at %#x, %d program headers, %d SELF records\n",
			len(data), inner, len(headers), len(records))
		for _, item := range headers {
			moved := ""
			if item.newOffset != item.offset {
				moved = fmt.Sprintf(", was %#x", item.offset)
			}
			fmt.Fprintf(verbose, "  [%2d] %-18s vaddr=%#011x filesz=%#010x offset=%#010x%s\n",
				item.index, headerName(item.kind), uint64(item.vaddr), uint64(item.size), uint64(item.newOffset), moved)
		}
	}
	return Output{Output: result, Info: info}, nil
}

func readRecords(data []byte) ([]record, error) {
	var records []record
	for offset := recordOffset; offset+recordSize <= firstRegion; offset += recordSize {
		flags := binary.LittleEndian.Uint64(data[offset:])
		segment := binary.LittleEndian.Uint64(data[offset+8:])
		size := binary.LittleEndian.Uint64(data[offset+16:])
		sizeAgain := binary.LittleEndian.Uint64(data[offset+24:])
		if size == 0 || size != sizeAgain {
			break
		}
		if segment >= uint64(len(data)) || segment < 0x350 || segment%0x10 != 0 {
			break
		}
		if size > uint64(len(data)) {
			break
		}
		records = append(records, record{flags: flags, segment: segment, size: size})
	}
	return records, nil
}

func parseELF(data []byte) (int64, []byte, []*phdr, error) {
	inner := bytes.Index(data, []byte("\x7fELF"))
	if inner < 0 {
		return 0, nil, nil, errors.New("no inner ELF header")
	}
	if inner+0x40 > len(data) {
		return 0, nil, nil, errors.New("inner ELF header is truncated")
	}
	header := append([]byte(nil), data[inner:inner+0x40]...)
	if header[4] != 2 || header[5] != 1 {
		return 0, nil, nil, errors.New("inner ELF is not 64-bit little-endian")
	}
	version := binary.LittleEndian.Uint32(header[0x14:])
	if header[6] != 1 || version != 1 {
		return 0, nil, nil, fmt.Errorf("unsupported inner ELF version (identification %d, object %d)", header[6], version)
	}
	programOffset := int64(binary.LittleEndian.Uint64(header[0x20:]))
	machine := binary.LittleEndian.Uint16(header[0x12:])
	if machine != 62 {
		return 0, nil, nil, fmt.Errorf("unsupported inner ELF machine %d (expected 62)", machine)
	}
	headerSize := binary.LittleEndian.Uint16(header[0x34:])
	if headerSize != 64 {
		return 0, nil, nil, fmt.Errorf("invalid inner ELF header size %d (expected 64)", headerSize)
	}
	entrySize := binary.LittleEndian.Uint16(header[0x36:])
	count := int(binary.LittleEndian.Uint16(header[0x38:]))
	if entrySize != 56 || count == 0 {
		return 0, nil, nil, fmt.Errorf("program header table is %d entries of %d bytes", count, entrySize)
	}
	if programOffset < 0x40 {
		return 0, nil, nil, errors.New("ELF program header table overlaps the ELF header")
	}
	if int64(inner)+programOffset+int64(count*56) > int64(len(data)) {
		return 0, nil, nil, errors.New("ELF program header table does not fit the file")
	}
	headers := make([]*phdr, 0, count)
	for index := 0; index < count; index++ {
		offset := int64(inner) + programOffset + int64(index*56)
		kind := binary.LittleEndian.Uint32(data[offset:])
		flags := binary.LittleEndian.Uint32(data[offset+4:])
		fileOffset := binary.LittleEndian.Uint64(data[offset+8:])
		vaddr := binary.LittleEndian.Uint64(data[offset+16:])
		fileSize := binary.LittleEndian.Uint64(data[offset+32:])
		memorySize := binary.LittleEndian.Uint64(data[offset+40:])
		align := binary.LittleEndian.Uint64(data[offset+48:])
		if fileOffset > math.MaxInt64 || vaddr > math.MaxInt64 || fileSize > math.MaxInt64 ||
			memorySize > math.MaxInt64 || align > math.MaxInt64 {
			return 0, nil, nil, fmt.Errorf("segment %d exceeds the supported range", index)
		}
		if kind != ptNull && align != 0 && align&(align-1) != 0 {
			return 0, nil, nil, fmt.Errorf("segment %d alignment is not a power of two", index)
		}
		if kind == ptLoad || kind == ptTLS {
			if fileSize > memorySize {
				return 0, nil, nil, fmt.Errorf("segment %d file size exceeds memory size", index)
			}
		}
		if kind == ptTLS {
			if align > 8192 {
				return 0, nil, nil, fmt.Errorf("segment %d TLS alignment exceeds 8192", index)
			}
			if memorySize > 0x7FFF0000 {
				return 0, nil, nil, fmt.Errorf("segment %d TLS memory size exceeds 0x7fff0000", index)
			}
		}
		if (kind == ptLoad || kind == ptTLS) && memorySize > math.MaxUint64-vaddr {
			return 0, nil, nil, fmt.Errorf("segment %d memory range exceeds ELF64", index)
		}
		headers = append(headers, &phdr{
			index: index, kind: kind, flags: flags, offset: int64(fileOffset), vaddr: int64(vaddr),
			size: int64(fileSize), memory: int64(memorySize), align: int64(align), source: int64(fileOffset),
		})
	}
	return int64(inner), header, headers, nil
}

func placeSegments(data []byte, headers []*phdr, records []record) ([]byte, error) {
	var zeroFilled []byte
	used := map[int]bool{}
	placed := map[int]bool{}
	for _, header := range headers {
		if header.kind == ptNull {
			header.flags = 0
			header.offset = 0
			header.vaddr = 0
			header.size = 0
			header.memory = 0
			header.align = 0
			header.source = 0
		}
	}
	var loads []*phdr
	for _, header := range headers {
		if header.kind == ptLoad {
			loads = append(loads, header)
		}
	}
	for _, item := range records {
		target := int(item.flags >> recordSegShift)
		if item.flags&recordHasSeg != 0 && target < len(headers) && !placed[target] && uint64(headers[target].size) == item.size {
			headers[target].source = int64(item.segment)
			placed[target] = true
		}
	}
	var recorded []*phdr
	for _, header := range headers {
		if placed[header.index] && header.vaddr == 0 && header.kind != ptLoad {
			recorded = append(recorded, header)
		}
	}
	for _, header := range headers {
		if header.kind != ptInterp || header.vaddr != 0 || header.size == 0 || placed[header.index] {
			continue
		}
		var best *phdr
		for _, load := range loads {
			if load.offset <= header.offset && header.offset+header.size <= load.offset+load.size {
				if best == nil || load.size < best.size {
					best = load
				}
			}
		}
		if best != nil {
			header.host = best
			placed[header.index] = true
		}
	}
	for _, header := range headers {
		if header.kind != ptDynamic || header.vaddr != 0 || header.size == 0 || placed[header.index] {
			continue
		}
		var best *phdr
		for _, other := range recorded {
			if other.offset <= header.offset && header.offset+header.size <= other.offset+other.size {
				if best == nil || other.size < best.size {
					best = other
				}
			}
		}
		if best != nil {
			header.source = best.source + header.offset - best.offset
			header.container = best
			placed[header.index] = true
		}
	}
	if len(recorded) > 0 {
		last := recorded[0]
		for _, header := range recorded {
			if header.offset > last.offset {
				last = header
			}
		}
		cursor := last.source + last.size
		var trailing []*phdr
		for _, header := range headers {
			if !placed[header.index] && header.vaddr == 0 && header.kind != ptLoad &&
				header.size > 0 && header.offset > last.offset {
				trailing = append(trailing, header)
			}
		}
		sortHeadersByOffset(trailing)
		for _, header := range trailing {
			placed[header.index] = true
			if header.kind == ptNote {
				header.source = 0
				header.size = 0
				continue
			}
			header.source = cursor
			cursor += header.size
			if cursor > int64(len(data)) {
				header.source = 0
				header.size = 0
			}
		}
	}
	for _, header := range loads {
		if header.size == 0 || placed[header.index] {
			continue
		}
		for index, item := range records {
			if item.flags&recordHasSeg != 0 || used[index] {
				continue
			}
			if int64(item.size) == header.size {
				used[index] = true
				header.source = int64(item.segment)
				break
			}
		}
	}
	for _, header := range loads {
		if header.size > 0 && (header.source < 0 || header.source > int64(len(data)) ||
			header.size > int64(len(data))-header.source) {
			return nil, fmt.Errorf("segment %d does not fit the file", header.index)
		}
	}

	loadFor := func(vaddr, size int64) *phdr {
		var found *phdr
		for _, header := range loads {
			end := header.vaddr + header.memory
			if header.memory > 0 && header.vaddr <= vaddr && (vaddr < end || (size == 0 && vaddr == end)) {
				found = header
			}
		}
		return found
	}

	var tail *[2]int64
	for _, header := range headers {
		if header.kind == ptLoad {
			continue
		}
		if header.host != nil {
			host := header.host
			header.source = host.source + header.offset - host.offset
		}
		if placed[header.index] {
			if host := loadFor(header.vaddr, header.size); host != nil {
				offset := header.vaddr - host.vaddr
				if host.size > 0 && offset <= host.size && header.size <= host.size-offset &&
					header.source == host.source+offset {
					header.host = host
				}
			}
			if header.size > 0 && (tail == nil || header.offset > tail[0]) {
				tail = &[2]int64{header.offset, header.source - header.offset}
			}
			continue
		}
		if header.vaddr != 0 || header.kind == ptPHDR || header.kind == ptDynamic || header.kind == ptTLS {
			host := loadFor(header.vaddr, header.size)
			if host == nil {
				if header.size == 0 {
					header.source = 0
					continue
				}
				return nil, fmt.Errorf("segment %d vaddr %#x is in no PT_LOAD", header.index, header.vaddr)
			}
			offset := header.vaddr - host.vaddr
			if header.size > 0 && (offset > host.size || header.size > host.size-offset) {
				if header.kind != ptTLS || header.size > host.memory-offset {
					return nil, fmt.Errorf("segment %d does not fit the file-backed range of PT_LOAD %d", header.index, host.index)
				}
				limit := int64(maxOutputFactor * len(data))
				if limit < minOutputLimit {
					limit = minOutputLimit
				}
				if int64(len(zeroFilled))+header.size > limit {
					return nil, fmt.Errorf("segment %d needs %d bytes of output, more than the %d allowed for a %d byte container",
						header.index, header.size, limit, len(data))
				}
				var backed []byte
				if offset < host.size {
					backed = data[host.source+offset : host.source+host.size]
				}
				header.source = int64(len(data)) + int64(len(zeroFilled))
				padding := make([]byte, header.size-int64(len(backed)))
				zeroFilled = append(zeroFilled, backed...)
				zeroFilled = append(zeroFilled, padding...)
				continue
			}
			if host.size == 0 || offset > host.size {
				header.source = 0
				continue
			}
			header.host = host
			header.source = host.source + offset
			if tail == nil || header.offset > tail[0] {
				tail = &[2]int64{header.offset, header.source - header.offset}
			}
		} else if header.size > 0 {
			shift := int64(0)
			if tail != nil {
				shift = tail[1]
			}
			header.source = header.offset + shift
			if header.source > int64(len(data)) {
				header.source = int64(len(data))
			}
			keep := int64(len(data)) - header.source
			if keep < 0 {
				keep = 0
			}
			if header.size > keep {
				header.size = keep
			}
			if header.memory > keep {
				header.memory = keep
			}
		}
	}
	end := int64(len(data)) + int64(len(zeroFilled))
	for _, header := range headers {
		if header.kind == ptLoad && header.size == 0 {
			continue
		}
		if header.source < 0 || header.source > end || header.size > end-header.source {
			return nil, fmt.Errorf("segment %d does not fit the file", header.index)
		}
	}
	return zeroFilled, nil
}

func checkDecrypted(data []byte, headers []*phdr) error {
	var encrypted []string
	var encryptedLarge, total int64
	loadCount := 0
	for _, header := range headers {
		if header.kind != ptLoad || header.size == 0 {
			continue
		}
		loadCount++
		total += header.size
		value := segmentEntropy(data, header)
		if value >= entropyLimit {
			encrypted = append(encrypted, fmt.Sprintf("segment %d at %#x (%.2f bits per byte)", header.index, header.source, value))
			if header.size >= entropyLarge {
				encryptedLarge += header.size
			}
		}
	}
	every := len(encrypted) == loadCount && loadCount > 0
	if len(encrypted) > 0 && (every || encryptedLarge*2 > total) {
		if every {
			return fmt.Errorf("every load segment looks encrypted: %s", strings.Join(encrypted, ", "))
		}
		return fmt.Errorf("most load bytes look encrypted: %s", strings.Join(encrypted, ", "))
	}
	return nil
}

func segmentEntropy(data []byte, header *phdr) float64 {
	start, size := header.source, header.size
	if size <= entropySample {
		return entropy(data[start : start+size])
	}
	offsets := []int64{0, (size - entropySample) / 2, size - entropySample}
	minimum := math.Inf(1)
	for _, offset := range offsets {
		value := entropy(data[start+offset : start+offset+entropySample])
		if value < minimum {
			minimum = value
		}
	}
	return minimum
}

func entropy(data []byte) float64 {
	if len(data) == 0 {
		return 0
	}
	var counts [256]int
	for _, value := range data {
		counts[value]++
	}
	length := float64(len(data))
	result := 0.0
	for _, count := range counts {
		if count == 0 {
			continue
		}
		fraction := float64(count) / length
		result -= fraction * math.Log2(fraction)
	}
	return result
}

func modPositive(value, modulus int64) int64 {
	if modulus <= 0 {
		return 0
	}
	result := value % modulus
	if result < 0 {
		result += modulus
	}
	return result
}

func sortRegions(regions []*region) {
	for i := 1; i < len(regions); i++ {
		for j := i; j > 0 && regions[j].old < regions[j-1].old; j-- {
			regions[j], regions[j-1] = regions[j-1], regions[j]
		}
	}
}

func sortHeadersByOffset(headers []*phdr) {
	for i := 1; i < len(headers); i++ {
		for j := i; j > 0 && headers[j].offset < headers[j-1].offset; j-- {
			headers[j], headers[j-1] = headers[j-1], headers[j]
		}
	}
}

func headerName(kind uint32) string {
	switch kind {
	case ptLoad:
		return "PT_LOAD"
	case ptDynamic:
		return "PT_DYNAMIC"
	case ptInterp:
		return "PT_INTERP"
	case ptNote:
		return "PT_NOTE"
	case ptPHDR:
		return "PT_PHDR"
	case ptTLS:
		return "PT_TLS"
	case 0x6474E550:
		return "PT_GNU_EH_FRAME"
	case 0x6474E551:
		return "PT_GNU_STACK"
	case 0x6474E552:
		return "PT_GNU_RELRO"
	case 0x61000000:
		return "PT_SCE_DYNLIBDATA"
	case 0x61000001:
		return "PT_OS_PROCPARAM"
	}
	return fmt.Sprintf("PT_%#x", kind)
}

func putU16(data []byte, offset int64, value uint16) {
	binary.LittleEndian.PutUint16(data[offset:offset+2], value)
}

func putU32(data []byte, offset int64, value uint32) {
	binary.LittleEndian.PutUint32(data[offset:offset+4], value)
}

func putU64(data []byte, offset int64, value uint64) {
	binary.LittleEndian.PutUint64(data[offset:offset+8], value)
}
