// Package extgate is a client for the opencrab External gate V3 wire: LF-delimited
// JSON frames over a Unix domain socket. crab-town acts as the gateway; opencrab
// core listens on the socket.
package extgate

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"unicode/utf8"
)

// MaxFrame is the frame size limit, LF included.
const MaxFrame = 1_048_576

// Fatal frame errors. Each one closes the connection.
var (
	ErrTooLarge     = errors.New("too_large")
	ErrBadFrame     = errors.New("bad_request")
	ErrDuplicateKey = errors.New("bad_request: duplicate member")
)

var (
	uuidRE   = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
	digestRE = regexp.MustCompile(`^[0-9a-f]{64}$`)
)

func validUUID(s string) bool   { return uuidRE.MatchString(s) }
func validDigest(s string) bool { return digestRE.MatchString(s) }
func validReqID(s string) bool  { return len(s) >= 1 && len(s) <= 128 }

// readFrame returns one frame without its LF. More than MaxFrame bytes (LF
// included) before a LF is ErrTooLarge; the rest is not resynchronised.
func readFrame(r *bufio.Reader) ([]byte, error) {
	var buf []byte
	for {
		chunk, err := r.ReadSlice('\n')
		if len(buf)+len(chunk) > MaxFrame {
			return nil, ErrTooLarge
		}
		buf = append(buf, chunk...)
		switch {
		case err == nil:
			return buf[:len(buf)-1], nil
		case errors.Is(err, bufio.ErrBufferFull):
			// MaxFrame bytes without LF can never become a valid frame: the LF
			// itself would exceed the limit. Fail now instead of waiting for more.
			if len(buf) >= MaxFrame {
				return nil, ErrTooLarge
			}
			continue
		default:
			return nil, err // io.EOF etc: a partial frame is discarded
		}
	}
}

// decodeObject parses one JSON object, rejecting invalid UTF-8, non-objects,
// trailing data, and duplicate members at any depth.
func decodeObject(b []byte) (map[string]any, error) {
	if !utf8.Valid(b) {
		return nil, ErrBadFrame
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	tok, err := dec.Token()
	if err != nil {
		return nil, ErrBadFrame
	}
	if d, ok := tok.(json.Delim); !ok || d != '{' {
		return nil, ErrBadFrame
	}
	v, err := decodeObjectBody(dec)
	if err != nil {
		return nil, err
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return nil, ErrBadFrame
	}
	return v, nil
}

func decodeValue(dec *json.Decoder) (any, error) {
	tok, err := dec.Token()
	if err != nil {
		return nil, ErrBadFrame
	}
	switch t := tok.(type) {
	case json.Delim:
		switch t {
		case '{':
			return decodeObjectBody(dec)
		case '[':
			arr := []any{}
			for dec.More() {
				v, err := decodeValue(dec)
				if err != nil {
					return nil, err
				}
				arr = append(arr, v)
			}
			if _, err := dec.Token(); err != nil {
				return nil, ErrBadFrame
			}
			return arr, nil
		}
		return nil, ErrBadFrame
	default:
		return t, nil
	}
}

func decodeObjectBody(dec *json.Decoder) (map[string]any, error) {
	obj := map[string]any{}
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return nil, ErrBadFrame
		}
		key, ok := tok.(string)
		if !ok {
			return nil, ErrBadFrame
		}
		if _, dup := obj[key]; dup {
			return nil, ErrDuplicateKey
		}
		v, err := decodeValue(dec)
		if err != nil {
			return nil, err
		}
		obj[key] = v
	}
	if _, err := dec.Token(); err != nil { // closing '}'
		return nil, ErrBadFrame
	}
	return obj, nil
}

// encodeFrame marshals v and appends LF. Frames over MaxFrame are refused.
func encodeFrame(v any) ([]byte, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	b = append(b, '\n')
	if len(b) > MaxFrame {
		return nil, fmt.Errorf("outgoing frame: %w", ErrTooLarge)
	}
	return b, nil
}

func str(obj map[string]any, key string) (string, bool) {
	s, ok := obj[key].(string)
	return s, ok
}

func nonempty(obj map[string]any, key string) (string, bool) {
	s, ok := str(obj, key)
	return s, ok && s != ""
}

func reqID(obj map[string]any) (string, bool) {
	s, ok := str(obj, "id")
	return s, ok && validReqID(s)
}
