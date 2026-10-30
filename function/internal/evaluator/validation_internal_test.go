package evaluator

import (
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// processValidations skips content without validation blocks before evaluating its locals, so a
// composition without them evaluates its locals only once.
func TestHasValidation(t *testing.T) {
	tests := []struct {
		name string
		hcl  string
		want bool
	}{
		{
			name: "none",
			hcl: `
locals {
  a = 1
}
group {
  locals {
    b = 2
  }
}
`,
		},
		{
			name: "top level",
			hcl: `
validation {
  condition     = true
  error_message = "m"
}
`,
			want: true,
		},
		{
			name: "nested group",
			hcl: `
group {
  group {
    validation {
      condition     = true
      error_message = "m"
    }
  }
}
`,
			want: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			file, diags := hclsyntax.ParseConfig([]byte(test.hcl), "main.hcl", hcl.InitialPos)
			require.False(t, diags.HasErrors(), diags.Error())
			content, diags := file.Body.Content(topLevelSchema())
			require.False(t, diags.HasErrors(), diags.Error())
			assert.Equal(t, test.want, hasValidation(content))
		})
	}
}
