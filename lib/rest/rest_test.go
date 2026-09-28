package rest

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
)

type item struct {
	Name string `json:"name"`
}

func TestGet(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, "/api/items", r.URL.Path)

		_, _, ok := r.BasicAuth()
		require.False(t, ok)

		_ = json.NewEncoder(w).Encode([]item{{Name: "a"}, {Name: "b"}})
	}))
	defer srv.Close()

	res, err := Get[[]item](context.Background(), Request{URL: srv.URL, Path: "/api/items"})
	require.NoError(t, err)
	require.Equal(t, []item{{Name: "a"}, {Name: "b"}}, res)
}

func TestGetURLWithPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/prefix/api/items", r.URL.Path)
		_ = json.NewEncoder(w).Encode([]item{})
	}))
	defer srv.Close()

	_, err := Get[[]item](context.Background(), Request{URL: srv.URL + "/prefix/", Path: "/api/items"})
	require.NoError(t, err)
}

func TestQueryAndResponseHeader(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/items", r.URL.Path)
		require.Equal(t, "2", r.URL.Query().Get("page"))
		w.Header().Set("X-Next-Page", "3")
		_ = json.NewEncoder(w).Encode([]item{})
	}))
	defer srv.Close()

	var hdr http.Header
	_, err := Get[[]item](context.Background(), Request{
		URL:            srv.URL,
		Path:           "/api/items",
		Query:          url.Values{"page": {"2"}},
		ResponseHeader: &hdr,
	})
	require.NoError(t, err)
	require.Equal(t, "3", hdr.Get("X-Next-Page"))
}

func TestBasicAuth(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		require.True(t, ok)
		require.Equal(t, "user", user)
		require.Equal(t, "pass", pass)

		_ = json.NewEncoder(w).Encode(item{Name: "a"})
	}))
	defer srv.Close()

	res, err := Get[item](context.Background(), Request{URL: srv.URL, BasicUser: "user", BasicPass: "pass"})
	require.NoError(t, err)
	require.Equal(t, item{Name: "a"}, res)
}

func TestPut(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPut, r.Method)

		var body item
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		require.Equal(t, item{Name: "new"}, body)

		_ = json.NewEncoder(w).Encode(item{Name: "created"})
	}))
	defer srv.Close()

	res, err := Put[item](context.Background(), item{Name: "new"}, Request{URL: srv.URL})
	require.NoError(t, err)
	require.Equal(t, item{Name: "created"}, res)
}

func TestDeleteEmptyResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodDelete, r.Method)
		require.Equal(t, "/items/1", r.URL.Path)

		body, _ := io.ReadAll(r.Body)
		require.Empty(t, body)

		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	_, err := Delete[any](context.Background(), nil, Request{URL: srv.URL, Path: "/items/1", ExpectEmptyResponse: true})
	require.NoError(t, err)
}

func TestBadStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte("go away"))
	}))
	defer srv.Close()

	_, err := Get[item](context.Background(), Request{URL: srv.URL, Path: "/x"})
	require.ErrorContains(t, err, "403")
	require.ErrorContains(t, err, "go away")
	require.ErrorContains(t, err, "GET")
}

func TestInvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("{not json"))
	}))
	defer srv.Close()

	_, err := Get[item](context.Background(), Request{URL: srv.URL})
	require.Error(t, err)
}

func TestContextCanceled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(item{})
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := Get[item](ctx, Request{URL: srv.URL})
	require.ErrorIs(t, err, context.Canceled)
}
