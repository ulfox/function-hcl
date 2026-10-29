package evaluator

import (
	"strings"

	"github.com/crossplane-contrib/function-hcl/function/internal/evaluator/hclutils"
	"github.com/hashicorp/hcl/v2"
	"github.com/zclconf/go-cty/cty"
)

// processValidations evaluates the validation blocks at the top level and in every group whose
// condition is true, before any other block. Other blocks stop the evaluation at their first error,
// so a resource that reads the invalid input would otherwise fail with its own expression error and
// hide the message of the rule that the input breaks. Errors in locals and group conditions are left
// to processGroup, which reports them and records the discarded groups.
func (e *Evaluator) processValidations(ctx *hcl.EvalContext, content *hcl.BodyContent) hcl.Diagnostics {
	ctx, ds := e.processLocals(ctx, content)
	if ds.HasErrors() {
		return nil
	}
	if condAttr, ok := content.Attributes[attrCondition]; ok {
		v, ds := condAttr.Expr.Value(ctx)
		if ds.HasErrors() || !v.IsWhollyKnown() || v.IsNull() || v.Type() != cty.Bool || v.False() {
			return nil
		}
	}

	var diags hcl.Diagnostics
	for _, b := range content.Blocks {
		var curDiags hcl.Diagnostics
		switch b.Type {
		case blockValidation:
			curDiags = e.processValidation(ctx, b)
		case blockGroup:
			groupContent, ds := b.Body.Content(groupSchema())
			if ds.HasErrors() {
				continue
			}
			curDiags = e.processValidations(ctx, groupContent)
		}
		diags = diags.Extend(curDiags)
		if curDiags.HasErrors() {
			return diags
		}
	}
	return diags
}

// processValidation fails the evaluation with the block's error message when its condition is
// known to be false. A condition that can't be evaluated yet is skipped with a warning, like any
// other incomplete value, because the same kind of error also means a dependency isn't ready yet.
// A condition or message whose type is known to be wrong fails the evaluation whatever the
// condition's value, so a broken block is reported before the input it checks is invalid.
func (e *Evaluator) processValidation(ctx *hcl.EvalContext, block *hcl.Block) hcl.Diagnostics {
	content, diags := block.Body.Content(validationSchema())
	if diags.HasErrors() {
		return diags
	}

	condExpr := content.Attributes[attrCondition].Expr
	cond, ds := condExpr.Value(ctx)
	if !ds.HasErrors() && hasWrongType(cond, cty.Bool) {
		return diags.Extend(ds).Append(&hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  "validation condition must be a bool",
			Detail:   "got " + cond.Type().FriendlyName(),
			Subject:  ptr(condExpr.Range()),
		})
	}
	if ds.HasErrors() || !cond.IsWhollyKnown() {
		context := e.messagesFromDiags(ds)
		if len(context) == 0 {
			context = []string{"the condition is not known yet"}
		}
		e.discard(DiscardItem{
			Type:        discardTypeValidation,
			Reason:      discardReasonIncomplete,
			Name:        attrCondition,
			SourceRange: condExpr.Range().String(),
			Context:     context,
		})
		return diags.Extend(hclutils.DowngradeDiags(ds))
	}
	diags = diags.Extend(ds)
	if cond.IsNull() {
		return diags.Append(&hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  "validation condition must be a bool",
			Detail:   "got null",
			Subject:  ptr(condExpr.Range()),
		})
	}

	// the message may interpolate values that only exist when the input is invalid, so a message
	// that can't be evaluated is an error only when the condition is false
	msgExpr := content.Attributes[attrErrMessage].Expr
	msg, msgDiags := msgExpr.Value(ctx)
	if !msgDiags.HasErrors() && hasWrongType(msg, cty.String) {
		return diags.Append(&hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  "validation error_message must be a string",
			Detail:   "got " + msg.Type().FriendlyName(),
			Subject:  ptr(msgExpr.Range()),
		})
	}
	if cond.True() {
		return diags
	}

	// the input is known to be invalid, so the evaluation fails as a validation failure even when
	// the message itself can't be built
	var detail string
	switch {
	case msgDiags.HasErrors():
		detail = "error_message could not be evaluated: " + strings.Join(e.messagesFromDiags(msgDiags), "; ")
	case !msg.IsWhollyKnown():
		detail = "error_message is not known yet"
	case msg.IsNull():
		detail = "error_message is null"
	default:
		detail = msg.AsString()
	}
	return diags.Append(&hcl.Diagnostic{
		Severity: hcl.DiagError,
		Summary:  "validation failed",
		Detail:   detail,
		Subject:  ptr(condExpr.Range()),
	})
}

// hasWrongType reports whether v's type is known and is not want. A value whose type is not known
// yet, such as an unknown or a null without a type, can still turn out to be of the right type.
func hasWrongType(v cty.Value, want cty.Type) bool {
	return v.Type() != want && v.Type() != cty.DynamicPseudoType
}
