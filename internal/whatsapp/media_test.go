package whatsapp

import (
	"context"
	"errors"
	"io"
	"runtime"
	"testing"

	"go.mau.fi/whatsmeow"

	"github.com/wsapi-chat/wsapi-app/internal/event"
)

// mockDownloader returns a fixed []byte for any Download call.
type mockDownloader struct {
	data []byte
	err  error
}

func (m *mockDownloader) Download(_ context.Context, _ whatsmeow.DownloadableMessage) ([]byte, error) {
	if m.err != nil {
		return nil, m.err
	}
	// Return a copy so the original stays alive for multiple calls.
	out := make([]byte, len(m.data))
	copy(out, m.data)
	return out, nil
}

// encodeTestMediaID creates a media ID for testing with the given file length.
func encodeTestMediaID(fileLength uint64) string {
	info := event.MediaDownloadInfo{
		URL:           "https://example.com/media",
		DirectPath:    "/v/t62.1234",
		MediaKey:      make([]byte, 32),
		FileSHA256:    make([]byte, 32),
		FileEncSHA256: make([]byte, 32),
		MimeType:      "video/mp4",
		MediaType:     "video",
		FileLength:    fileLength,
	}
	id, err := event.EncodeMediaID(info)
	if err != nil {
		panic(err)
	}
	return id
}

func TestDownloadByID_SizeGate(t *testing.T) {
	svc := &MediaService{
		dl:          &mockDownloader{data: []byte("small")},
		maxFileSize: 1024,
	}

	mediaID := encodeTestMediaID(2048)
	_, err := svc.DownloadByID(context.Background(), mediaID)
	if err == nil {
		t.Fatal("expected error for oversized file, got nil")
	}
	if !errors.Is(err, ErrTooLarge) {
		t.Fatalf("expected ErrTooLarge, got: %v", err)
	}
}

func TestDownloadByID_PostDownloadSizeCheck(t *testing.T) {
	// Simulate a spoofed media ID: FileLength says 512 but actual payload is 2048.
	svc := &MediaService{
		dl:          &mockDownloader{data: make([]byte, 2048)},
		maxFileSize: 1024,
	}

	mediaID := encodeTestMediaID(512) // lies about size
	_, err := svc.DownloadByID(context.Background(), mediaID)
	if err == nil {
		t.Fatal("expected error for oversized download, got nil")
	}
	if !errors.Is(err, ErrTooLarge) {
		t.Fatalf("expected ErrTooLarge, got: %v", err)
	}
}

func TestDownloadByID_SizeGateAllows(t *testing.T) {
	payload := make([]byte, 512)
	svc := &MediaService{
		dl:          &mockDownloader{data: payload},
		maxFileSize: 1024,
	}

	mediaID := encodeTestMediaID(512)
	result, err := svc.DownloadByID(context.Background(), mediaID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = result.Body.Close() }()

	if result.Size != 512 {
		t.Fatalf("expected size 512, got %d", result.Size)
	}
	got, err := io.ReadAll(result.Body)
	if err != nil {
		t.Fatalf("failed to read body: %v", err)
	}
	if len(got) != 512 {
		t.Fatalf("expected 512 bytes from body, got %d", len(got))
	}
}

