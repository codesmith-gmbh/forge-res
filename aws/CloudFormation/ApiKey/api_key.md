# CF Api Key

To bind an Api of the API Gateway to a custom Cloud Front distribution requires the use
of a API Key to protect the native end point of the API and to make sure that the API is
available only via the Cloud Front distribution.

Unfortunately, the out-of-the-box resource `AWS::ApiGateway::ApiKey` does not allow to
extract the key secret. This resource creates api keys and export the key secret.

## Syntax
To create a new api key, add the following resource to your template

```yaml
MyCfApiKey:
  Type: Custom::ApiKey
  Properties:
    ServiceToken: !Import ForgeResources-ApiKey
    Ordinal: <number>
```

## Properties

### `Ordinal`

The name of the API keys is created automatically from the Stack Name and the Ordinal.
By making the Ordinal a parameter of the stack, one can easily rotate the keys.

_Type_: Number

_Required_: Yes

_Update Requires_: Replacement

