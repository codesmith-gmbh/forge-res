# Forge CLI

The forge is a project to deploy fast on AWS. Applications that can be multi-template + multi-region (+ multi account???)

example of multiregion applications:

1. the forge cloudformation resources (partial, to bootstrap, the s3 buckets for artifacts must be created.)
2. a homepage in any region but us-east-1 with a cloudfront distribution
3. the cognito barrier application for CF installed in another region like eu-west-1

key values:

1. does _not_ reimplement anything of CF or sam
2. uses awscli + other cli tool
3. applications are described with yaml template that includes a list of 
   templates to install in order. name prefix and co.
   + wether it must be packaged or not.


