package leb128

import (
	"bytes"
	"errors"
	"fmt"
)

const (
	maxVarintLen32 = 5
	maxVarintLen64 = 10
)

var (
	errOverflow32 = errors.New("overflows a 32-bit integer")
	errOverflow33 = errors.New("overflows a 33-bit integer")
	errOverflow64 = errors.New("overflows a 64-bit integer")
)

// encodeCache reduces allocations by returning a constant slice of size one for values that can encode in a single byte
var encodeCache = [0x80][1]byte{
	{0x00}, {0x01}, {0x02}, {0x03}, {0x04}, {0x05}, {0x06}, {0x07}, {0x08}, {0x09}, {0x0a}, {0x0b}, {0x0c}, {0x0d}, {0x0e}, {0x0f},
	{0x10}, {0x11}, {0x12}, {0x13}, {0x14}, {0x15}, {0x16}, {0x17}, {0x18}, {0x19}, {0x1a}, {0x1b}, {0x1c}, {0x1d}, {0x1e}, {0x1f},
	{0x20}, {0x21}, {0x22}, {0x23}, {0x24}, {0x25}, {0x26}, {0x27}, {0x28}, {0x29}, {0x2a}, {0x2b}, {0x2c}, {0x2d}, {0x2e}, {0x2f},
	{0x30}, {0x31}, {0x32}, {0x33}, {0x34}, {0x35}, {0x36}, {0x37}, {0x38}, {0x39}, {0x3a}, {0x3b}, {0x3c}, {0x3d}, {0x3e}, {0x3f},
	{0x40}, {0x41}, {0x42}, {0x43}, {0x44}, {0x45}, {0x46}, {0x47}, {0x48}, {0x49}, {0x4a}, {0x4b}, {0x4c}, {0x4d}, {0x4e}, {0x4f},
	{0x50}, {0x51}, {0x52}, {0x53}, {0x54}, {0x55}, {0x56}, {0x57}, {0x58}, {0x59}, {0x5a}, {0x5b}, {0x5c}, {0x5d}, {0x5e}, {0x5f},
	{0x60}, {0x61}, {0x62}, {0x63}, {0x64}, {0x65}, {0x66}, {0x67}, {0x68}, {0x69}, {0x6a}, {0x6b}, {0x6c}, {0x6d}, {0x6e}, {0x6f},
	{0x70}, {0x71}, {0x72}, {0x73}, {0x74}, {0x75}, {0x76}, {0x77}, {0x78}, {0x79}, {0x7a}, {0x7b}, {0x7c}, {0x7d}, {0x7e}, {0x7f},
}

// EncodeInt32 encodes the signed value into a buffer in LEB128 format
//
// See https://en.wikipedia.org/wiki/LEB128#Encode_signed_integer
func EncodeInt32(value int32) []byte {
	return EncodeInt64(int64(value))
}

// EncodeInt64 encodes the signed value into a buffer in LEB128 format
//
// See https://en.wikipedia.org/wiki/LEB128#Encode_signed_integer
func EncodeInt64(value int64) (buf []byte) {
	for {
		// Take 7 remaining low-order bits from the value into b.
		b := uint8(value & 0x7f)
		// Extract the sign bit.
		s := uint8(value & 0x40)
		value >>= 7

		// The encoding unsigned numbers is simpler as it only needs to check if the value is non-zero to tell if there
		// are more bits to encode. Signed is a little more complicated as you have to double-check the sign bit.
		// If either case, set the high-order bit to tell the reader there are more bytes in this int.
		if (value != -1 || s == 0) && (value != 0 || s != 0) {
			b |= 0x80
		}

		// Append b into the buffer
		buf = append(buf, b)
		if b&0x80 == 0 {
			break
		}
	}
	return buf
}

// EncodeUint32 encodes the value into a buffer in LEB128 format
//
// See https://en.wikipedia.org/wiki/LEB128#Encode_unsigned_integer
func EncodeUint32(value uint32) []byte {
	return EncodeUint64(uint64(value))
}

// EncodeUint64 encodes the value into a buffer in LEB128 format
//
// See https://en.wikipedia.org/wiki/LEB128#Encode_unsigned_integer
func EncodeUint64(value uint64) (buf []byte) {
	if value < 0x80 {
		return encodeCache[value][:]
	}

	// This is effectively a do/while loop where we take 7 bits of the value and encode them until it is zero.
	for {
		// Take 7 remaining low-order bits from the value into b.
		b := uint8(value & 0x7f)
		value = value >> 7

		// If there are remaining bits, the value won't be zero: Set the high-
		// order bit to tell the reader there are more bytes in this uint.
		if value != 0 {
			b |= 0x80
		}

		// Append b into the buffer
		buf = append(buf, b)
		if b&0x80 == 0 {
			return buf
		}
	}
}

