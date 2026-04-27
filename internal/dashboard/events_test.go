package dashboard

import (
	"context"
	"testing"
	"time"
)

func startBroker(t *testing.T) (*Broker, context.CancelFunc) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	b := NewBroker()
	go b.Run(ctx)
	return b, cancel
}

// TestPublishDeliversToAllSubscribers verifies that Publish sends to all subscribers.
func TestPublishDeliversToAllSubscribers(t *testing.T) {
	b, cancel := startBroker(t)
	defer cancel()

	ch1 := b.Subscribe()
	ch2 := b.Subscribe()

	msg := []byte("hello")
	b.Publish(msg)

	for i, ch := range []chan []byte{ch1, ch2} {
		select {
		case got := <-ch:
			if string(got) != string(msg) {
				t.Errorf("subscriber %d: got %q, want %q", i, got, msg)
			}
		case <-time.After(time.Second):
			t.Errorf("subscriber %d: timed out waiting for message", i)
		}
	}
}

// TestUnsubscribeStopsDelivery verifies that after Unsubscribe, the channel no longer receives messages.
func TestUnsubscribeStopsDelivery(t *testing.T) {
	b, cancel := startBroker(t)
	defer cancel()

	ch := b.Subscribe()
	b.Unsubscribe(ch)

	// Give the broker time to process the unsubscribe before publishing.
	// We use a second subscriber as a synchronization point.
	sync := b.Subscribe()

	b.Publish([]byte("after unsubscribe"))

	// Wait for the sync subscriber to receive, meaning broker processed publish.
	select {
	case <-sync:
	case <-time.After(time.Second):
		t.Fatal("sync subscriber timed out")
	}

	// The unsubscribed channel should have nothing.
	select {
	case got := <-ch:
		t.Errorf("unsubscribed channel unexpectedly received %q", got)
	default:
		// expected: no message
	}
}

// TestFullSubscriberDoesNotBlockPublish verifies that a full subscriber channel does not block Publish.
func TestFullSubscriberDoesNotBlockPublish(t *testing.T) {
	b, cancel := startBroker(t)
	defer cancel()

	ch := b.Subscribe()

	// Fill the subscriber channel to capacity (buffer size is 16).
	for i := 0; i < 16; i++ {
		b.Publish([]byte("fill"))
	}

	// This publish should not block even though ch is full.
	done := make(chan struct{})
	go func() {
		b.Publish([]byte("overflow"))
		close(done)
	}()

	select {
	case <-done:
		// expected: Publish returned without blocking
	case <-time.After(time.Second):
		t.Error("Publish blocked on a full subscriber channel")
	}

	_ = ch // prevent gc
}
