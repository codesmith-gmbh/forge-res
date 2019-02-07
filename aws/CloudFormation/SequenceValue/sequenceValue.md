# Sequence Value: custom CloudFormation resource

The `sequenceValue` custom resource is used to fetch values from a sequence created by the `sequence` custom resource.
A `sequenceValue` custom resource draw a value from a sequence on creation only.

## Syntax

To create an `sequenceValue` resource, add the following resource to your cloudformation
template (yaml notation, json is similar)

```yaml
MySequenceValue:
  Type: Custom::SequenceValue
  Properties:
    ServiceToken: !ImportValue ForgeResources-SequenceValue
    SequenceName: !Ref Sequence
```

## Properties

`SequenceName`

> The name of the sequence to draw a value from

_Type_: String

_Required_: Yes

_Update Requires_: replacement

## Return Values

`Ref`

The `Ref` intrinsic function gives the Logical Id of the resource concatenated with the sequence name.

`Fn::GetAtt`

The attribute `Value` contains the value that has been drawn from the sequence as integer value.

The attribute `ValueText` contains the value that has been drawn from sequence as string.