func DecodeUint32(r *bytes.Reader) (ret uint32, bytesRead uint64, err error) {
	// Derived from https://github.com/golang/go/blob/aafad20b617ee63d58fcd4f6e0d98fe27760678c/src/encoding/binary/varint.go
	// with the modification on the overflow handling tailored for 32-bits.
	var s uint32
	for i := 0; i < maxVarintLen32; i++ {
		b, err := r.ReadByte()
		if err != nil {
			return 0, 0, err
		}
		if b < 0x80 {
			// Unused bits must be all zero.
			if i == maxVarintLen32-1 && (b&0xf0) > 0 {
				return 0, 0, errOverflow32
			}
			return ret | uint32(b)<<s, uint64(i) + 1, nil
		}
		ret |= (uint32(b) & 0x7f) << s
		s += 7
	}
	return 0, 0, errOverflow32
}

func DecodeUint64(r *bytes.Reader) (ret uint64, bytesRead uint64, err error) {
	// Derived from https://github.com/golang/go/blob/aafad20b617ee63d58fcd4f6e0d98fe27760678c/src/encoding/binary/varint.go
	var s uint64
	for i := 0; i < maxVarintLen64; i++ {
		b, err := r.ReadByte()
		if err != nil {
			return 0, 0, err
		}
		if b < 0x80 {
			// Unused bits (non first bit) must all be zero.
			if i == maxVarintLen64-1 && b > 1 {
				return 0, 0, errOverflow64
			}
			return ret | uint64(b)<<s, uint64(i) + 1, nil
		}
		ret |= (uint64(b) & 0x7f) << s
		s += 7
	}
	return 0, 0, errOverflow64
}

func DecodeInt32(r *bytes.Reader) (ret int32, bytesRead uint64, err error) {
	var shift int
	var b byte
	for {
		b, err = r.ReadByte()
		if err != nil {
			return 0, 0, fmt.Errorf("readByte failed: %w", err)
		}
		ret |= (int32(b) & 0x7f) << shift
		shift += 7
		bytesRead++
		if b&0x80 == 0 {
			if shift < 32 && (b&0x40) != 0 {
				ret |= ^0 << shift
			}
			// Over flow checks.
			// fixme: can be optimized.
			if bytesRead > 5 {
				return 0, 0, errOverflow32
			} else if unused := b & 0b00110000; bytesRead == 5 && ret < 0 && unused != 0b00110000 {
				return 0, 0, errOverflow32
			} else if bytesRead == 5 && ret >= 0 && unused != 0x00 {
				return 0, 0, errOverflow32
			}
			return
		}
	}
}

// DecodeInt33AsInt64 is a special cased decoder for wasm.BlockType which is encoded as a positive signed integer, yet
// still needs to fit the 32-bit range of allowed indices. Hence, this is 33, not 32-bit!
//
// See https://webassembly.github.io/spec/core/binary/instructions.html#control-instructions
func DecodeInt33AsInt64(r *bytes.Reader) (ret int64, bytesRead uint64, err error) {
	const (
		int33Mask  int64 = 1 << 7
		int33Mask2       = ^int33Mask
		int33Mask3       = 1 << 6
		int33Mask4       = 8589934591 // 2^33-1
		int33Mask5       = 1 << 32
		int33Mask6       = int33Mask4 + 1 // 2^33
	)
	var shift int
	var b int64
	var rb byte
	for shift < 35 {
		rb, err = r.ReadByte()
		if err != nil {
			return 0, 0, fmt.Errorf("readByte failed: %w", err)
		}
		b = int64(rb)
		ret |= (b & int33Mask2) << shift
		shift += 7
		bytesRead++
		if b&int33Mask == 0 {
			break
		}
	}

	// fixme: can be optimized
	if shift < 33 && (b&int33Mask3) == int33Mask3 {
		ret |= int33Mask4 << shift
	}
	ret = ret & int33Mask4

	// if 33rd bit == 1, we translate it as a corresponding signed-33bit minus value
	if ret&int33Mask5 > 0 {
		ret = ret - int33Mask6
	}
	// Over flow checks.
	// fixme: can be optimized.
	if bytesRead > 5 {
		return 0, 0, errOverflow33
	} else if unused := b & 0b00100000; bytesRead == 5 && ret < 0 && unused != 0b00100000 {
		return 0, 0, errOverflow33
	} else if bytesRead == 5 && ret >= 0 && unused != 0x00 {
		return 0, 0, errOverflow33
	}
	return ret, bytesRead, nil
}

func DecodeInt64(r *bytes.Reader) (ret int64, bytesRead uint64, err error) {
	const (
		int64Mask3 = 1 << 6
		int64Mask4 = ^0
	)
	var shift int
	var b byte
	for {
		b, err = r.ReadByte()
		if err != nil {
			return 0, 0, fmt.Errorf("readByte failed: %w", err)
		}
		ret |= (int64(b) & 0x7f) << shift
		shift += 7
		bytesRead++
		if b&0x80 == 0 {
			if shift < 64 && (b&int64Mask3) == int64Mask3 {
				ret |= int64Mask4 << shift
			}
			// Over flow checks.
			// fixme: can be optimized.
			if bytesRead > 10 {
				return 0, 0, errOverflow64
			} else if unused := b & 0b00111110; bytesRead == 10 && ret < 0 && unused != 0b00111110 {
				return 0, 0, errOverflow64
			} else if bytesRead == 10 && ret >= 0 && unused != 0x00 {
				return 0, 0, errOverflow64
			}
			return
		}
	}
}
