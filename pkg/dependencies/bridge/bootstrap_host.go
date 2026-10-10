package bridge

import (
	"errors"
	"fmt"
	"math"
	"net/netip"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

var (
	errHostBrackets = errors.New("Invalid IPv6 URL") //nolint:staticcheck // Preserve Python urlsplit's exact parser diagnostic.
	errHostFuture   = errors.New("IPvFuture address is invalid")
	errHostIPv4     = errors.New("An IPv4 address cannot be in brackets") //nolint:staticcheck // Preserve Python ipaddress's exact parser diagnostic.
)

// sourceURLHostname projects Python urlsplit's authority and hostname rules.
// In particular, ports and percent escapes are not parsed or validated here.
func sourceURLHostname(candidate string) (string, error) {
	remaining := strings.TrimLeftFunc(candidate, func(value rune) bool { return value <= ' ' })
	remaining = strings.NewReplacer("\t", "", "\r", "", "\n", "").Replace(remaining)

	if scheme, tail, found := strings.Cut(remaining, ":"); found && validHostScheme(scheme) {
		remaining = tail
	}

	if !strings.HasPrefix(remaining, "//") {
		return "", nil
	}

	authority := remaining[2:]
	if end := strings.IndexAny(authority, "/?#"); end >= 0 {
		authority = authority[:end]
	}

	err := validateHostAuthority(authority)
	if err != nil {
		return "", err
	}

	if separator := strings.LastIndex(authority, "@"); separator >= 0 {
		authority = authority[separator+1:]
	}

	if _, bracketed, found := strings.Cut(authority, "["); found {
		host, _, _ := strings.Cut(bracketed, "]")

		return host, nil
	}

	host, _, _ := strings.Cut(authority, ":")

	return host, nil
}

func validHostScheme(scheme string) bool {
	if scheme == "" || !asciiHostLetter(scheme[0]) {
		return false
	}

	for _, value := range []byte(scheme) {
		if !asciiHostLetter(value) && (value < '0' || value > '9') && value != '+' && value != '-' && value != '.' {
			return false
		}
	}

	return true
}

func asciiHostLetter(value byte) bool {
	return value >= 'a' && value <= 'z' || value >= 'A' && value <= 'Z'
}

func validateHostAuthority(authority string) error {
	open, closeBracket := strings.Contains(authority, "["), strings.Contains(authority, "]")
	if open != closeBracket {
		return errHostBrackets
	}

	if open {
		_, remainder, _ := strings.Cut(authority, "[")
		bracketed, _, _ := strings.Cut(remainder, "]")
		err := validateBracketedHost(bracketed)
		if err != nil {
			return err
		}
	}

	withoutSyntax := strings.NewReplacer("@", "", ":", "", "#", "", "?", "").Replace(authority)
	normalized := norm.NFKC.String(withoutSyntax)
	if normalized != withoutSyntax && strings.ContainsAny(normalized, "/?#@:") {
		//nolint:err113 // Preserve Source's exact authority diagnostic; explicitSocketHost wraps this parser cause.
		return fmt.Errorf("netloc '%s' contains invalid characters under NFKC normalization", authority)
	}

	return nil
}

func validateBracketedHost(host string) error {
	if strings.HasPrefix(host, "v") {
		version, address, found := strings.Cut(host[1:], ".")
		if !found || version == "" || address == "" || strings.ContainsAny(address, "\r\n") || !hostHexadecimal(version) {
			return errHostFuture
		}

		return nil
	}

	literal, zone, scoped := strings.Cut(host, "%")
	if scoped && (zone == "" || strings.Contains(zone, "%")) {
		return invalidHostIP(host)
	}

	address, err := netip.ParseAddr(literal)
	if err != nil {
		return invalidHostIP(host)
	}

	if address.Is4() {
		if scoped {
			return invalidHostIP(host)
		}

		return errHostIPv4
	}

	return nil
}

func invalidHostIP(host string) error {
	//nolint:err113 // Preserve Source's exact invalid-IP repr diagnostic; explicitSocketHost wraps this parser cause.
	return fmt.Errorf("%s does not appear to be an IPv4 or IPv6 address", pythonHostRepresentation(host))
}

// Python ipaddress errors use repr for invalid literals, unlike the NFKC error.
func pythonHostRepresentation(text string) string {
	quote := byte('\'')
	if strings.Contains(text, "'") && !strings.Contains(text, "\"") {
		quote = '"'
	}

	var output strings.Builder

	output.WriteByte(quote)

	for _, value := range text {
		switch {
		case value == rune(quote) || value == '\\':
			output.WriteByte('\\')
			output.WriteRune(value)
		case value == '\t':
			output.WriteString("\\t")
		case value == '\r':
			output.WriteString("\\r")
		case value == '\n':
			output.WriteString("\\n")
		case value == ' ' || unicode.IsPrint(value) && !unicode.IsSpace(value):
			output.WriteRune(value)
		case value <= math.MaxUint8:
			fmt.Fprintf(&output, "\\x%02x", value)
		case value <= math.MaxUint16:
			fmt.Fprintf(&output, "\\u%04x", value)
		default:
			fmt.Fprintf(&output, "\\U%08x", value)
		}
	}

	output.WriteByte(quote)

	return output.String()
}

func hostHexadecimal(text string) bool {
	for _, value := range []byte(text) {
		if (value < '0' || value > '9') && (value < 'a' || value > 'f') && (value < 'A' || value > 'F') {
			return false
		}
	}

	return true
}
