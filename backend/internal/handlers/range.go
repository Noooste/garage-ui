package handlers

import (
	"strconv"
	"strings"
)

// byteRange is a resolved, inclusive byte range within an object.
type byteRange struct {
	start int64
	end   int64
}

// parseRangeHeader resolves a Range request header against the object size.
// It supports a single "bytes=" range in its three forms: start-end, start-,
// and -suffix. A nil result with unsatisfiable false means serve the full
// object with 200; absent, malformed, and multi-range headers all land there,
// which RFC 9110 permits. unsatisfiable true means respond 416.
func parseRangeHeader(header string, size int64) (rng *byteRange, unsatisfiable bool) {
	spec, ok := strings.CutPrefix(header, "bytes=")
	if !ok {
		return nil, false
	}
	spec = strings.TrimSpace(spec)
	if strings.Contains(spec, ",") {
		return nil, false
	}
	startStr, endStr, ok := strings.Cut(spec, "-")
	if !ok {
		return nil, false
	}
	startStr = strings.TrimSpace(startStr)
	endStr = strings.TrimSpace(endStr)

	// Suffix form "-n" asks for the last n bytes.
	if startStr == "" {
		n, err := strconv.ParseInt(endStr, 10, 64)
		if err != nil {
			return nil, false
		}
		if n <= 0 || size == 0 {
			return nil, true
		}
		if n > size {
			n = size
		}
		return &byteRange{start: size - n, end: size - 1}, false
	}

	start, err := strconv.ParseInt(startStr, 10, 64)
	if err != nil || start < 0 {
		return nil, false
	}
	if start >= size {
		return nil, true
	}
	if endStr == "" {
		return &byteRange{start: start, end: size - 1}, false
	}
	end, err := strconv.ParseInt(endStr, 10, 64)
	if err != nil || end < start {
		return nil, false
	}
	if end >= size {
		end = size - 1
	}
	return &byteRange{start: start, end: end}, false
}