func TestDownloadByID_TempFileCleanup(t *testing.T) {
	payload := make([]byte, 1024)
	svc := &MediaService{
		dl:          &mockDownloader{data: payload},
		maxFileSize: 0, // no limit
	}

	mediaID := encodeTestMediaID(1024)
	result, err := svc.DownloadByID(context.Background(), mediaID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The temp file was unlinked via os.Remove right after creation.
	// Reading should still work via the open fd.
	got, err := io.ReadAll(result.Body)
	if err != nil {
		t.Fatalf("failed to read body: %v", err)
	}
	if len(got) != 1024 {
		t.Fatalf("expected 1024 bytes, got %d", len(got))
	}
	_ = result.Body.Close()
}

// TestDownloadByID_MemoryBounded verifies that after the download completes
// and we start streaming from the temp file, the large buffer is GC-eligible.
// We download a 50MB payload, then verify heap doesn't retain it during streaming.
func TestDownloadByID_MemoryBounded(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping memory stress test in short mode")
	}

	const payloadSize = 50 << 20 // 50MB
	payload := make([]byte, payloadSize)
	// Fill with non-zero data to ensure it's actually allocated.
	for i := range payload {
		payload[i] = byte(i % 256)
	}

	svc := &MediaService{
		dl:          &mockDownloader{data: payload},
		maxFileSize: 0, // no limit for this test
	}

	// Free our copy of the payload so only the service holds one.
	//nolint:ineffassign,wastedassign
	payload = nil
	runtime.GC()

	mediaID := encodeTestMediaID(payloadSize)
	result, err := svc.DownloadByID(context.Background(), mediaID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = result.Body.Close() }()

	// At this point the []byte from Download has been written to temp file
	// and nil'd. Force GC and measure heap.
	runtime.GC()

	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)

	// HeapInuse should be well under 50MB since the large buffer was freed.
	// Allow 30MB for Go runtime overhead, test infrastructure, etc.
	const maxHeap = 30 << 20
	if mem.HeapInuse > maxHeap {
		t.Fatalf("heap too large after download: %d MB (limit %d MB); large buffer was not freed",
			mem.HeapInuse>>20, maxHeap>>20)
	}

	// Verify the content is still readable from the temp file.
	n, err := io.Copy(io.Discard, result.Body)
	if err != nil {
		t.Fatalf("failed to stream body: %v", err)
	}
	if n != payloadSize {
		t.Fatalf("expected %d bytes streamed, got %d", payloadSize, n)
	}
}

// TestDownloadByID_NoLeakAfterRepeatedDownloads verifies that memory stays
// flat after multiple large downloads. If the temp file fd or the []byte
// leaked, heap would grow with each iteration.
func TestDownloadByID_NoLeakAfterRepeatedDownloads(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping memory leak test in short mode")
	}

	const payloadSize = 20 << 20 // 20MB per download
	const iterations = 10        // 200MB total if leaking

	payload := make([]byte, payloadSize)
	for i := range payload {
		payload[i] = byte(i % 256)
	}

	svc := &MediaService{
		dl:          &mockDownloader{data: payload},
		maxFileSize: 0,
	}
	//nolint:ineffassign,wastedassign
	payload = nil

	mediaID := encodeTestMediaID(payloadSize)

	// Warm up: one download to stabilize runtime allocations.
	result, err := svc.DownloadByID(context.Background(), mediaID)
	if err != nil {
		t.Fatalf("warmup failed: %v", err)
	}
	_, _ = io.Copy(io.Discard, result.Body)
	_ = result.Body.Close()
	runtime.GC()

	var baseline runtime.MemStats
	runtime.ReadMemStats(&baseline)

	for i := 0; i < iterations; i++ {
		result, err := svc.DownloadByID(context.Background(), mediaID)
		if err != nil {
			t.Fatalf("iteration %d: unexpected error: %v", i, err)
		}
		// Simulate the handler: stream and close.
		n, err := io.Copy(io.Discard, result.Body)
		if err != nil {
			t.Fatalf("iteration %d: stream error: %v", i, err)
		}
		if n != payloadSize {
			t.Fatalf("iteration %d: expected %d bytes, got %d", i, payloadSize, n)
		}
		_ = result.Body.Close()
	}

	runtime.GC()
	var after runtime.MemStats
	runtime.ReadMemStats(&after)

	// After 10 × 20MB downloads (200MB total), heap growth should be minimal.
	// Allow 20MB growth for test overhead — if the buffer leaked, it would be 200MB+.
	growth := int64(after.HeapInuse) - int64(baseline.HeapInuse)
	const maxGrowth = 20 << 20
	if growth > maxGrowth {
		t.Fatalf("heap grew by %d MB after %d downloads; likely a leak (baseline %d MB, after %d MB)",
			growth>>20, iterations, baseline.HeapInuse>>20, after.HeapInuse>>20)
	}
	t.Logf("heap growth after %d × %dMB downloads: %d KB (OK)",
		iterations, payloadSize>>20, growth>>10)
}
