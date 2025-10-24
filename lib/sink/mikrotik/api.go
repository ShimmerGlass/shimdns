package mikrotik

import (
	"context"
	"fmt"

	"github.com/ShimmerGlass/shimdns/lib/rest"
)

type entry struct {
	ID       string `json:".id,omitempty"`
	CName    string `json:"cname"`
	Address  string `json:"address"`
	Disabled string `json:"disabled"`
	Name     string `json:"name"`
	TTL      string `json:"ttl"`
	Type     string `json:"type"`
	Comment  string `json:"comment"`
}

type api struct {
	url      string
	user     string
	password string
}

func newAPI(url string, user string, password string) *api {
	return &api{
		url:      url,
		user:     user,
		password: password,
	}
}

func (a *api) Entries(ctx context.Context) ([]entry, error) {
	return rest.Get[[]entry](ctx, rest.Request{
		URL:  a.url,
		Path: "/rest/ip/dns/static",

		BasicUser: a.user,
		BasicPass: a.password,
	})
}

func (a *api) Add(ctx context.Context, e entry) error {
	_, err := rest.Put[any](ctx, e, rest.Request{
		URL:  a.url,
		Path: "/rest/ip/dns/static",

		BasicUser: a.user,
		BasicPass: a.password,
	})
	return err
}

func (a *api) Delete(ctx context.Context, id string) error {
	_, err := rest.Delete[any](ctx, nil, rest.Request{
		URL:  a.url,
		Path: fmt.Sprintf("/rest/ip/dns/static/%s", id),

		BasicUser:           a.user,
		BasicPass:           a.password,
		ExpectEmptyResponse: true,
	})
	return err
}
