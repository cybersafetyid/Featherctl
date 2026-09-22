package generator

import (
	"fmt"
	"go/token"
	"strings"
	"unicode"
)

// maxNameLength bounds how long a feature name may be, so generated
// identifiers stay readable.
const maxNameLength = 40

// nameSeparators are the characters a feature name may use to separate words.
const nameSeparators = " \t-_."

// FeatureName is the validated, normalised form of the name a user typed for a
// feature.
//
// A user may write `user-profile`, `user_profile` or `userProfile`; all three
// produce the same Go package, identifier and route.
type FeatureName struct {
	// Raw is the name exactly as the user supplied it.
	Raw string
	// Package is the Go package name of the feature, for example
	// "userprofile".
	Package string
	// Ident is the exported Go identifier for the feature, for example
	// "UserProfile".
	Ident string
	// Route is the URL path segment of the feature, for example
	// "user-profile".
	Route string
}

// InvalidNameError reports a feature name that cannot be turned into Go
// identifiers.
type InvalidNameError struct {
	// Name is the offending input.
	Name string
	// Reason explains why it was rejected.
	Reason string
}

// Error implements the error interface.
func (e *InvalidNameError) Error() string {
	if e.Name == "" {
		return "feature name is required: " + e.Reason
	}
	return fmt.Sprintf("invalid feature name %q: %s", e.Name, e.Reason)
}

// reservedContainerFields are names featherctl already uses in the generated
// container. A feature with one of these names would produce a duplicate field
// and would not compile.
var reservedContainerFields = map[string]bool{
	"Logger": true,
}

// ParseFeatureName validates raw and derives the Go identifiers used by the
// generated feature, the router and the container.
func ParseFeatureName(raw string) (FeatureName, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return FeatureName{}, &InvalidNameError{Name: raw, Reason: "a name is required, for example \"order\""}
	}
	if len(trimmed) > maxNameLength {
		return FeatureName{}, &InvalidNameError{
			Name:   raw,
			Reason: fmt.Sprintf("shorter than %d characters", maxNameLength),
		}
	}

	words := splitWords(trimmed)
	if len(words) == 0 {
		return FeatureName{}, &InvalidNameError{Name: raw, Reason: "must contain at least one letter"}
	}

	for _, word := range words {
		if !isWord(word) {
			return FeatureName{}, &InvalidNameError{
				Name:   raw,
				Reason: "use only letters and digits, starting each word with a letter",
			}
		}
	}

	name := FeatureName{
		Raw:     trimmed,
		Package: strings.ToLower(strings.Join(words, "")),
		Ident:   strings.Join(mapWords(words, title), ""),
		Route:   strings.ToLower(strings.Join(words, "-")),
	}

	if token.Lookup(name.Package).IsKeyword() {
		return FeatureName{}, &InvalidNameError{
			Name:   raw,
			Reason: fmt.Sprintf("%q is a Go keyword", name.Package),
		}
	}
	if reservedContainerFields[name.Ident] {
		return FeatureName{}, &InvalidNameError{
			Name:   raw,
			Reason: fmt.Sprintf("%q collides with a dependency featherctl already wires into the container", name.Ident),
		}
	}

	return name, nil
}

// splitWords splits a name on separators and camel case boundaries.
func splitWords(raw string) []string {
	var (
		words   []string
		current []rune
	)

	flush := func() {
		if len(current) > 0 {
			words = append(words, string(current))
			current = nil
		}
	}

	for _, r := range raw {
		switch {
		case strings.ContainsRune(nameSeparators, r):
			flush()
		case unicode.IsUpper(r) && len(current) > 0 && unicode.IsLower(current[len(current)-1]):
			flush()
			current = append(current, r)
		default:
			current = append(current, r)
		}
	}
	flush()

	return words
}

// isWord reports whether word is letters and digits with a leading letter.
func isWord(word string) bool {
	if word == "" {
		return false
	}
	for i, r := range word {
		switch {
		case unicode.IsLetter(r):
		case i > 0 && unicode.IsDigit(r):
		default:
			return false
		}
	}
	return true
}

// mapWords applies transform to every word and returns the result.
func mapWords(words []string, transform func(string) string) []string {
	out := make([]string, 0, len(words))
	for _, word := range words {
		out = append(out, transform(word))
	}
	return out
}

// title upper cases the first rune of word and leaves the rest alone.
func title(word string) string {
	if word == "" {
		return word
	}
	runes := []rune(word)
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}
