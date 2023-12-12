package download_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/semaphore"

	"github.com/replicate/pget/pkg/client"
	"github.com/replicate/pget/pkg/download"
)

var testFSes = []fstest.MapFS{
	{"hello.txt": {Data: []byte("0000000000000000")}},
	{"hello.txt": {Data: []byte("1111111111111111")}},
	{"hello.txt": {Data: []byte("2222222222222222")}},
	{"hello.txt": {Data: []byte("3333333333333333")}},
	{"hello.txt": {Data: []byte("4444444444444444")}},
	{"hello.txt": {Data: []byte("5555555555555555")}},
	{"hello.txt": {Data: []byte("6666666666666666")}},
	{"hello.txt": {Data: []byte("7777777777777777")}},
}

func makeConsistentHashingMode(opts download.Options) *download.ConsistentHashingMode {
	client := client.NewHTTPClient(opts.Client)
	fallbackMode := download.BufferMode{Options: opts, Client: client}

	return &download.ConsistentHashingMode{Client: client, Options: opts, FallbackStrategy: &fallbackMode}
}

type chTestCase struct {
	name           string
	concurrency    int
	sliceSize      int64
	minChunkSize   int64
	numCacheHosts  int
	expectedOutput string
}

var chTestCases = []chTestCase{
	{ // pre-computed demo that only some slices change as we add a new cache host
		name:           "1 host",
		concurrency:    8,
		sliceSize:      3,
		numCacheHosts:  1,
		minChunkSize:   1,
		expectedOutput: "0000000000000000",
	},
	{
		name:           "2 hosts",
		concurrency:    8,
		sliceSize:      3,
		numCacheHosts:  2,
		minChunkSize:   1,
		expectedOutput: "0001110000001110",
	},
	{
		name:           "3 hosts",
		concurrency:    8,
		sliceSize:      3,
		numCacheHosts:  3,
		minChunkSize:   1,
		expectedOutput: "0001110002221110",
	},
	{
		name:           "4 hosts",
		concurrency:    8,
		sliceSize:      3,
		numCacheHosts:  4,
		minChunkSize:   1,
		expectedOutput: "0001113333331110",
	},
	{
		name:           "5 hosts",
		concurrency:    8,
		sliceSize:      3,
		numCacheHosts:  5,
		minChunkSize:   1,
		expectedOutput: "0001114443331110",
	},
	{
		name:           "6 hosts",
		concurrency:    8,
		sliceSize:      3,
		numCacheHosts:  6,
		minChunkSize:   1,
		expectedOutput: "0001114443331115",
	},
	{
		name:           "7 hosts",
		concurrency:    8,
		sliceSize:      3,
		numCacheHosts:  7,
		minChunkSize:   1,
		expectedOutput: "0006664443336665",
	},
	{
		name:           "8 hosts",
		concurrency:    8,
		sliceSize:      3,
		numCacheHosts:  8,
		minChunkSize:   1,
		expectedOutput: "0006664443336667",
	},
	{
		name:           "test when fileSize % sliceSize == 0",
		concurrency:    8,
		sliceSize:      4,
		numCacheHosts:  8,
		minChunkSize:   1,
		expectedOutput: "0000666644443333",
	},
	{
		name:           "when minChunkSize == sliceSize",
		concurrency:    8,
		sliceSize:      3,
		numCacheHosts:  8,
		minChunkSize:   3,
		expectedOutput: "0006664443336667",
	},
	{
		name:           "test when concurrency > file size",
		concurrency:    24,
		sliceSize:      3,
		numCacheHosts:  8,
		minChunkSize:   3,
		expectedOutput: "0006664443336667",
	},
	{
		name:           "test when concurrency < number of slices",
		concurrency:    3,
		sliceSize:      3,
		numCacheHosts:  8,
		minChunkSize:   3,
		expectedOutput: "0006664443336667",
	},
	{
		name:           "test when minChunkSize == file size",
		concurrency:    4,
		sliceSize:      16,
		numCacheHosts:  8,
		minChunkSize:   16,
		expectedOutput: "0000000000000000",
	},
	{
		name:           "test when minChunkSize > file size",
		concurrency:    4,
		sliceSize:      24,
		numCacheHosts:  8,
		minChunkSize:   24,
		expectedOutput: "0000000000000000",
	},
	{
		name:           "if minChunkSize > sliceSize, sliceSize overrides it",
		concurrency:    8,
		sliceSize:      3,
		numCacheHosts:  8,
		minChunkSize:   24,
		expectedOutput: "0006664443336667",
	},
}

func TestConsistentHashing(t *testing.T) {
	hostnames := make([]string, len(testFSes))
	for i, fs := range testFSes {
		ts := httptest.NewServer(http.FileServer(http.FS(fs)))
		defer ts.Close()
		url, err := url.Parse(ts.URL)
		require.NoError(t, err)
		hostnames[i] = url.Host
	}

	for _, tc := range chTestCases {
		t.Run(tc.name, func(t *testing.T) {
			opts := download.Options{
				Client:         client.Options{},
				MaxConcurrency: tc.concurrency,
				MinChunkSize:   tc.minChunkSize,
				Semaphore:      semaphore.NewWeighted(int64(tc.concurrency)),
				CacheHosts:     hostnames[0:tc.numCacheHosts],
				DomainsToCache: []string{"test.replicate.delivery"},
				SliceSize:      tc.sliceSize,
			}

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			strategy := makeConsistentHashingMode(opts)

			reader, _, err := strategy.Fetch(ctx, "http://test.replicate.delivery/hello.txt")
			assert.NoError(t, err)
			bytes, err := io.ReadAll(reader)
			assert.NoError(t, err)

			assert.Equal(t, tc.expectedOutput, string(bytes))
		})
	}
}

