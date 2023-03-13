package healthcheck

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestClient_DiskUsage(t *testing.T) {
	var (
		ctx        = context.Background()
		httpClient = &http.Client{}
	)

	const host = "http://10.1.1.1"

	t.Run("happy path", func(t *testing.T) {
		client := NewClient(httpClient)
		require.NotNil(t, client.httpDo)

		want := DiskUsageResponse{
			Dir:       "/test",
			AllBytes:  100,
			FreeBytes: 10,
		}

		client.httpDo = func(req *http.Request) (*http.Response, error) {
			require.Equal(t, "http://10.1.1.1:1251/disk", req.URL.String())
			require.Equal(t, "GET", req.Method)

			b, err := json.Marshal(want)
			if err != nil {
				panic(err)
			}
			return &http.Response{Body: io.NopCloser(bytes.NewReader(b))}, nil
		}

		got, err := client.DiskUsage(ctx, host)

		require.NoError(t, err)
		require.Equal(t, want, got)
	})

	t.Run("request error", func(t *testing.T) {
		client := NewClient(httpClient)
		client.httpDo = func(req *http.Request) (*http.Response, error) {
			return nil, errors.New("boom")
		}
		_, err := client.DiskUsage(ctx, host)

		require.Error(t, err)
		require.EqualError(t, err, "http do: boom")
	})

	t.Run("error in response", func(t *testing.T) {
		client := NewClient(httpClient)

		stub := DiskUsageResponse{Error: "something bad happened"}
		client.httpDo = func(req *http.Request) (*http.Response, error) {
			b, err := json.Marshal(stub)
			if err != nil {
				panic(err)
			}
			return &http.Response{
				Body: io.NopCloser(bytes.NewReader(b)),
			}, nil
		}

		_, err := client.DiskUsage(ctx, host)

		require.Error(t, err)
		require.EqualError(t, err, "something bad happened")
	})

	t.Run("invalid JSON", func(t *testing.T) {
		client := NewClient(httpClient)

		client.httpDo = func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				Body: io.NopCloser(strings.NewReader("{")),
			}, nil
		}

		_, err := client.DiskUsage(ctx, host)

		require.Error(t, err)
		require.EqualError(t, err, "malformed json: unexpected EOF")
	})

	t.Run("zero values", func(t *testing.T) {
		client := NewClient(httpClient)

		client.httpDo = func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				Body: io.NopCloser(strings.NewReader(`{}`)),
			}, nil
		}

		_, err := client.DiskUsage(ctx, host)

		require.Error(t, err)
		require.EqualError(t, err, "invalid response: 0 free bytes")
	})
}
