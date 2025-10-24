package dns

import "strings"

func NormName(name string) string {
	if !strings.HasSuffix(name, ".") {
		name += "."
	}

	return name
}
