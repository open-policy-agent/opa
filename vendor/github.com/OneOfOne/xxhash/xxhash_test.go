package xxhash_test

import (
	"bytes"
	"encoding/binary"
	"hash/adler32"
	"hash/crc32"
	"hash/crc64"
	"hash/fnv"
	"os"
	"strconv"
	"testing"

	"github.com/OneOfOne/xxhash"
)

const inS = `Lorem ipsum dolor sit amet, consectetur adipisicing elit, sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat. Duis aute irure dolor in reprehenderit in voluptate velit esse cillum dolore eu fugiat nulla pariatur. Excepteur sint occaecat cupidatat non proident, sunt in culpa qui officia deserunt mollit anim id est laborum.
Lorem ipsum dolor sit amet, consectetur adipisicing elit, sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat. Duis aute irure dolor in reprehenderit in voluptate velit esse cillum dolore eu fugiat nulla pariatur. Excepteur sint occaecat cupidatat non proident, sunt in culpa qui officia deserunt mollit anim id est laborum.
Lorem ipsum dolor sit amet, consectetur adipisicing elit, sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat. Duis aute irure dolor in reprehenderit in voluptate velit esse cillum dolore eu fugiat nulla pariatur. Excepteur sint occaecat cupidatat non proident, sunt in culpa qui officia deserunt mollit anim id est laborum.
Lorem ipsum dolor sit amet, consectetur adipisicing elit, sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat. Duis aute irure dolor in reprehenderit in voluptate velit esse cillum dolore eu fugiat nulla pariatur. Excepteur sint occaecat cupidatat non proident, sunt in culpa qui officia deserunt mollit anim id est laborum.
Lorem ipsum dolor sit amet, consectetur adipisicing elit, sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat. Duis aute irure dolor in reprehenderit in voluptate velit esse cillum dolore eu fugiat nulla pariatur. Excepteur sint occaecat cupidatat non proident, sunt in culpa qui officia deserunt mollit anim id est laborum.
`

var (
	in = []byte(inS)
)

const (
	expected32 uint32 = 0x6101218F
	expected64 uint64 = 0xFFAE31BEBFED7652
)

func Test(t *testing.T) {
	t.Log("xxhash backend:", xxhash.Backend)
	t.Log("Benchmark string len:", len(inS))
}

func TestHash32(t *testing.T) {
	h := xxhash.New32()
	h.Write(in)
	r := h.Sum32()
	if r != expected32 {
		t.Errorf("expected 0x%x, got 0x%x.", expected32, r)
	}
}

func TestHash32Short(t *testing.T) {
	r := xxhash.Checksum32(in)
	if r != expected32 {
		t.Errorf("expected 0x%x, got 0x%x.", expected32, r)
	}
}

func TestWriteStringNil(t *testing.T) {
	h32, h64 := xxhash.New32(), xxhash.New64()
	for i := 0; i < 1e6; i++ {
		h32.WriteString("")
		h64.WriteString("")
	}
	_, _ = h32.Sum32(), h64.Sum64()
}

var testsTable = []struct {
	input string
	want  uint64
}{
	{"", 0xef46db3751d8e999},
	{"a", 0xd24ec4f1a98c6e5b},
	{"as", 0x1c330fb2d66be179},
	{"asd", 0x631c37ce72a97393},
	{"asdf", 0x415872f599cea71e},
	{
		// Exactly 63 characters, which exercises all code paths.
		"Call me Ishmael. Some years ago--never mind how long precisely-",
		0x02a2e85470d6fd96,
	},
	{"The quick brown fox jumps over the lazy dog http://i.imgur.com/VHQXScB.gif", 0x93267f9820452ead},
	{string(in), expected64},
}

// Shamelessly copied from https://github.com/cespare/xxhash/blob/5c37fe3735342a2e0d01c87a907579987c8936cc/xxhash_test.go#L28
func TestSum64(t *testing.T) {
	for i, tt := range testsTable {
		for chunkSize := 1; chunkSize <= len(tt.input); chunkSize++ {
			x := xxhash.New64()
			for j := 0; j < len(tt.input); j += chunkSize {
				end := j + chunkSize
				if end > len(tt.input) {
					end = len(tt.input)
				}
				chunk := tt.input[j:end]
				n, err := x.WriteString(chunk)
				if err != nil || n != len(chunk) {
					t.Fatalf("[i=%d,chunkSize=%d] Write: got (%d, %v); want (%d, nil)",
						i, chunkSize, n, err, len(chunk))
				}
			}
			if got := x.Sum64(); got != tt.want {
				t.Fatalf("[i=%d,chunkSize=%d] got 0x%x; want 0x%x",
					i, chunkSize, got, tt.want)
			}
			if got := x.Sum64(); got != tt.want {
				t.Fatalf("[i=%d,chunkSize=%d] got 0x%x; want 0x%x (called .Sum64 twice)",
					i, chunkSize, got, tt.want)
			}
			var b [8]byte
			binary.BigEndian.PutUint64(b[:], tt.want)
			if got := x.Sum(nil); !bytes.Equal(got, b[:]) {
				t.Fatalf("[i=%d,chunkSize=%d] Sum: got %v; want %v",
					i, chunkSize, got, b[:])
			}
		}

		if got := xxhash.ChecksumString64(tt.input); got != tt.want {
			t.Fatalf("[i=%d] ChecksumString64: got 0x%x; want 0x%x", i, got, tt.want)
		}
	}
}

