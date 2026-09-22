package generator_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cybersafetyid/featherctl/internal/generator"
)

func TestParseFeatureName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  generator.FeatureName
	}{
		{
			input: "order",
			want:  generator.FeatureName{Raw: "order", Package: "order", Ident: "Order", Route: "order"},
		},
		{
			input: "user-profile",
			want:  generator.FeatureName{Raw: "user-profile", Package: "userprofile", Ident: "UserProfile", Route: "user-profile"},
		},
		{
			input: "user_profile",
			want:  generator.FeatureName{Raw: "user_profile", Package: "userprofile", Ident: "UserProfile", Route: "user-profile"},
		},
		{
			input: "userProfile",
			want:  generator.FeatureName{Raw: "userProfile", Package: "userprofile", Ident: "UserProfile", Route: "user-profile"},
		},
		{
			input: "UserProfile",
			want:  generator.FeatureName{Raw: "UserProfile", Package: "userprofile", Ident: "UserProfile", Route: "user-profile"},
		},
		{
			input: "  order  ",
			want:  generator.FeatureName{Raw: "order", Package: "order", Ident: "Order", Route: "order"},
		},
		{
			input: "order2",
			want:  generator.FeatureName{Raw: "order2", Package: "order2", Ident: "Order2", Route: "order2"},
		},
	}

	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			t.Parallel()

			got, err := generator.ParseFeatureName(test.input)
			require.NoError(t, err)
			assert.Equal(t, test.want, got)
		})
	}
}

func TestParseFeatureNameRejects(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "empty", input: "", want: "required"},
		{name: "blank", input: "   ", want: "required"},
		{name: "leading digit", input: "2fast", want: "letters and digits"},
		{name: "punctuation", input: "order!", want: "letters and digits"},
		{name: "keyword", input: "func", want: "Go keyword"},
		{name: "reserved container field", input: "logger", want: "collides"},
		{name: "too long", input: strings.Repeat("a", 41), want: "shorter than"},
		{name: "separators only", input: "---", want: "at least one letter"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			_, err := generator.ParseFeatureName(test.input)
			require.Error(t, err)
			assert.Contains(t, err.Error(), test.want)

			var invalid *generator.InvalidNameError
			assert.ErrorAs(t, err, &invalid)
		})
	}
}
