package dns

import "strings"

func SubdomainOf(sub, top string) bool {
	sub = NormName(sub)
	top = NormName(top)

	return strings.HasSuffix(sub, "."+top)
}

func RelativeTo(sub, top string) string {
	sub = NormName(sub)
	top = NormName(top)

	return strings.TrimSuffix(sub, "."+top)
}
