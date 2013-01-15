/* Copyright 2012 Marc-Antoine Ruel. Licensed under the Apache License, Version
2.0 (the "License"); you may not use this file except in compliance with the
License.  You may obtain a copy of the License at
http://www.apache.org/licenses/LICENSE-2.0. Unless required by applicable law or
agreed to in writing, software distributed under the License is distributed on
an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express
or implied. See the License for the specific language governing permissions and
limitations under the License. */

package main

import (
	"crypto/sha1"
	"encoding/binary"
	"github.com/maruel/rs"
	"hash"
	"io"
)

const BlockSize = 128

// Defines a layer of ECC codes. For larger files multiple layers of ECC codes
// can be used to cross-reference the valid blocks to regenerate missing
// blocks.
type Layer struct {
	Stride   int64
	Size     int
	EccBytes int
}

type layerToProcess struct {
	layer  Layer
	filled int
	buffer [BlockSize]byte
	check  []byte
	rs     rs.Encoder
}

//
//
// For files smaller than 128b
// - Each 128b block has a (len(file) / 8) bytes of ECC check
//   Overhead: 8/128
//
//
//
// For files larger than 128 bytes

// - Each 1b stride across 128 blocks of 128b have 2 bytes ECC check and loop with files larger than 16kb.
//   Overhead: 2/128
// - Each 1b stride across 128 blocks of 4kb have 4 bytes ECC check, loop at 512kb.
//   Overhead: 4/128
//
// Total overhead: 16/128 or 12.5%
//
// Note: 'b' means 'byte'

// GetLayers returns the layers that should be used to generate the ECC
// depending on the file size and the desired ranges of size.
//
// It works by scaling the number of ECC bytes by adding multiple layers of ECC
// as the order of magnitude of the file size increases.
//
// For now the block size is hardcoded to 128 bytes for efficiency reason. So
// lower and higher bounds are in term of 128 bytes.
func GetLayers(size int64, lower int, higher int) []Layer {
	if size <= 256 {
		return []Layer{{0, BlockSize, lower}}
	}
	return nil
}

type eccObj struct {
	eccHeader
	dst    io.Writer
	field  *rs.Field
	layers []layerToProcess
	h      hash.Hash
}

type eccHeader struct {
	format    int32
	poly      int32 // Defines the Galois finite field.
	alpha     int32 // Actually a byte.
}

type eccTrailer struct {
	sha1 [20]byte
}

func (e *eccObj) Write(p []byte) (int, error) {
	for _, layer := range e.layers {
		layer.buffer[0] = p[0]
		/*
			for e.layer1Length+len(p) >= int(e.blockSize) {
				var input []byte
				if e.layer1Length != 0 || len(p) < int(e.blockSize) {
					// Use layer1.
					missing := int(e.blockSize) - e.layer1Length
					copy(e.layer1[e.layer1Length:], p[:missing])
					input = e.layer1
					p = p[missing:]
				} else {
					// Use p directly to skip a copy.
					input = p[:e.blockSize]
					p = p[e.blockSize:]
				}
				if err := e.flush(input); err != nil {
					return 0, err
				}
			}
			if len(p) != 0 {
				// Buffer the remainder.
				copy(e.layer1, p)
			}
			e.layer1Length = len(p)
		*/
	}
	return len(p), nil
}

func (e *eccObj) Close() {
	for _, layer := range e.layers {
		layer.buffer[0] = 0
		/*
			if e.layer1Length != 0 {
				// Writes the remaining.
				if err := e.flush(e.layer1[:e.layer1Length]); err != nil {
					return
				}
			}
			e.layer1Length = 0
		*/
	}
	t := eccTrailer{}
	e.h.Sum(t.sha1[:0])
	e.dst.Write(t.sha1[:])
	e.dst = nil
}

// Writes an ECC block.
func (e *eccObj) flush(p []byte) error {
	var err error
	for _, layer := range e.layers {
		layer.rs.Encode(p, layer.check)
		e.h.Write(layer.check)
		_, err = e.dst.Write(layer.check)
	}
	return err
}

func (e *eccHeader) write(dst io.Writer) error {
	if err := binary.Write(dst, binary.LittleEndian, uint32(e.format)); err != nil {
		return err
	}
	if err := binary.Write(dst, binary.LittleEndian, uint32(e.poly)); err != nil {
		return err
	}
	if err := binary.Write(dst, binary.LittleEndian, uint32(e.alpha)); err != nil {
		return err
	}
	/*
	if err := binary.Write(dst, binary.LittleEndian, uint32(e.blockSize)); err != nil {
		return err
	}
	if err := binary.Write(dst, binary.LittleEndian, uint32(e.eccBytesPerBlock)); err != nil {
		return err
	}
	*/
	return nil
}

// Generates an Error Correcting Encoder.
func NewECC(dst io.Writer) *eccObj {
	// TODO(maruel): x^8 + x^4 + x^3 + x^2 + 1 is what QR codes use but it is not
	// an indication that it is a good choice..
	h := eccHeader{0, 0x11d, 2}
	e := &eccObj{
		eccHeader: h,
		dst:       dst,
		field:     rs.NewField(int(h.poly), byte(h.alpha)),
		//layers:    make([]byte, h.blockSize),
		h:     sha1.New(),
	}
	h.write(e.h)
	if err := h.write(dst); err != nil {
		return nil
	}
	//e.rs = rs.NewEncoder(e.field, int(e.eccBytesPerBlock))
	return e
}