func BenchmarkXXChecksum32(b *testing.B) {
	for i := 0; i < b.N; i++ {
		xxhash.Checksum32(in)
	}
}

func BenchmarkXXChecksumString32(b *testing.B) {
	for i := 0; i < b.N; i++ {
		xxhash.ChecksumString32(inS)
	}
}

func BenchmarkXXSum64(b *testing.B) {
	b.Run("Func", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			xxhash.Checksum64(in)
		}
	})
	b.Run("Struct", func(b *testing.B) {
		h := xxhash.New64()
		for i := 0; i < b.N; i++ {
			h.Write(in)
			h.Sum64()
			h.Reset()
		}
	})
}

func BenchmarkXXSum64Short(b *testing.B) {
	k := []byte("Test-key-1000")
	b.Run("Func", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			xxhash.Checksum64(k)
		}
	})
	b.Run("Struct", func(b *testing.B) {
		h := xxhash.New64()
		for i := 0; i < b.N; i++ {
			h.Write(k)
			h.Sum64()
			h.Reset()
		}
	})
}

func BenchmarkXXSum64EvenPoint(b *testing.B) {
	if os.Getenv("EP") == "" {
		b.SkipNow()
	}

	for i := 256; i < len(in); i += 256 {
		block := in[:i]
		b.Run("Func/"+strconv.Itoa(i), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				xxhash.Checksum64(block)
			}
		})
		b.Run("Struct/"+strconv.Itoa(i), func(b *testing.B) {
			h := xxhash.New64()
			for i := 0; i < b.N; i++ {
				h.Write(block)
				h.Sum64()
				h.Reset()
			}
		})
	}
}

func BenchmarkFnv32(b *testing.B) {
	var bv []byte
	h := fnv.New32()
	for i := 0; i < b.N; i++ {
		h.Write(in)
		bv = h.Sum(nil)
		h.Reset()
	}
	_ = bv
}

func BenchmarkAdler32(b *testing.B) {
	for i := 0; i < b.N; i++ {
		adler32.Checksum(in)
	}
}

func BenchmarkCRC32IEEE(b *testing.B) {
	for i := 0; i < b.N; i++ {
		crc32.ChecksumIEEE(in)
	}
}

func BenchmarkCRC32IEEEString(b *testing.B) {
	for i := 0; i < b.N; i++ {
		crc32.ChecksumIEEE([]byte(inS))
	}
}

var crc64ISO = crc64.MakeTable(crc64.ISO)

func BenchmarkCRC64ISO(b *testing.B) {
	for i := 0; i < b.N; i++ {
		crc64.Checksum(in, crc64ISO)
	}
}

func BenchmarkCRC64ISOString(b *testing.B) {
	for i := 0; i < b.N; i++ {
		crc64.Checksum([]byte(inS), crc64ISO)
	}
}

func BenchmarkCRC32IEEEShort(b *testing.B) {
	k := []byte("Test-key-100")

	for i := 0; i < b.N; i++ {
		crc32.ChecksumIEEE(k)
	}
}

func BenchmarkCRC64ISOShort(b *testing.B) {
	k := []byte("Test-key-100")
	for i := 0; i < b.N; i++ {
		crc64.Checksum(k, crc64ISO)
	}
}

func BenchmarkFnv64(b *testing.B) {
	h := fnv.New64()
	for i := 0; i < b.N; i++ {
		h.Write(in)
		h.Sum(nil)
		h.Reset()
	}
}

func BenchmarkFnv64Short(b *testing.B) {
	var bv []byte
	k := []byte("Test-key-100")
	for i := 0; i < b.N; i++ {
		h := fnv.New64()
		h.Write(k)
		bv = h.Sum(nil)
	}
	_ = bv
}
