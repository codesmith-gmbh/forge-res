# ListenerRuleSwapper

The `ListenerRuleSwapper` custom resource is meant for a very specific use case of an application served by an ALB
and that requires interruption on release.

## Syntax

To create an ListenerRuleSwapper resource, add the following resource to your cloudformation
template (yaml notation, json is similar)

```yaml
ListenerRuleSwapper:
  Type: Custom::ListenerRuleSwapper
  Properties:
    ServiceToken: !ImportValue ForgeResources-ListenerRuleSwapper
	   ListenerArn: <listener arn>
    Rule1Arn: <rule arn>
	   Rule2Arn: <rule arn>
    Trigger: <changing value>
```