package evaluator

import (
	"fmt"

	"github.com/crossplane-contrib/function-hcl/function/internal/evaluator/hclutils"
	"github.com/hashicorp/hcl/v2"
	"github.com/zclconf/go-cty/cty"
)

// processValidation fails the evaluation with the block's error message when its condition is
// known to be false. A condition that can't be evaluated yet is skipped with a warning, like any
// other incomplete value, because the same kind of error also means a dependency isn't ready yet.
func (e *Evaluator) processValidation(ctx *hcl.EvalContext, block *hcl.Block) hcl.Diagnostics {
	content, diags := block.Body.Content(validationSchema())
	if diags.HasErrors() {
		return diags
	}

	condExpr := content.Attributes[attrCondition].Expr
	cond, ds := condExpr.Value(ctx)
	if ds.HasErrors() || !cond.IsWhollyKnown() {
		e.discard(DiscardItem{
			Type:        discardTypeValidation,
			Reason:      discardReasonIncomplete,
			SourceRange: condExpr.Range().String(),
			Context:     e.messagesFromDiags(ds),
		})
		return diags.Extend(hclutils.DowngradeDiags(ds))
	}
	diags = diags.Extend(ds)
	if cond.IsNull() || cond.Type() != cty.Bool {
		return diags.Append(&hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  fmt.Sprintf("validation condition must be a bool, got %s", cond.Type().FriendlyName()),
			Subject:  ptr(condExpr.Range()),
		})
	}
	if cond.True() {
		return diags
	}

	msgExpr := content.Attributes[attrErrMessage].Expr
	msg, ds := msgExpr.Value(ctx)
	diags = diags.Extend(ds)
	if ds.HasErrors() {
		return diags
	}
	if !msg.IsWhollyKnown() || msg.IsNull() || msg.Type() != cty.String {
		return diags.Append(&hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  "validation error_message must be a known string",
			Subject:  ptr(msgExpr.Range()),
		})
	}
	return diags.Append(&hcl.Diagnostic{
		Severity: hcl.DiagError,
		Summary:  "validation failed",
		Detail:   msg.AsString(),
		Subject:  ptr(condExpr.Range()),
	})
}
