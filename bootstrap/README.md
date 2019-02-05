# Bootstrap cloudformation template.

The template is used to create the install/update jobs for the fondamental core of the forge.

It is normally installed using the `forge` cli tool. The following snippet is used for testing

```
aws --region us-east-1 cloudformation deploy \
    --stack-name ForgeBootstrap \
    --template-file bootstrap.yaml \
    --capabilities CAPABILITY_IAM
```

kick the release of the current commit with the following command

```
aws --region us-east-1 codebuild start-build \
    --project-name CodesmithForgeBootstrap \
    --source-version $(git rev-parse HEAD) \
    --environment-variables-override \
        name=AWS_REGION,value=eu-west-1,type=PLAINTEXT
```