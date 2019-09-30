# Sequence: custom CloudFormation resource

The `sequence` custom resource is used to create a sequence that is stored as an SSM parameter.

By default, a sequence always starts with 1 and increments by 1. To alter the default behaviour,
the sequence can be created with an optional simple arithmetic expression where the variable `x` stands for the
value of the sequence.

Examples:
1. `2 * (x - 1)`: creates a sequence starting with 0 and incrementing by 2: 0, 2, 4, 6, ...
2. `8000 + x`: creates a sequence starting with 8001 and incerementing by 1: 8001, 8002, 8003, ...

To draw values from the sequence, use the `sequenceValue` custom resource.

## Syntax

To create an `sequence` resource, add the following resource to your cloudformation
template (yaml notation, json is similar)

```yaml
MySequence:
  Type: Custom::Sequence
  Properties:
    ServiceToken: !ImportValue ForgeResources-Sequence
    SequenceName: /parameter/name
    Expression: 8000 + x
```

## Properties

`SequenceName`

> The name of the sequence that will be a suffix for the underlying SSM parameter. Must start with "/".
> The SSM parameter names have the prefix "/codesmith-forge/sequence".

_Type_: String

_Required_: Yes

_Update Requires_: replacement

`Expression`

> The arithmetic expression to compute the sequence: standard operations + variable x for the current value of the
> sequence.
> WARNING: changing the expression is dangerous as the sequence could retake previously issued values.
>
> _Type_: String
>
> _Default_: x
>
> _Required_: No
>
> _Update Requires_: no interruption

## Return Values

`Ref`

The `Ref` intrinsic function gives the name of the sequence
