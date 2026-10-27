---
title: "Validation"
linkTitle: "Validation"
weight: 15
description: >
  Rejecting invalid input with an error message.
---

The `validation` block fails the evaluation with its `error_message` when its `condition` is false.
Use it to reject invalid input instead of rendering nothing. It can appear at the top level and in groups.

## Syntax

```hcl
validation {
  condition     = length(try(req.composite.spec.parameters.subnets, [])) > 0
  error_message = "subnets must list at least one subnet"
}
```

## Behavior

| Condition | Result |
|-----------|--------|
| `true` | Nothing happens. |
| `false` | The evaluation fails with `validation failed; <error_message>`, whether or not the resources already exist. |
| Cannot be evaluated yet | The block is skipped with a warning, like other incomplete values. |
| Not a bool | The evaluation fails. |

A condition that reads a missing attribute cannot be evaluated, so it is skipped rather than failed. Read
optional inputs with `try`, as in the example above, so that a missing value makes the condition false.

A `validation` block inside a group whose `condition` is false is not evaluated.
