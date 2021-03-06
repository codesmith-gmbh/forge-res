# Bootstrap cloudformation template.

The template is used to create the install/update jobs for the fundamental core of the forge.

It is normally installed using the `forge` cli tool (alternatively, until the `forge` cli tool is
available). The following snippet is used for testing.

```
aws --region us-east-1 cloudformation deploy \
    --stack-name ForgeBootstrap \
    --template-file templates/ForgeBootstrap.yaml \
    --capabilities CAPABILITY_NAMED_IAM \
    --parameter-overrides \
        ForgeDomainName=codesmith.ch
```

kick the release of the current commit with the following command.
```
aws --region us-east-1 codebuild start-build \
    --project-name ForgeBootstrap \
    --source-version $(git rev-parse HEAD) \
    --environment-variables-override \
        name=AWS_REGION,value=eu-west-1,type=PLAINTEXT
```

when the forge is installed in (all) the necessary region(s), you can delete the ForgeBootstrap stack with the following
command:

```
aws cloudformation delete-stack --stack-name ForgeBootstrap
```