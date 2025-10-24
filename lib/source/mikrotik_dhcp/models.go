package mikrotikdhcp

type Lease struct {
	ID               string `json:".id"`
	ActiveAddress    string `json:"active-address"`
	ActiveClientID   string `json:"active-client-id"`
	ActiveMacAddress string `json:"active-mac-address"`
	ActiveServer     string `json:"active-server"`
	Address          string `json:"address"`
	AddressLists     string `json:"address-lists"`
	Blocked          string `json:"blocked"`
	ClassID          string `json:"class-id"`
	ClientID         string `json:"client-id"`
	DhcpOption       string `json:"dhcp-option"`
	Disabled         string `json:"disabled"`
	Dynamic          string `json:"dynamic"`
	ExpiresAfter     string `json:"expires-after"`
	LastSeen         string `json:"last-seen"`
	MacAddress       string `json:"mac-address"`
	Radius           string `json:"radius"`
	Server           string `json:"server"`
	Status           string `json:"status"`
	Comment          string `json:"comment"`
}
