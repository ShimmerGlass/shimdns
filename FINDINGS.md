# Findings

Open issues found while adding tests to the repository. None of them are
covered by tests: the test suite deliberately avoids asserting on these
behaviors. Items 1, 2, 3, 4, 6, 7, 8, 9, 11 and 17 have been fixed; the original
numbering is kept.

## Likely bugs

### 5. Traefik address mode fails on the default entrypoint format

`lib/source/traefik/traefik.go:92`

Traefik entrypoints are usually declared as `":443"`, which
`netip.ParseAddrPort` rejects, so the whole source errors. With `0.0.0.0:443`
the source would publish `A 0.0.0.0`. Address mode only works with explicit IPs
on the entrypoints, or with `addresses:` set in the config.

## Questionable behavior

### 10. Traefik misses v2-style rules

`lib/source/traefik/traefik.go:219`

The regexp doesn't match v2 multi-host rules like ``Host(`a`, `b`)``, and those
hosts are silently dropped.

### 12. Mikrotik sink naming and ownership

- `recordToEntry` sends names with a trailing dot (see the `TODO`), but
  `entryMatchesRecord` compares against names without one. If RouterOS keeps
  the dot, entries would be deleted and re-added every cycle. (The mikrotik
  sink test doesn't assert on the name it sends because of this.)
- With `match_comment: false`, every static entry not tagged with the sink's
  own comment is deleted, including manual ones.

### 13. Gandi replaces the whole zone

`PUT /v5/livedns/domains/{domain}/records` replaces every record in the domain,
so any record shimdns doesn't produce (MX, TXT, NS, …) gets wiped. Worth
confirming that's intended.

### 14. mikrotik_dhcp

- One lease with a bad address fails the whole source.
- Disabled or blocked leases are still published.

### 15. `dns.NormName(".")` returns `""`

### 16. `rewrite`: unhelpful error when an expression returns `nil`

`lib/exp/compile.go:62`

`Prog.Run` rejects `nil` results, but the message reads
`unexpected return type <nil>, wanted <nil>`.