func TestConsistentHashingHasFallback(t *testing.T) {
	server := httptest.NewServer(http.FileServer(http.FS(testFSes[0])))
	defer server.Close()

	opts := download.Options{
		Client:         client.Options{},
		MaxConcurrency: 8,
		MinChunkSize:   2,
		Semaphore:      semaphore.NewWeighted(8),
		CacheHosts:     []string{},
		DomainsToCache: []string{"fake.replicate.delivery"},
		SliceSize:      3,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	strategy := makeConsistentHashingMode(opts)

	urlString, err := url.JoinPath(server.URL, "hello.txt")
	assert.NoError(t, err)
	reader, _, err := strategy.Fetch(ctx, urlString)
	assert.NoError(t, err)
	bytes, err := io.ReadAll(reader)
	assert.NoError(t, err)

	assert.Equal(t, "0000000000000000", string(bytes))
}

type fallbackFailingHandler struct {
	responseStatus int
	responseFunc   func(w http.ResponseWriter, r *http.Request)
}

func (h fallbackFailingHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.responseFunc != nil {
		h.responseFunc(w, r)
	} else {
		w.WriteHeader(h.responseStatus)
	}
}

type testStrategy struct {
	fetchCalledCount     int
	doRequestCalledCount int
	mut                  sync.Mutex
}

func (s *testStrategy) Fetch(ctx context.Context, url string) (io.Reader, int64, error) {
	s.fetchCalledCount++
	return io.NopCloser(strings.NewReader("00")), -1, nil
}

func (s *testStrategy) DoRequest(ctx context.Context, start, end int64, url string) (*http.Response, error) {
	s.mut.Lock()
	s.doRequestCalledCount++
	s.mut.Unlock()
	resp := &http.Response{Body: io.NopCloser(strings.NewReader("00"))}
	return resp, nil
}

func TestConsistentHashingFileFallback(t *testing.T) {
	tc := []struct {
		name                 string
		responseStatus       int
		failureFunc          func(w http.ResponseWriter, r *http.Request)
		fetchCalledCount     int
		doRequestCalledCount int
		expectedError        error
	}{
		{
			name:                 "BadGateway",
			responseStatus:       http.StatusBadGateway,
			fetchCalledCount:     1,
			doRequestCalledCount: 0,
		},
		// "NotFound" should not trigger fall-back
		{
			name:                 "NotFound",
			responseStatus:       http.StatusNotFound,
			fetchCalledCount:     0,
			doRequestCalledCount: 0,
			expectedError:        download.ErrUnexpectedHTTPStatus,
		},
	}

	for _, tc := range tc {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(fallbackFailingHandler{responseStatus: tc.responseStatus, responseFunc: tc.failureFunc})
			defer server.Close()

			url, _ := url.Parse(server.URL)
			opts := download.Options{
				Client:         client.Options{},
				MaxConcurrency: 8,
				MinChunkSize:   2,
				Semaphore:      semaphore.NewWeighted(8),
				CacheHosts:     []string{url.Host},
				DomainsToCache: []string{"fake.replicate.delivery"},
				SliceSize:      3,
			}

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			strategy := makeConsistentHashingMode(opts)
			fallbackStrategy := &testStrategy{}
			strategy.FallbackStrategy = fallbackStrategy

			urlString := "http://fake.replicate.delivery/hello.txt"
			_, _, err := strategy.Fetch(ctx, urlString)
			if tc.expectedError != nil {
				assert.ErrorIs(t, err, tc.expectedError)
			}
			assert.Equal(t, tc.fetchCalledCount, fallbackStrategy.fetchCalledCount)
			assert.Equal(t, tc.doRequestCalledCount, fallbackStrategy.doRequestCalledCount)
		})
	}
}

func TestConsistentHashingChunkFallback(t *testing.T) {
	handlerFunc := func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Range") != "bytes=0-2" {
			w.WriteHeader(http.StatusBadGateway)
		} else {
			w.Header().Set("Content-Range", "bytes 0-2/4")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("000"))
		}
	}

	tc := []struct {
		name                 string
		responseStatus       int
		handlerFunc          func(w http.ResponseWriter, r *http.Request)
		fetchCalledCount     int
		doRequestCalledCount int
		expectedError        error
	}{
		{
			name:                 "fail-on-second-chunk",
			handlerFunc:          handlerFunc,
			fetchCalledCount:     0,
			doRequestCalledCount: 1,
		},
	}

	for _, tc := range tc {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(fallbackFailingHandler{responseStatus: tc.responseStatus, responseFunc: tc.handlerFunc})
			defer server.Close()

			url, _ := url.Parse(server.URL)
			opts := download.Options{
				Client:         client.Options{},
				MaxConcurrency: 8,
				MinChunkSize:   3,
				Semaphore:      semaphore.NewWeighted(8),
				CacheHosts:     []string{url.Host},
				DomainsToCache: []string{"fake.replicate.delivery"},
				SliceSize:      3,
			}

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			strategy := makeConsistentHashingMode(opts)
			fallbackStrategy := &testStrategy{}
			strategy.FallbackStrategy = fallbackStrategy

			urlString := "http://fake.replicate.delivery/hello.txt"
			_, _, err := strategy.Fetch(ctx, urlString)
			assert.ErrorIs(t, err, tc.expectedError)
			assert.Equal(t, tc.fetchCalledCount, fallbackStrategy.fetchCalledCount)
			assert.Equal(t, tc.doRequestCalledCount, fallbackStrategy.doRequestCalledCount)
		})
	}
}
