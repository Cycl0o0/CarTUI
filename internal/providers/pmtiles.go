// SPDX-FileCopyrightText: 2026 Cycl0o0 <contact@cyclooo.fr>
// SPDX-License-Identifier: AGPL-3.0-or-later

package providers

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"

	"github.com/cycl0o0/cartui/internal/pbf"
)

// PMTiles spec v3:
//
//   https://github.com/protomaps/PMTiles/blob/main/spec/v3/spec.md
//
// The archive layout is:
//   - 127-byte header (this file's HeaderV3)
//   - root directory (compressed entries)
//   - JSON metadata (compressed)
//   - leaf directories blob (compressed entries)
//   - tile data blob (compressed individual tiles)
//
// Tile addresses are mapped to a single 64-bit ID via a Hilbert curve
// per zoom level + a per-zoom offset, which keeps the archive sorted in
// a way that maximises cache locality.

// HeaderSize is the fixed PMTiles v3 header length.
const HeaderSize = 127

// HeaderV3 mirrors the v3 spec. Only the fields CarTUI consults are kept.
type HeaderV3 struct {
	RootDirOffset    uint64
	RootDirLength    uint64
	JSONOffset       uint64
	JSONLength       uint64
	LeafDirOffset    uint64
	LeafDirLength    uint64
	TileDataOffset   uint64
	TileDataLength   uint64
	AddressedTiles   uint64
	TileEntries      uint64
	TileContents     uint64
	Clustered        bool
	InternalCompress uint8 // 1=none, 2=gzip
	TileCompress     uint8
	TileType         uint8
	MinZoom          uint8
	MaxZoom          uint8
}

// Compression types.
const (
	CompressNone = 1
	CompressGzip = 2
)

// Tile types.
const (
	TileTypeMVT = 1
	TileTypePNG = 2
)

// EntryV3 is a single directory entry. Either points to another leaf
// directory (when RunLength==0) or to a tile blob.
type EntryV3 struct {
	TileID    uint64
	Offset    uint64
	Length    uint32
	RunLength uint32
}

// PMTiles is a read-only client for a single PMTiles archive, served
// either from disk (`file:///` or relative path) or via HTTP range
// requests.
type PMTiles struct {
	source pmtilesSource
	header HeaderV3

	// Cached root directory — reads are amortised across the lifetime of
	// the client.
	mu      sync.Mutex
	rootDir []EntryV3
}

// pmtilesSource abstracts byte-range access over either a local file or
// an HTTP endpoint.
type pmtilesSource interface {
	ReadRange(ctx context.Context, offset uint64, length uint32) ([]byte, error)
	Close() error
}

// NewPMTiles opens an archive. urlOrPath accepts:
//
//   - http:// or https:// URL: every read is a Range GET.
//   - file:// URL or local path: reads use os.File.ReadAt.
//
// The header is fetched and parsed eagerly so configuration mistakes
// surface at startup rather than on the first tile fetch.
func NewPMTiles(ctx context.Context, c *Client, urlOrPath string) (*PMTiles, error) {
	if strings.TrimSpace(urlOrPath) == "" {
		return nil, errors.New("pmtiles: empty source")
	}
	src, err := openSource(c, urlOrPath)
	if err != nil {
		return nil, err
	}
	headerBytes, err := src.ReadRange(ctx, 0, HeaderSize)
	if err != nil {
		_ = src.Close()
		return nil, fmt.Errorf("pmtiles: read header: %w", err)
	}
	hdr, err := DeserializeHeader(headerBytes)
	if err != nil {
		_ = src.Close()
		return nil, err
	}
	return &PMTiles{source: src, header: hdr}, nil
}

// Close releases the underlying file handle (no-op for HTTP).
func (p *PMTiles) Close() error { return p.source.Close() }

// Header exposes archive metadata (zoom range, compression, etc.).
func (p *PMTiles) Header() HeaderV3 { return p.header }

// Tile returns the raw tile blob (already gzip-decompressed when the
// archive marks tiles as gzip). Returns ErrNotFound when the archive
// does not contain the requested address.
func (p *PMTiles) Tile(ctx context.Context, z, x, y int) ([]byte, error) {
	if z < int(p.header.MinZoom) || z > int(p.header.MaxZoom) {
		return nil, ErrTileNotFound
	}
	tileID := ZxyToID(uint8(z), uint32(x), uint32(y))

	if err := p.loadRoot(ctx); err != nil {
		return nil, err
	}
	entry, found := findEntry(p.rootDir, tileID)
	if !found {
		return nil, ErrTileNotFound
	}
	// RunLength == 0 means the entry points to a leaf directory rather
	// than a tile.
	for entry.RunLength == 0 {
		leaf, err := p.loadLeaf(ctx, entry)
		if err != nil {
			return nil, err
		}
		next, ok := findEntry(leaf, tileID)
		if !ok {
			return nil, ErrTileNotFound
		}
		entry = next
	}

	blob, err := p.source.ReadRange(ctx, p.header.TileDataOffset+entry.Offset, entry.Length)
	if err != nil {
		return nil, fmt.Errorf("pmtiles: tile read: %w", err)
	}
	if p.header.TileCompress == CompressGzip {
		return gunzip(blob)
	}
	return blob, nil
}

