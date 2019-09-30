# CogCondPreAuth

The CogCondPreAuth lambda function is as a hook for the PreAuth event of a Cognito User and is used in conjunction
with the cogCondPreAuthSettings custom Cloudformation resource for its configuration.

The CogCondPreAuth lambda function will allow/deny authentication based on the email address of the user logging in.
Domains and individual email addresses can be whitelisted (via the cogCondPreAuthSettings custom CloudFormation
resource.
