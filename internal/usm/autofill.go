package usm

import "net/url"

type Autofill struct {
	AllowHTTP  bool     `json:"allow_http,omitempty"`
	TLDPlusOne string   `json:"tld_plus_one,omitempty"`
	URL        *url.URL `json:"url,omitempty"`
}
