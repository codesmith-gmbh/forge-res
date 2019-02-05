# Bootstrap cloudformation template.

The template is used to create the install/update jobs for the fondamental core of the forge.

It is normally installed using the `forge` cli tool. The following snippet is used for testing

```
aws --region us-east-1 cloudformation deploy \
    --stack-name ForgeBootstrap \
    --template-file bootstrap.yaml \
    --capabilities CAPABILITY_IAM
```