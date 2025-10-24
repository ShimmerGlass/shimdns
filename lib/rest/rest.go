package rest

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

type Request struct {
	URL  string
	Path string

	ExpectEmptyResponse bool

	BasicUser string
	BasicPass string
}

func Get[T any](ctx context.Context, r Request) (T, error) {
	return req[T](ctx, http.MethodGet, r, nil)
}

func Put[T any](ctx context.Context, body any, r Request) (T, error) {
	return req[T](ctx, http.MethodPut, r, body)
}

func Delete[T any](ctx context.Context, body any, r Request) (T, error) {
	return req[T](ctx, http.MethodDelete, r, body)
}

func req[T any](ctx context.Context, method string, r Request, body any) (T, error) {
	res, err := reqInternal[T](ctx, method, r, body)
	if err != nil {
		return res, fmt.Errorf("%s %s %s: %w", method, r.URL, r.Path, err)
	}

	return res, nil
}

func reqInternal[T any](ctx context.Context, method string, r Request, body any) (T, error) {
	var data T

	buf := &bytes.Buffer{}
	if body != nil {
		err := json.NewEncoder(buf).Encode(body)
		if err != nil {
			return data, err
		}
	}

	url, err := url.JoinPath(r.URL, r.Path)
	if err != nil {
		return data, err
	}

	req, err := http.NewRequestWithContext(ctx, method, url, buf)
	if err != nil {
		return data, err
	}

	if r.BasicUser != "" {
		req.SetBasicAuth(r.BasicUser, r.BasicPass)
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return data, err
	}
	defer res.Body.Close()

	if res.StatusCode >= 400 {
		body, err := io.ReadAll(res.Body)
		if err != nil {
			return data, fmt.Errorf("bad status code: %d", res.StatusCode)
		}

		return data, fmt.Errorf("bad status code %d: %s", res.StatusCode, string(body))
	}

	if !r.ExpectEmptyResponse {
		err = json.NewDecoder(res.Body).Decode(&data)
		if err != nil {
			return data, err
		}
	}

	return data, nil
}
