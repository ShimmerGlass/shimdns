# ShimDNS - A smart DNS connector and server !

ShimDNS builds DNS records from various sources, processes them, and exposes / publishes them.

ShimDNS runs a a daemon on a configured interval and dynamically updates DNS records when sources change.

## Supported sources

- Traefik
- Mikrotik DHCP leases
- Netbox
- File
- HTTP

## Supported sinks

- Integrated DNS server
- Miktotik static DNS
- HTTP

## Modifiers

- Filter
- Auto PTR generation
- Rewrite
