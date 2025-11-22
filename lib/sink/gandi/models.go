package gandi

type DomainRecord struct {
	RrsetType   string   `json:"rrset_type,omitempty"`
	RrsetTTL    int      `json:"rrset_ttl,omitempty"`
	RrsetName   string   `json:"rrset_name,omitempty"`
	RrsetHref   string   `json:"rrset_href,omitempty"`
	RrsetValues []string `json:"rrset_values,omitempty"`
}

type Records struct {
	Items []*DomainRecord `json:"items"`
}
