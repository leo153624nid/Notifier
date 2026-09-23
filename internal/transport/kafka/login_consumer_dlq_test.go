package kafka

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	authevents "contracts/events/auth/v1"

	"github.com/segmentio/kafka-go"

	"uuid"
)

// mockWriter — тестовая замена *kafka.Writer: пишет в DLQ в память вместо
// сети и умеет симулировать ошибку записи (брокер недоступен).
type mockWriter struct {
	writeErr error
	messages []kafka.Message
	mu       sync.Mutex
	closed   bool
}

func (w *mockWriter) WriteMessages(_ context.Context, msgs ...kafka.Message) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.writeErr != nil {
		return w.writeErr
	}
	w.messages = append(w.messages, msgs...)
	return nil
}

func (w *mockWriter) Close() error {
	w.closed = true
	return nil
}

func (w *mockWriter) messagesSnapshot() []kafka.Message {
	w.mu.Lock()
	defer w.mu.Unlock()

	out := make([]kafka.Message, len(w.messages))
	copy(out, w.messages)
	return out
}

// mockReader отдаёт сообщения из заранее заданной очереди по одному на
// каждый FetchMessage; когда очередь пуста, блокируется на ctx.Done(), как
// это делал бы настоящий *kafka.Reader, ожидая новых сообщений от брокера.
// Так Run() можно останавливать штатно — отменой контекста, — без реального
// брокера.
type mockReader struct {
	queue     []kafka.Message
	committed []kafka.Message
	mu        sync.Mutex
	closed    bool
}

func (r *mockReader) FetchMessage(ctx context.Context) (kafka.Message, error) {
	r.mu.Lock()
	if len(r.queue) > 0 {
		msg := r.queue[0]
		r.queue = r.queue[1:]
		r.mu.Unlock()
		return msg, nil
	}
	r.mu.Unlock()

	<-ctx.Done()
	return kafka.Message{}, ctx.Err()
}

func (r *mockReader) CommitMessages(_ context.Context, msgs ...kafka.Message) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.committed = append(r.committed, msgs...)
	return nil
}

func (r *mockReader) Close() error {
	r.closed = true
	return nil
}

func (r *mockReader) committedSnapshot() []kafka.Message {
	r.mu.Lock()
	defer r.mu.Unlock()

	out := make([]kafka.Message, len(r.committed))
	copy(out, r.committed)
	return out
}

// newDLQTestConsumer — как newTestConsumer в login_consumer_test.go, но с
// поддельными reader/writer вместо nil, чтобы тестировать sendToDLQ и Run().
func newDLQTestConsumer(t *testing.T) (*LoginConsumer, *mockReader, *mockWriter) {
	t.Helper()

	base, _ := newTestConsumer(t)
	reader := &mockReader{}
	writer := &mockWriter{}
	base.reader = reader
	base.dlqWriter = writer

	return base, reader, writer
}

func TestLoginConsumer_SendToDLQ(t *testing.T) {
	c, _, writer := newDLQTestConsumer(t)

	msg := kafka.Message{
		Topic: authevents.TopicUserLoggedIn,
		Key:   []byte("some-key"),
		Value: []byte("not json"),
	}
	cause := errors.New("boom")

	if err := c.sendToDLQ(context.Background(), msg, cause); err != nil {
		t.Fatalf("sendToDLQ() error: %s", err)
	}

	got := writer.messagesSnapshot()
	if len(got) != 1 {
		t.Fatalf("dlq messages = %d, want 1", len(got))
	}
	if string(got[0].Value) != string(msg.Value) {
		t.Errorf("dlq message value = %q, want %q", got[0].Value, msg.Value)
	}
	if string(got[0].Key) != string(msg.Key) {
		t.Errorf("dlq message key = %q, want %q", got[0].Key, msg.Key)
	}

	headers := map[string]string{}
	for _, h := range got[0].Headers {
		headers[h.Key] = string(h.Value)
	}
	if headers["error"] != cause.Error() {
		t.Errorf("dlq header error = %q, want %q", headers["error"], cause.Error())
	}
	if headers["source_topic"] != authevents.TopicUserLoggedIn {
		t.Errorf("dlq header source_topic = %q, want %q", headers["source_topic"], authevents.TopicUserLoggedIn)
	}
}

// runUntilQueueDrained запускает Run() в фоне, ждёт пока reader обработает
// всю очередь (queue опустеет и Run() уйдёт в блокирующий FetchMessage), а
// затем останавливает консьюмер отменой контекста — так же, как это делает
// graceful shutdown в cmd/notifier/main.go.
func runUntilQueueDrained(t *testing.T, c *LoginConsumer, r *mockReader) {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		c.Run(ctx)
		close(done)
	}()

	deadline := time.Now().Add(2 * time.Second)
	for {
		r.mu.Lock()
		empty := len(r.queue) == 0
		r.mu.Unlock()
		if empty {
			break
		}
		if time.Now().After(deadline) {
			cancel()
			t.Fatal("timed out waiting for mockReader queue to drain")
		}
		time.Sleep(time.Millisecond)
	}

	// Даём последней итерации цикла (commit/DLQ уже отработавшего сообщения)
	// завершиться, прежде чем отменять контекст.
	time.Sleep(10 * time.Millisecond)

	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run() did not return after context cancellation")
	}
}

func TestLoginConsumer_Run_DLQRouting(t *testing.T) {
	t.Run("unprocessable message goes to DLQ and offset is committed", func(t *testing.T) {
		c, reader, writer := newDLQTestConsumer(t)

		badMsg := kafka.Message{Topic: authevents.TopicUserLoggedIn, Value: []byte("not json")}
		reader.queue = []kafka.Message{badMsg}

		runUntilQueueDrained(t, c, reader)

		if got := writer.messagesSnapshot(); len(got) != 1 {
			t.Fatalf("dlq messages = %d, want 1 (poison pill must be routed to DLQ)", len(got))
		}
		if got := reader.committedSnapshot(); len(got) != 1 {
			t.Fatalf("committed messages = %d, want 1 (offset must advance once message is safely in DLQ)", len(got))
		}
	})

	t.Run("valid message is processed and committed without touching DLQ", func(t *testing.T) {
		c, reader, writer := newDLQTestConsumer(t)

		event := authevents.UserLoggedIn{EventID: uuid.New(), UserID: uuid.New(), Email: "user@mail.com", OccurredAt: time.Now()}
		reader.queue = []kafka.Message{{Topic: authevents.TopicUserLoggedIn, Value: marshalEvent(t, event)}}

		runUntilQueueDrained(t, c, reader)

		if got := writer.messagesSnapshot(); len(got) != 0 {
			t.Fatalf("dlq messages = %d, want 0 (successfully handled message must not reach DLQ)", len(got))
		}
		if got := reader.committedSnapshot(); len(got) != 1 {
			t.Fatalf("committed messages = %d, want 1", len(got))
		}
	})

	t.Run("DLQ unavailable: offset is not committed so the message is retried", func(t *testing.T) {
		c, reader, writer := newDLQTestConsumer(t)
		writer.writeErr = errors.New("broker unreachable")

		badMsg := kafka.Message{Topic: authevents.TopicUserLoggedIn, Value: []byte("not json")}
		reader.queue = []kafka.Message{badMsg}

		runUntilQueueDrained(t, c, reader)

		if got := reader.committedSnapshot(); len(got) != 0 {
			t.Fatalf("committed messages = %d, want 0 (must not commit when DLQ write fails, so the message is redelivered)", len(got))
		}
	})
}
