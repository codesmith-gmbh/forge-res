# Sequence Generator

The `CogCondPreAuthSettings` custom resource is used to configure the
`cog_cond_pre_auth` cognito trigger lambda with a SSM parameter.

For more information, consult the documentation of the `cog_cond_pre_auth`
cognito trigger

## Syntax

To create an `CogCondPreAuthSettings` resource, add the following resource
to your cloudformation template (yaml notation, json is similar)

```yaml
UserPoolPreAuthSettings:
  Type: Custom::CogCondPreAuthSettings
  Properties:
    ServiceToken:
      Fn::ImportValue:
        !Sub ${HyperdriveCore}-CogCondPreAuthSettings
    UserPoolId: <userpool-id>
	   UserPoolCliendId: <userpoolclient-id>
    All: false
    Domains:
    - test.com
    Emails:
    - stan@test2.com
```

## Properties

`UserPoolId`

> The id of the user pool to which the trigger `cog_cond_pre_auth` is attached.

_Type_: String

_Required_: Yes

_Update Requires_: replacement


`UserPoolCliendId`

> The id of the user pool client id used to login users.

_Type_: String

_Required_: Yes

_Update Requires_: replacement


`All`:

> A flag to configure whether all users can authenticate via the given client.

_Type_: boolean

_Required_: no (default: false)

_Update Requires_: no interruption


`Domains`:

> A list of email domains to whitelist

_Type_: List of Strings

_Required_: no (default: [])

_Update Requires_: no interruption


`Emails`:

> A list of individual emails to whitelist

_Type_: List of Strings

_Required_: no (default: [])

_Update Requires_: no interruption

## Return Values

`Ref`

The `Ref` intrinsic function gives the name of the created SSM parameter