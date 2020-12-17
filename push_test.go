package push

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func BenchmarkMiddleware(b *testing.B) {
	m := pubsubPushMessage{
		Message: pubsubMessage{
			Attributes: map[string]string{
				"Content-Type": "text/plain",
			},
			Data:      []byte("hello world"),
			MessageID: "abc123",
		},
		Subscription: "some/subscription/pubsub",
	}

	data, err := json.Marshal(&m)
	require.NoError(b, err)

	fmt.Println(string(data))

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	})

	mw := New(handler)
	n := &nopWriter{}

	b.ResetTimer()

	b.RunParallel(func(p *testing.PB) {
		rdr := bytes.NewReader(data)

		r := httptest.NewRequest(http.MethodPost, "http://localhost:8080", rdr)
		r.Header.Set("Content-Type", "application/json")

		for p.Next() {
			rdr.Seek(0, 0)
			mw.ServeHTTP(n, r)
		}
	})
}

type nopWriter struct{}

func (n *nopWriter) Header() http.Header {
	return make(http.Header)
}

func (n *nopWriter) Write(in []byte) (int, error) {
	return len(in), nil
}

func (n *nopWriter) WriteHeader(statusCode int) {}
