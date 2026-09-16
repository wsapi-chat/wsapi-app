package whatsapp

import (
	"testing"
	"time"

	"go.mau.fi/whatsmeow"
)

// whatsmeow's GetQRChannel hands back a channel with this much buffer. The
// regression below only means something at the real size.
const qrChannelBuffer = 8

func TestDrainQRChannelForwardsFirstItem(t *testing.T) {
	src := make(chan whatsmeow.QRChannelItem, qrChannelBuffer)
	out := drainQRChannel(src, time.Minute)

	src <- whatsmeow.QRChannelItem{Event: whatsmeow.QRChannelEventCode, Code: "first"}
	src <- whatsmeow.QRChannelItem{Event: whatsmeow.QRChannelEventCode, Code: "second"}

	select {
	case got := <-out:
		if got.Code != "first" {
			t.Fatalf("forwarded code = %q, want %q", got.Code, "first")
		}
	case <-time.After(time.Second):
		t.Fatal("no item forwarded")
	}
}

// The regression. whatsmeow's qrChannel handler runs inside dispatchEvent,
// which holds a read lock on the client's event-handler list, and sends its
// terminal items (pair success, passkey request) with a plain blocking send.
// Before this fix the caller read one code and walked away, the buffer filled
// with rotating codes, and that send parked forever with the read lock held —
// which starved the write lock RemoveEventHandler wants and froze every
// subsequent dispatchEvent on the client.
//
// So what is asserted here is not that items arrive anywhere. It is that a
// sender never blocks, even long after the caller has stopped caring.
func TestDrainQRChannelKeepsSendersUnblocked(t *testing.T) {
	src := make(chan whatsmeow.QRChannelItem, qrChannelBuffer)
	out := drainQRChannel(src, time.Minute)

	// The caller takes its one code and returns, as the HTTP handler does.
	src <- whatsmeow.QRChannelItem{Event: whatsmeow.QRChannelEventCode, Code: "code-0"}
	<-out

	// Far more codes than the buffer holds, as a slow pairing produces.
	for i := 0; i < qrChannelBuffer*3; i++ {
		select {
		case src <- whatsmeow.QRChannelItem{Event: whatsmeow.QRChannelEventCode}:
		case <-time.After(time.Second):
			t.Fatalf("emitter blocked sending code %d of %d", i, qrChannelBuffer*3)
		}
	}

	// The send that used to deadlock: a terminal event arriving on a channel
	// whose buffer had filled up.
	done := make(chan struct{})
	go func() {
		defer close(done)
		src <- whatsmeow.QRChannelSuccess
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("terminal send blocked: this is the deadlock that froze node handling")
	}
}

func TestDrainQRChannelStopsWhenSourceCloses(t *testing.T) {
	src := make(chan whatsmeow.QRChannelItem, qrChannelBuffer)
	out := drainQRChannel(src, time.Minute)

	close(src)

	select {
	case _, ok := <-out:
		if ok {
			t.Fatal("got an item from an empty closed source")
		}
	case <-time.After(time.Second):
		t.Fatal("drain did not stop after the source closed")
	}
}

// whatsmeow's QR emitter returns without closing the channel when the client
// disconnects as expected, which is what the next poll of the QR endpoint
// causes. Without the deadline the drain goroutine would outlive every such
// poll. The deadline is only a backstop against piling up goroutines — see
// qrSessionMaxLifetime for why it has to be far longer than a pairing.
func TestDrainQRChannelStopsAtDeadlineWhenSourceNeverCloses(t *testing.T) {
	src := make(chan whatsmeow.QRChannelItem, qrChannelBuffer)
	out := drainQRChannel(src, 20*time.Millisecond)

	select {
	case _, ok := <-out:
		if ok {
			t.Fatal("got an item from a source that never sent one")
		}
	case <-time.After(time.Second):
		t.Fatal("drain outlived its deadline on a source that never closes")
	}
}
