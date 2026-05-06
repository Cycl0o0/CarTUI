// SPDX-FileCopyrightText: 2026 Cycl0o0 <contact@cyclooo.fr>
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package pbf is a tiny protobuf wire-format decoder. It only implements
// what CarTUI needs to read Mapbox Vector Tiles and PMTiles directories:
// varints, zigzag varints, length-delimited fields and 32/64-bit fixed
// integers. No code generation, no schema, no `google.golang.org/protobuf`
// dependency.
package pbf

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
)

// ErrTruncated is returned when the buffer ends mid-message.
var ErrTruncated = errors.New("pbf: truncated message")

// Wire types from the protobuf spec.
const (
	WireVarint = 0
	Wire64bit  = 1
	WireBytes  = 2
	Wire32bit  = 5
)

// Reader walks a protobuf-encoded byte slice.
type Reader struct {
	data []byte
	pos  int
}

// New wraps a buffer.
func New(data []byte) *Reader { return &Reader{data: data} }

// Pos returns the current read position.
func (r *Reader) Pos() int { return r.pos }

// Done reports whether the buffer has been fully consumed.
func (r *Reader) Done() bool { return r.pos >= len(r.data) }

// Remaining returns the number of unread bytes.
func (r *Reader) Remaining() int { return len(r.data) - r.pos }

// Tag reads the next field tag and returns (field number, wire type).
// Returns io.EOF when the buffer is exhausted.
func (r *Reader) Tag() (field int, wire int, err error) {
	if r.Done() {
		return 0, 0, io.EOF
	}
	v, err := r.Varint()
	if err != nil {
		return 0, 0, err
	}
	return int(v >> 3), int(v & 0x7), nil
}

// Varint reads a base-128 varint.
func (r *Reader) Varint() (uint64, error) {
	var x uint64
	var shift uint
	for i := 0; i < 10; i++ {
		if r.pos >= len(r.data) {
			return 0, ErrTruncated
		}
		b := r.data[r.pos]
		r.pos++
		x |= uint64(b&0x7F) << shift
		if b < 0x80 {
			return x, nil
		}
		shift += 7
	}
	return 0, fmt.Errorf("pbf: varint overflow at %d", r.pos)
}

// Sint reads a zigzag-encoded varint as signed int64.
func (r *Reader) Sint() (int64, error) {
	v, err := r.Varint()
	if err != nil {
		return 0, err
	}
	return ZigZagDecode(v), nil
}

// Bool reads a varint and interprets it as a boolean.
func (r *Reader) Bool() (bool, error) {
	v, err := r.Varint()
	return v != 0, err
}

// String reads a length-delimited UTF-8 string.
func (r *Reader) String() (string, error) {
	b, err := r.Bytes()
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// Bytes reads a length-delimited byte slice. The returned slice aliases
// the underlying buffer and must not be mutated by the caller.
func (r *Reader) Bytes() ([]byte, error) {
	n, err := r.Varint()
	if err != nil {
		return nil, err
	}
	end := r.pos + int(n)
	if end > len(r.data) || end < r.pos {
		return nil, ErrTruncated
	}
	out := r.data[r.pos:end]
	r.pos = end
	return out, nil
}

// Fixed32 reads a 4-byte little-endian unsigned integer.
func (r *Reader) Fixed32() (uint32, error) {
	if r.pos+4 > len(r.data) {
		return 0, ErrTruncated
	}
	v := binary.LittleEndian.Uint32(r.data[r.pos:])
	r.pos += 4
	return v, nil
}

// Fixed64 reads an 8-byte little-endian unsigned integer.
func (r *Reader) Fixed64() (uint64, error) {
	if r.pos+8 > len(r.data) {
		return 0, ErrTruncated
	}
	v := binary.LittleEndian.Uint64(r.data[r.pos:])
	r.pos += 8
	return v, nil
}

// Float reads a 32-bit IEEE-754 float.
func (r *Reader) Float() (float32, error) {
	v, err := r.Fixed32()
	return math.Float32frombits(v), err
}

// Double reads a 64-bit IEEE-754 float.
func (r *Reader) Double() (float64, error) {
	v, err := r.Fixed64()
	return math.Float64frombits(v), err
}

// PackedVarint reads a length-delimited block of packed varints.
func (r *Reader) PackedVarint() ([]uint64, error) {
	b, err := r.Bytes()
	if err != nil {
		return nil, err
	}
	sub := New(b)
	var out []uint64
	for !sub.Done() {
		v, err := sub.Varint()
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

// Skip advances past a field of the given wire type.
func (r *Reader) Skip(wire int) error {
	switch wire {
	case WireVarint:
		_, err := r.Varint()
		return err
	case Wire64bit:
		if r.pos+8 > len(r.data) {
			return ErrTruncated
		}
		r.pos += 8
		return nil
	case WireBytes:
		_, err := r.Bytes()
		return err
	case Wire32bit:
		if r.pos+4 > len(r.data) {
			return ErrTruncated
		}
		r.pos += 4
		return nil
	}
	return fmt.Errorf("pbf: unsupported wire type %d", wire)
}

// ZigZagDecode reverses the zigzag encoding used for signed integers in
// protobuf.
func ZigZagDecode(v uint64) int64 {
	return int64((v >> 1) ^ -(v & 1))
}

// EncodeVarint writes a base-128 varint into out and returns the new
// length. Used by the test harness; not by the runtime.
func EncodeVarint(out []byte, v uint64) []byte {
	for v >= 0x80 {
		out = append(out, byte(v)|0x80)
		v >>= 7
	}
	return append(out, byte(v))
}
