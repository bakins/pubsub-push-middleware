package push

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
	"sync"

	jsoniter "github.com/json-iterator/go"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

// DefaultDetect checks if the body contains subscription and message json fields
func DefaultDetect(r *http.Request, body []byte) bool {
	return bytes.Contains(body, []byte(`"subscription":`)) && bytes.Contains(body, []byte(`"message":`))
}

// Options define options when creating middleware.
type Options struct {
	detect   DetectFunc
	prefixes []string
}

// Option is a single option when creating middleware.
type Option func(*Options)

// DetectFunc detects if the request is a pubsub push request.
// Request will have already been tested to see if body is json and method is POST.
type DetectFunc func(r *http.Request, body []byte) bool

// WithDetector sets the function to call to detect if the request body is a pubsub push request.
// If not set, DefaultDetect is used.
func WithDetector(detect DetectFunc) Option {
	return func(o *Options) {
		o.detect = detect
	}
}

// WithPrefixes sets the middleware to only run for the given prefixes. By default, all requests
// are checked.
// Prefixes are checked using simple string prefix checks.
func WithPrefixes(prefixes []string) Option {
	return func(o *Options) {
		cp := make([]string, len(prefixes))
		copy(cp, prefixes)
		o.prefixes = cp
	}
}

// New creates HTTP middleware.
func New(next http.Handler, options ...Option) http.Handler {
	o := Options{}

	for _, opt := range options {
		opt(&o)
	}

	if o.detect == nil {
		o.detect = DefaultDetect
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") || r.Method != http.MethodPost {
			next.ServeHTTP(w, r)
			return
		}

		if len(o.prefixes) > 0 {
			var found bool
			for _, p := range o.prefixes {
				if strings.HasPrefix(r.URL.Path, p) {
					found = true
					break
				}
			}
			if !found {
				next.ServeHTTP(w, r)
				return
			}
		}

		// if you want to limit request body, then use https://golang.org/pkg/net/http/#MaxBytesReader
		// in a middleware
		buff := bufferPool.Get().(*bytes.Buffer)
		defer bufferPool.Put(buff)

		buff.Reset()

		if r.ContentLength > 0 {
			buff.Grow(int(r.ContentLength))
		}

		if _, err := io.Copy(buff, r.Body); err != nil {
			http.Error(w, fmt.Sprintf("failed to read request body: %v", err), http.StatusBadRequest)
			return
		}

		if !o.detect(r, buff.Bytes()) {
			r.Body = ioutil.NopCloser(buff)
			next.ServeHTTP(w, r)
			return
		}

		var p pubsubPushMessage
		if err := json.Unmarshal(buff.Bytes(), &p); err != nil {
			http.Error(w, fmt.Sprintf("invalid pubsub push message: %v", err), http.StatusBadRequest)
			return
		}

		r = r.Clone(r.Context())

		for k, v := range p.Message.Attributes {
			// allow setting content type for message body. default is json (which is the incoming header value)
			if strings.ToLower(k) == "content-type" {
				r.Header.Set("Content-Type", v)
			}
			r.Header.Set("X-Pubsub-"+k, v)
		}

		if id := p.Message.MessageID; id != "" {
			r.Header.Set("X-Pubsub-Message-Id", id)
		}

		if sub := p.Subscription; sub != "" {
			r.Header.Set("X-Pubsub-Subscription", sub)
		}

		r.ContentLength = int64(len(p.Message.Data))
		r.Header.Del("Content-Length")
		r.Body = ioutil.NopCloser(bytes.NewReader(p.Message.Data))

		next.ServeHTTP(w, r)
	})
}

// see https://cloud.google.com/pubsub/docs/push#receiving_messages
type pubsubPushMessage struct {
	Message      pubsubMessage `json:"message"`
	Subscription string        `json:"subscription"`
}

type pubsubMessage struct {
	Attributes map[string]string `json:"attributes,omitempty"`
	Data       []byte            `json:"data"`
	MessageID  string            `json:"message_id"`
}

var bufferPool = sync.Pool{
	New: func() interface{} {
		return &bytes.Buffer{}
	},
}