// ErrTileNotFound is returned when the requested tile is not present in
// the archive (e.g. ocean-only tile, outside the bounds, or beyond the
// max zoom).
var ErrTileNotFound = errors.New("pmtiles: tile not found")

// loadRoot lazily fetches and parses the root directory.
func (p *PMTiles) loadRoot(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.rootDir != nil {
		return nil
	}
	blob, err := p.source.ReadRange(ctx, p.header.RootDirOffset, uint32(p.header.RootDirLength))
	if err != nil {
		return fmt.Errorf("pmtiles: read root dir: %w", err)
	}
	if p.header.InternalCompress == CompressGzip {
		blob, err = gunzip(blob)
		if err != nil {
			return err
		}
	}
	entries, err := DeserializeEntries(blob)
	if err != nil {
		return err
	}
	p.rootDir = entries
	return nil
}

// loadLeaf fetches a leaf directory pointed to by the given entry.
// Leaves are not cached — they are typically larger than the working
// set we'd want to keep in memory.
func (p *PMTiles) loadLeaf(ctx context.Context, e EntryV3) ([]EntryV3, error) {
	blob, err := p.source.ReadRange(ctx, p.header.LeafDirOffset+e.Offset, e.Length)
	if err != nil {
		return nil, fmt.Errorf("pmtiles: read leaf dir: %w", err)
	}
	if p.header.InternalCompress == CompressGzip {
		blob, err = gunzip(blob)
		if err != nil {
			return nil, err
		}
	}
	return DeserializeEntries(blob)
}

