package dashboard

import "context"

// Broker is a fan-out pub/sub broker for SSE events.
// A single goroutine (Run) owns the subscriber map to avoid races.
type Broker struct {
	subscribe   chan chan []byte
	unsubscribe chan chan []byte
	publish     chan []byte
}

// NewBroker creates a new Broker ready for use.
func NewBroker() *Broker {
	return &Broker{
		subscribe:   make(chan chan []byte),
		unsubscribe: make(chan chan []byte),
		publish:     make(chan []byte),
	}
}

// Run processes subscribe/unsubscribe/publish until ctx is cancelled.
// Must be called in its own goroutine.
func (b *Broker) Run(ctx context.Context) {
	subscribers := make(map[chan []byte]struct{})
	for {
		select {
		case <-ctx.Done():
			return
		case ch := <-b.subscribe:
			subscribers[ch] = struct{}{}
		case ch := <-b.unsubscribe:
			delete(subscribers, ch)
		case data := <-b.publish:
			for ch := range subscribers {
				select {
				case ch <- data:
				default:
					// drop if subscriber channel is full
				}
			}
		}
	}
}

// Subscribe returns a buffered channel that will receive published payloads.
func (b *Broker) Subscribe() chan []byte {
	ch := make(chan []byte, 16)
	b.subscribe <- ch
	return ch
}

// Unsubscribe removes the channel from the broker.
func (b *Broker) Unsubscribe(ch chan []byte) {
	b.unsubscribe <- ch
}

// Publish sends data to all current subscribers (non-blocking per subscriber).
func (b *Broker) Publish(data []byte) {
	b.publish <- data
}
