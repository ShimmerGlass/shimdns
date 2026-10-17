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

## Removal grace

A record that disappears from its sources is not removed from a sink right away. It stays published
until it has been absent for the sink's `removal_grace`. If it comes back before, the sink sees
nothing. This covers a container that is recreated and drops out of traefik for a minute: a public
resolver that asked during the gap would cache the empty answer for the zone's SOA minimum (3 hours
at Gandi).

```yaml
sinks:
  - type: gandi
    removal_grace: 5m # default for gandi
  - type: dnsserver
    removal_grace: 0s # default for every other sink: remove at once
```

- `removal_grace` is set per sink. The default is `5m` for `gandi` and `0s` (no grace) for the other
  sinks, which serve or show the records locally, where a dead record does more harm than a missing one.
- A record with the same name and type but other data (a new address) replaces the old one at once,
  so a name is never empty and the old address is not kept.
- A failing source keeps its last records as current, so the grace period only starts once the source
  reads again without the record.
- Logs: `record removed` is logged when the last sink with a grace drops the record. A disappearance
  and a return inside the grace period are logged at debug level only.
