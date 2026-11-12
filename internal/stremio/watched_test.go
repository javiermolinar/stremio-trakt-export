package stremio

import (
	"bytes"
	"compress/zlib"
	"encoding/base64"
	"fmt"
	"reflect"
	"testing"
)

func encodeWatched(t *testing.T, anchorID string, values []bool) string {
	t.Helper()
	packed := make([]byte, (len(values)+7)/8)
	for index, value := range values {
		if value {
			packed[index/8] |= 1 << uint(index%8)
		}
	}
	var compressed bytes.Buffer
	writer := zlib.NewWriter(&compressed)
	if _, err := writer.Write(packed); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return fmt.Sprintf("%s:%d:%s", anchorID, len(values), base64.StdEncoding.EncodeToString(compressed.Bytes()))
}

func TestDecodeWatchedVideos(t *testing.T) {
	videoIDs := []string{"tt1:1:1", "tt1:1:2", "tt1:1:3", "tt1:1:4"}
	serialized := encodeWatched(t, "tt1:1:4", []bool{true, false, true, true})

	got, err := DecodeWatchedVideos(serialized, videoIDs)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{"tt1:1:1": true, "tt1:1:2": false, "tt1:1:3": true, "tt1:1:4": true}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("DecodeWatchedVideos() = %#v, want %#v", got, want)
	}
}

func TestDecodeWatchedVideosShiftsInsertedEpisode(t *testing.T) {
	// The old list did not contain the new first episode. Its anchor S01E04 is
	// now at index 3 instead of index 2, so every old bit shifts right by one.
	serialized := encodeWatched(t, "tt1:1:4", []bool{true, false, true})
	currentIDs := []string{"tt1:1:1", "tt1:1:2", "tt1:1:3", "tt1:1:4"}

	got, err := DecodeWatchedVideos(serialized, currentIDs)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{"tt1:1:2": true, "tt1:1:3": false, "tt1:1:4": true}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("DecodeWatchedVideos() = %#v, want %#v", got, want)
	}
}

func TestDecodeWatchedVideosRejectsMissingAnchor(t *testing.T) {
	serialized := encodeWatched(t, "removed:1:1", []bool{true})
	if _, err := DecodeWatchedVideos(serialized, []string{"tt1:1:1"}); err == nil {
		t.Fatal("expected missing-anchor error")
	}
}

func TestDecodeWatchedVideosEmpty(t *testing.T) {
	got, err := DecodeWatchedVideos("", []string{"tt1:1:1"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("expected empty map, got %#v", got)
	}
}
