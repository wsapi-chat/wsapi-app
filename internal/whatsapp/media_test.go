package whatsapp

import (
	"context"
	"errors"
	"io"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

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

// blockingDownloader holds Download until release is closed, so tests can
// stage concurrent requests deterministically.
type blockingDownloader struct {
	inFlight int32
	peak     int32
	release  chan struct{}
}

func (b *blockingDownloader) Download(ctx context.Context, _ whatsmeow.DownloadableMessage) ([]byte, error) {
	cur := atomic.AddInt32(&b.inFlight, 1)
	for {
		peak := atomic.LoadInt32(&b.peak)
		if cur <= peak || atomic.CompareAndSwapInt32(&b.peak, peak, cur) {
			break
		}
	}
	defer atomic.AddInt32(&b.inFlight, -1)

	select {
	case <-b.release:
		return []byte("ok"), nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// TestDownloadByID_ConcurrencySlotsCap verifies the semaphore caps the number
// of simultaneous Download calls. With slots=2 and 5 concurrent requests, peak
// in-flight must stay at 2.
func TestDownloadByID_ConcurrencySlotsCap(t *testing.T) {
	dl := &blockingDownloader{release: make(chan struct{})}
	svc := &MediaService{
		dl:    dl,
		slots: make(chan struct{}, 2),
	}

	const requests = 5
	var wg sync.WaitGroup
	wg.Add(requests)
	for i := 0; i < requests; i++ {
		go func() {
			defer wg.Done()
			result, err := svc.DownloadByID(context.Background(), encodeTestMediaID(8))
			if err != nil {
				t.Errorf("download failed: %v", err)
				return
			}
			_ = result.Body.Close()
		}()
	}

	// Give goroutines time to all reach the semaphore + downloader.
	// Two should be inside Download; three should be parked on the slot acquire.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if atomic.LoadInt32(&dl.inFlight) == 2 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	if got := atomic.LoadInt32(&dl.inFlight); got != 2 {
		close(dl.release)
		wg.Wait()
		t.Fatalf("expected 2 concurrent downloads, got %d", got)
	}

	close(dl.release)
	wg.Wait()

	if peak := atomic.LoadInt32(&dl.peak); peak > 2 {
		t.Fatalf("peak concurrency %d exceeded slot cap of 2", peak)
	}
}

// TestDownloadByID_ReleasesSlotOnError verifies a failing Download still
// returns its slot, so the cap doesn't leak under error conditions.
func TestDownloadByID_ReleasesSlotOnError(t *testing.T) {
	svc := &MediaService{
		dl:    &mockDownloader{err: errors.New("network blew up")},
		slots: make(chan struct{}, 1),
	}

	for i := 0; i < 5; i++ {
		_, err := svc.DownloadByID(context.Background(), encodeTestMediaID(8))
		if err == nil {
			t.Fatalf("iteration %d: expected error, got nil", i)
		}
	}

	// If a slot leaked, the channel would be full and this acquire would block.
	select {
	case svc.slots <- struct{}{}:
		<-svc.slots
	case <-time.After(100 * time.Millisecond):
		t.Fatal("slot leaked after failed download")
	}
}

// TestDownloadByID_NilSlotsUnlimited verifies that a nil slots channel imposes
// no concurrency cap (the unlimited / 0-config case).
func TestDownloadByID_NilSlotsUnlimited(t *testing.T) {
	dl := &blockingDownloader{release: make(chan struct{})}
	svc := &MediaService{
		dl:    dl,
		slots: nil,
	}

	const requests = 10
	var wg sync.WaitGroup
	wg.Add(requests)
	for i := 0; i < requests; i++ {
		go func() {
			defer wg.Done()
			result, err := svc.DownloadByID(context.Background(), encodeTestMediaID(8))
			if err != nil {
				t.Errorf("download failed: %v", err)
				return
			}
			_ = result.Body.Close()
		}()
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if atomic.LoadInt32(&dl.inFlight) == requests {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	if got := atomic.LoadInt32(&dl.inFlight); got != requests {
		close(dl.release)
		wg.Wait()
		t.Fatalf("expected %d concurrent downloads with nil slots, got %d", requests, got)
	}

	close(dl.release)
	wg.Wait()
}

// TestDownloadByID_TimeoutFires verifies a stuck Download is bounded by the
// service's downloadTimeout — the slot does not stay held forever.
func TestDownloadByID_TimeoutFires(t *testing.T) {
	dl := &blockingDownloader{release: make(chan struct{})}
	defer close(dl.release) // unblock any stragglers on test exit

	svc := &MediaService{
		dl:              dl,
		slots:           make(chan struct{}, 1),
		downloadTimeout: 50 * time.Millisecond,
	}

	start := time.Now()
	_, err := svc.DownloadByID(context.Background(), encodeTestMediaID(8))
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
	if elapsed > time.Second {
		t.Fatalf("download took %v, expected ~50ms timeout", elapsed)
	}

	// Slot should have been released by the deferred receive.
	select {
	case svc.slots <- struct{}{}:
		<-svc.slots
	case <-time.After(100 * time.Millisecond):
		t.Fatal("slot leaked after timeout")
	}
}

// TestDownloadByID_AcquireRespectsContextCancellation verifies that callers
// blocked waiting for a slot unblock when the request context is cancelled,
// instead of holding the goroutine indefinitely.
func TestDownloadByID_AcquireRespectsContextCancellation(t *testing.T) {
	dl := &blockingDownloader{release: make(chan struct{})}
	defer close(dl.release)

	svc := &MediaService{
		dl:    dl,
		slots: make(chan struct{}, 1),
	}

	// Fill the only slot with a long-running download.
	go func() {
		_, _ = svc.DownloadByID(context.Background(), encodeTestMediaID(8))
	}()

	// Wait until the slot is taken.
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) && atomic.LoadInt32(&dl.inFlight) == 0 {
		time.Sleep(5 * time.Millisecond)
	}
	if atomic.LoadInt32(&dl.inFlight) == 0 {
		t.Fatal("first download did not reach Download")
	}

	// Second caller comes in with a context that's already cancelled.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	start := time.Now()
	_, err := svc.DownloadByID(ctx, encodeTestMediaID(8))
	elapsed := time.Since(start)

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got: %v", err)
	}
	if elapsed > 200*time.Millisecond {
		t.Fatalf("cancelled caller waited %v, should return immediately", elapsed)
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
