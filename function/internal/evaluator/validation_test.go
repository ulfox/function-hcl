package evaluator_test

import (
	"testing"

	"github.com/crossplane-contrib/function-hcl/function/internal/evaluator"
	fnv1 "github.com/crossplane/function-sdk-go/proto/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const validationBucket = `
resource primary-bucket {
  body = {
    apiVersion = "aws.com/v1"
    kind       = "S3Bucket"
    metadata   = { name = "primary" }
  }
}
`

// noObservedResources turns the base request, which observes primary-bucket, into a new XR.
func noObservedResources(req *fnv1.RunFunctionRequest) {
	req.Observed.Resources = nil
}

func TestValidation(t *testing.T) {
	tests := []struct {
		name      string
		hcl       string
		fn        func(*fnv1.RunFunctionRequest)
		errSubstr string // empty when the render must succeed
		resources []string
	}{
		{
			name: "condition true",
			hcl: `
				validation {
				  condition     = req.composite.spec.parameters.region == "us-east-1"
				  error_message = "region must be us-east-1"
				}
			` + validationBucket,
			resources: []string{"primary-bucket"},
		},
		{
			name: "condition false, new XR",
			hcl: `
				validation {
				  condition     = length(try(req.composite.spec.parameters.subnets, [])) > 0
				  error_message = "subnets must list at least one subnet"
				}
			` + validationBucket,
			fn:        noObservedResources,
			errSubstr: "validation failed; subnets must list at least one subnet",
		},
		{
			name: "condition false, existing XR",
			hcl: `
				validation {
				  condition     = req.composite.spec.parameters.region == "eu-west-1"
				  error_message = "region must be eu-west-1"
				}
			` + validationBucket,
			errSubstr: "validation failed; region must be eu-west-1",
		},
		{
			name: "condition not known yet",
			hcl: `
				validation {
				  condition     = req.resource["primary-bucket"].status.atProvider.arn != ""
				  error_message = "the bucket has no ARN"
				}
			` + validationBucket,
			fn:        noObservedResources,
			resources: []string{"primary-bucket"},
		},
		{
			name: "condition not a bool",
			hcl: `
				validation {
				  condition     = "yes"
				  error_message = "never shown"
				}
			`,
			errSubstr: "validation condition must be a bool, got string",
		},
		{
			name: "inside a group that is skipped",
			hcl: `
				group {
				  condition = false
				  validation {
				    condition     = false
				    error_message = "never shown"
				  }
				}
			` + validationBucket,
			resources: []string{"primary-bucket"},
		},
		{
			name: "error_message missing",
			hcl: `
				validation {
				  condition = true
				}
			`,
			errSubstr: `Missing required argument; The argument "error_message" is required`,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := makeRequest(t, baseRequestJSON, test.fn)
			e, err := evaluator.New(evaluator.Options{})
			require.NoError(t, err)

			res, err := e.Eval(req, evaluator.File{Name: "main.hcl", Content: test.hcl})
			if test.errSubstr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), test.errSubstr)
				return
			}
			require.NoError(t, err)
			for _, name := range test.resources {
				assert.Contains(t, res.Desired.Resources, name)
			}
		})
	}
}
