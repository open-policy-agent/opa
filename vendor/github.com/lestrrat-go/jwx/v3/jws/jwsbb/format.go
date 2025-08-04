package jwsbb

import (
	"bytes"
	"errors"
	"io"

	"github.com/lestrrat-go/jwx/v3/internal/base64"
	"github.com/lestrrat-go/jwx/v3/internal/jwxio"
	"github.com/lestrrat-go/jwx/v3/internal/tokens"
)

// SignBuffer combines the base64-encoded header and payload into a single byte slice
// for signing purposes. This creates the signing input according to JWS specification (RFC 7515).
// The result should be passed to signature generation functions.
//
// Parameters:
//   - buf: Reusable buffer (can be nil for automatic allocation)
//   - hdr: Raw header bytes (will be base64-encoded)
//   - payload: Raw payload bytes (encoded based on encodePayload flag)
//   - encoder: Base64 encoder to use for encoding components
//   - encodePayload: If true, payload is base64-encoded; if false, payload is used as-is
//
// Returns the constructed signing input in the format: base64(header).base64(payload) or base64(header).payload
func SignBuffer(buf, hdr, payload []byte, encoder base64.Encoder, encodePayload bool) []byte {
	l := encoder.EncodedLen(len(hdr)+len(payload)) + 1
	if cap(buf) < l {
		buf = make([]byte, 0, l)
	}
	buf = buf[:0]
	buf = encoder.AppendEncode(buf, hdr)
	buf = append(buf, tokens.Period)
	if encodePayload {
		buf = encoder.AppendEncode(buf, payload)
	} else {
		buf = append(buf, payload...)
	}

	return buf
}

// AppendSignature appends a base64-encoded signature to a JWS signing input buffer.
// This completes the compact JWS serialization by adding the final signature component.
// The input buffer should contain the signing input (header.payload), and this function
// adds the period separator and base64-encoded signature.
//
// Parameters:
//   - buf: Buffer containing the signing input (typically from SignBuffer)
//   - signature: Raw signature bytes (will be base64-encoded)
//   - encoder: Base64 encoder to use for encoding the signature
//
// Returns the complete compact JWS in the format: base64(header).base64(payload).base64(signature)
func AppendSignature(buf, signature []byte, encoder base64.Encoder) []byte {
	l := len(buf) + len(signature) + 1
	if cap(buf) < l {
		buf = make([]byte, 0, l)
	}
	buf = append(buf, tokens.Period)
	buf = encoder.AppendEncode(buf, signature)

	return buf
}

// JoinCompact creates a complete compact JWS serialization from individual components.
// This is a one-step function that combines header, payload, and signature into the final JWS format.
// It includes safety checks to prevent excessive memory allocation.
//
// Parameters:
//   - buf: Reusable buffer (can be nil for automatic allocation)
//   - hdr: Raw header bytes (will be base64-encoded)
//   - payload: Raw payload bytes (encoded based on encodePayload flag)
//   - signature: Raw signature bytes (will be base64-encoded)
//   - encoder: Base64 encoder to use for encoding all components
//   - encodePayload: If true, payload is base64-encoded; if false, payload is used as-is
//
// Returns the complete compact JWS or an error if the total size exceeds safety limits (1GB).
func JoinCompact(buf, hdr, payload, signature []byte, encoder base64.Encoder, encodePayload bool) ([]byte, error) {
	const MaxBufferSize = 1 << 30 // 1 GB
	totalSize := len(hdr) + len(payload) + len(signature) + 2
	if totalSize > MaxBufferSize {
		return nil, errors.New("input sizes exceed maximum allowable buffer size")
	}
	if cap(buf) < totalSize {
		buf = make([]byte, 0, totalSize)
	}
	buf = buf[:0]
	buf = encoder.AppendEncode(buf, hdr)
	buf = append(buf, tokens.Period)
	if encodePayload {
		buf = encoder.AppendEncode(buf, payload)
	} else {
		buf = append(buf, payload...)
	}
	buf = append(buf, tokens.Period)
	buf = encoder.AppendEncode(buf, signature)

	return buf, nil
}

var compactDelim = []byte{tokens.Period}

var errInvalidNumberOfSegments = errors.New(`jwsbb: invalid number of segments`)