// findEntry binary-searches a sorted directory for the entry that owns
// the given tile id. An entry "owns" tile id `t` when
// entry.TileID ≤ t < entry.TileID + entry.RunLength (run_length>0) or
// entry.TileID ≤ t (when run_length==0; leaf-dir pointer).
func findEntry(entries []EntryV3, tileID uint64) (EntryV3, bool) {
	if len(entries) == 0 {
		return EntryV3{}, false
	}
	lo, hi := 0, len(entries)
	for lo < hi {
		mid := (lo + hi) / 2
		if entries[mid].TileID <= tileID {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	if lo == 0 {
		return EntryV3{}, false
	}
	e := entries[lo-1]
	if e.RunLength == 0 {
		return e, true // leaf pointer
	}
	if tileID < e.TileID+uint64(e.RunLength) {
		return e, true
	}
	return EntryV3{}, false
}

// DeserializeHeader parses the 127-byte PMTiles v3 header.
func DeserializeHeader(b []byte) (HeaderV3, error) {
	if len(b) < HeaderSize {
		return HeaderV3{}, fmt.Errorf("pmtiles: header too short (%d)", len(b))
	}
	if string(b[:7]) != "PMTiles" {
		return HeaderV3{}, fmt.Errorf("pmtiles: bad magic")
	}
	if b[7] != 3 {
		return HeaderV3{}, fmt.Errorf("pmtiles: unsupported version %d", b[7])
	}
	u64 := binary.LittleEndian.Uint64
	h := HeaderV3{
		RootDirOffset:    u64(b[8:]),
		RootDirLength:    u64(b[16:]),
		JSONOffset:       u64(b[24:]),
		JSONLength:       u64(b[32:]),
		LeafDirOffset:    u64(b[40:]),
		LeafDirLength:    u64(b[48:]),
		TileDataOffset:   u64(b[56:]),
		TileDataLength:   u64(b[64:]),
		AddressedTiles:   u64(b[72:]),
		TileEntries:      u64(b[80:]),
		TileContents:     u64(b[88:]),
		Clustered:        b[96] != 0,
		InternalCompress: b[97],
		TileCompress:     b[98],
		TileType:         b[99],
		MinZoom:          b[100],
		MaxZoom:          b[101],
	}
	return h, nil
}

// DeserializeEntries decodes a directory blob (already decompressed) into
// a slice of EntryV3. The on-wire format uses delta-coded varints — see
// the PMTiles v3 spec.
func DeserializeEntries(b []byte) ([]EntryV3, error) {
	r := pbf.New(b)
	n64, err := r.Varint()
	if err != nil {
		return nil, fmt.Errorf("pmtiles: entries count: %w", err)
	}
	n := int(n64)
	if n < 0 || n > 1<<24 {
		return nil, fmt.Errorf("pmtiles: implausible entry count %d", n)
	}
	out := make([]EntryV3, n)

	// Tile IDs: delta-encoded.
	var prev uint64
	for i := 0; i < n; i++ {
		d, err := r.Varint()
		if err != nil {
			return nil, fmt.Errorf("pmtiles: tile id at %d: %w", i, err)
		}
		prev += d
		out[i].TileID = prev
	}
	// Run lengths.
	for i := 0; i < n; i++ {
		v, err := r.Varint()
		if err != nil {
			return nil, fmt.Errorf("pmtiles: run length at %d: %w", i, err)
		}
		out[i].RunLength = uint32(v)
	}
	// Lengths.
	for i := 0; i < n; i++ {
		v, err := r.Varint()
		if err != nil {
			return nil, fmt.Errorf("pmtiles: length at %d: %w", i, err)
		}
		out[i].Length = uint32(v)
	}
	// Offsets: 0 means "right after the previous one"; otherwise it's
	// the absolute offset minus 1.
	for i := 0; i < n; i++ {
		v, err := r.Varint()
		if err != nil {
			return nil, fmt.Errorf("pmtiles: offset at %d: %w", i, err)
		}
		switch {
		case v == 0 && i > 0:
			out[i].Offset = out[i-1].Offset + uint64(out[i-1].Length)
		case v != 0:
			out[i].Offset = v - 1
		}
	}
	return out, nil
}

// ZxyToID converts a slippy-map address (z, x, y) to the PMTiles
// linear tile id by walking the Hilbert curve at zoom z and adding the
// number of tiles covered by smaller zooms.
func ZxyToID(z uint8, x, y uint32) uint64 {
	var acc uint64
	for tz := uint8(0); tz < z; tz++ {
		acc += uint64(1) << (2 * tz)
	}
	n := uint32(1) << z
	rx, ry := uint32(0), uint32(0)
	d := uint64(0)
	for s := n / 2; s > 0; s /= 2 {
		if (x & s) > 0 {
			rx = 1
		} else {
			rx = 0
		}
		if (y & s) > 0 {
			ry = 1
		} else {
			ry = 0
		}
		d += uint64(s) * uint64(s) * uint64((3*rx)^ry)
		// rotate quadrant
		if ry == 0 {
			if rx == 1 {
				x = s - 1 - x
				y = s - 1 - y
			}
			x, y = y, x
		}
	}
	return acc + d
}

// gunzip decompresses a gzip blob.
func gunzip(b []byte) ([]byte, error) {
	zr, err := gzip.NewReader(bytes.NewReader(b))
	if err != nil {
		return nil, fmt.Errorf("pmtiles: gunzip: %w", err)
	}
	defer func() { _ = zr.Close() }()
	out, err := io.ReadAll(zr)
	if err != nil {
		return nil, fmt.Errorf("pmtiles: gunzip read: %w", err)
	}
	return out, nil
}

// openSource picks the right backend depending on the URL scheme.
func openSource(c *Client, urlOrPath string) (pmtilesSource, error) {
	u, err := url.Parse(urlOrPath)
	if err == nil && (u.Scheme == "http" || u.Scheme == "https") {
		return &httpRangeSource{client: c, url: urlOrPath}, nil
	}
	if err == nil && u.Scheme == "file" {
		return openFileSource(u.Path)
	}
	return openFileSource(urlOrPath)
}

// fileSource reads byte ranges from an *os.File.
type fileSource struct {
	f    *os.File
	size int64
}

func openFileSource(path string) (*fileSource, error) {
	f, err := os.Open(path) //nolint:gosec // path comes from user config
	if err != nil {
		return nil, fmt.Errorf("pmtiles: open %s: %w", path, err)
	}
	st, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, err
	}
	return &fileSource{f: f, size: st.Size()}, nil
}

func (f *fileSource) ReadRange(_ context.Context, offset uint64, length uint32) ([]byte, error) {
	buf := make([]byte, length)
	if _, err := f.f.ReadAt(buf, int64(offset)); err != nil {
		return nil, fmt.Errorf("pmtiles: ReadAt %d/%d: %w", offset, length, err)
	}
	return buf, nil
}

func (f *fileSource) Close() error { return f.f.Close() }

// httpRangeSource issues HTTP Range GETs against a remote PMTiles file.
type httpRangeSource struct {
	client *Client
	url    string
}

func (h *httpRangeSource) ReadRange(ctx context.Context, offset uint64, length uint32) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, h.url, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("pmtiles: build request: %w", err)
	}
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", offset, offset+uint64(length)-1))
	resp, err := h.client.Do(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("pmtiles: range fetch: %w", err)
	}
	defer drain(resp)
	if resp.StatusCode != http.StatusPartialContent && resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("pmtiles: unexpected status %d", resp.StatusCode)
	}
	buf, err := io.ReadAll(io.LimitReader(resp.Body, int64(length)+16))
	if err != nil {
		return nil, fmt.Errorf("pmtiles: read body: %w", err)
	}
	return buf, nil
}

func (h *httpRangeSource) Close() error { return nil }
