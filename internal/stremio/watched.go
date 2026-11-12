package stremio

import (
	"bytes"
	"compress/zlib"
	"encoding/base64"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// DecodeWatchedVideos decodes Stremio's watched-bitfield format:
// anchorVideoID:anchorLength:base64(zlib(bits)). Bits are indexed by the
// current Cinemeta video list, least-significant bit first within each byte.
func DecodeWatchedVideos(serialized string, videoIDs []string) (map[string]bool, error) {
	watched := make(map[string]bool, len(videoIDs))
	if serialized == "" {
		return watched, nil
	}

	lastColon := strings.LastIndex(serialized, ":")
	if lastColon < 0 {
		return nil, fmt.Errorf("invalid watched bitfield: missing encoded data separator")
	}
	encoded := serialized[lastColon+1:]
	remainder := serialized[:lastColon]
	lengthColon := strings.LastIndex(remainder, ":")
	if lengthColon < 0 {
		return nil, fmt.Errorf("invalid watched bitfield: missing anchor length separator")
	}
	anchorID := remainder[:lengthColon]
	anchorLength, err := strconv.Atoi(remainder[lengthColon+1:])
	if err != nil || anchorLength < 1 {
		return nil, fmt.Errorf("invalid watched bitfield anchor length %q", remainder[lengthColon+1:])
	}

	anchorIndex := -1
	for i, id := range videoIDs {
		if id == anchorID {
			anchorIndex = i
			break
		}
	}
	if anchorIndex == -1 {
		return nil, fmt.Errorf("watched bitfield anchor %q is absent from current metadata", anchorID)
	}

	compressed, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("decode watched bitfield base64: %w", err)
	}
	zlibReader, err := zlib.NewReader(bytes.NewReader(compressed))
	if err != nil {
		return nil, fmt.Errorf("open watched bitfield zlib stream: %w", err)
	}
	packed, readErr := io.ReadAll(io.LimitReader(zlibReader, 16<<20))
	closeErr := zlibReader.Close()
	if readErr != nil {
		return nil, fmt.Errorf("decompress watched bitfield: %w", readErr)
	}
	if closeErr != nil {
		return nil, fmt.Errorf("close watched bitfield zlib stream: %w", closeErr)
	}

	offset := (anchorLength - 1) - anchorIndex
	for currentIndex, videoID := range videoIDs {
		previousIndex := currentIndex + offset
		if previousIndex < 0 || previousIndex >= anchorLength {
			continue
		}
		byteIndex := previousIndex / 8
		if byteIndex >= len(packed) {
			continue
		}
		bit := uint(previousIndex % 8)
		watched[videoID] = packed[byteIndex]&(1<<bit) != 0
	}
	return watched, nil
}