// InvalidNumberOfSegmentsError returns the standard error for invalid JWS segment count.
// A valid compact JWS must have exactly 3 segments separated by periods: header.payload.signature
func InvalidNumberOfSegmentsError() error {
	return errInvalidNumberOfSegments
}

// SplitCompact parses a compact JWS serialization into its three components.
// This function validates that the input has exactly 3 segments separated by periods
// and returns the base64-encoded components without decoding them.
//
// Parameters:
//   - src: Complete compact JWS string as bytes
//
// Returns:
//   - protected: Base64-encoded protected header
//   - payload: Base64-encoded payload (or raw payload if b64=false was used)
//   - signature: Base64-encoded signature
//   - err: Error if the format is invalid or segment count is wrong
func SplitCompact(src []byte) (protected, payload, signature []byte, err error) {
	var s []byte
	var ok bool

	protected, s, ok = bytes.Cut(src, compactDelim)
	if !ok { // no period found
		return nil, nil, nil, InvalidNumberOfSegmentsError()
	}
	payload, s, ok = bytes.Cut(s, compactDelim)
	if !ok { // only one period found
		return nil, nil, nil, InvalidNumberOfSegmentsError()
	}
	signature, _, ok = bytes.Cut(s, compactDelim)
	if ok { // three periods found
		return nil, nil, nil, InvalidNumberOfSegmentsError()
	}
	return protected, payload, signature, nil
}

// SplitCompactString is a convenience wrapper around SplitCompact for string inputs.
// It converts the string to bytes and parses the compact JWS serialization.
//
// Parameters:
//   - src: Complete compact JWS as a string
//
// Returns the same components as SplitCompact: protected header, payload, signature, and error.
func SplitCompactString(src string) (protected, payload, signature []byte, err error) {
	return SplitCompact([]byte(src))
}

// SplitCompactReader parses a compact JWS serialization from an io.Reader.
// This function handles both finite and streaming sources efficiently.
// For finite sources, it reads all data at once. For streaming sources,
// it uses a buffer-based approach to find segment boundaries.
//
// Parameters:
//   - rdr: Reader containing the compact JWS data
//
// Returns:
//   - protected: Base64-encoded protected header
//   - payload: Base64-encoded payload (or raw payload if b64=false was used)
//   - signature: Base64-encoded signature
//   - err: Error if reading fails or the format is invalid
//
// The function validates that exactly 3 segments are present, separated by periods.
func SplitCompactReader(rdr io.Reader) (protected, payload, signature []byte, err error) {
	data, err := jwxio.ReadAllFromFiniteSource(rdr)
	if err == nil {
		return SplitCompact(data)
	}

	if !errors.Is(err, jwxio.NonFiniteSourceError()) {
		return nil, nil, nil, err
	}

	var periods int
	var state int

	buf := make([]byte, 4096)
	var sofar []byte

	for {
		// read next bytes
		n, err := rdr.Read(buf)
		// return on unexpected read error
		if err != nil && err != io.EOF {
			return nil, nil, nil, io.ErrUnexpectedEOF
		}

		// append to current buffer
		sofar = append(sofar, buf[:n]...)
		// loop to capture multiple tokens.Period in current buffer
		for loop := true; loop; {
			var i = bytes.IndexByte(sofar, tokens.Period)
			if i == -1 && err != io.EOF {
				// no tokens.Period found -> exit and read next bytes (outer loop)
				loop = false
				continue
			} else if i == -1 && err == io.EOF {
				// no tokens.Period found -> process rest and exit
				i = len(sofar)
				loop = false
			} else {
				// tokens.Period found
				periods++
			}

			// Reaching this point means we have found a tokens.Period or EOF and process the rest of the buffer
			switch state {
			case 0:
				protected = sofar[:i]
				state++
			case 1:
				payload = sofar[:i]
				state++
			case 2:
				signature = sofar[:i]
			}
			// Shorten current buffer
			if len(sofar) > i {
				sofar = sofar[i+1:]
			}
		}
		// Exit on EOF
		if err == io.EOF {
			break
		}
	}
	if periods != 2 {
		return nil, nil, nil, InvalidNumberOfSegmentsError()
	}

	return protected, payload, signature, nil
}
