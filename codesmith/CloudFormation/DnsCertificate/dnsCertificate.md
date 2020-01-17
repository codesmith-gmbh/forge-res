# Dns Certificate (OBSOLETE!!)

**OBSOLETE**: Use AcmCertificate, AcmCertificateDns?? and AcmCertificateIssueWait
together with the helper AcmVerifiedCertificate (expends into these 3 resources).

As of 2018-07-10, cloudformation does not support a ACM SSL
certification with DNS verification, only the old method via email. This
custom resource lambda function allows the creation of DNS verified ACM
SSL certificate.

### Syntax

To create a new ACM certificate, add the following resource to your
cloudformation template (yaml notation, json is similar)

```yaml
MyCertificate:
  Type: Custom::DnsCertificate
  Properties:
    ServiceToken: !ImportValue ForgeResources-DnsCertificate
    DomainName: <main-domain-name>
    Region: <region of the certificate>
    SubjectAlternativeNames:
    - <alternative names>
    - ...
    Tags:
    - Key: key
      Value: value
    - ...
```

### Properties

`DomainName`

> The main domain name for this certificate.
>
> _Type_: String
>
> _Required_: Yes
>
> _Update Requires_: Replacement

`Region`

> The region for the certificate. This is mostly useful to create
> certificates in the us-east-1 region for stacks that are _not_ in the
> us-east-1 region and that creates cloudfront distributions. If not
> specified, it is the region of the stack.
>
> _Type_: Region (string)
>
> _Required_: No
>
> _Update Requires_: Replacement

`SubjectAlternativeNames`

> Additional Domain Names for the certificate.
>
> _Type_: List of String
>
> _Required_: No
>
> _Update Requires_: Replacement

`Tags`

> Tags to apply on the certificate.
>
> _Type_: List of Tags (a Tag a a map with keys `Key` and `Value`)
>
> _Required_: No
>
> _Update Requires_: No interruption.

### Return Values

`Ref`

The `Ref` intrinsic function gives the ARN of the created certificate

`Fn::GetAtt`

For every domain name (given either through the property `DomainName` or
the property `SubjectAlternativeNames`, the resource generated 3
attributes for the CNAME record that is used for validation.

1. `<domain-name>-RecordName` : the name for the DNS record.
2. `<domain-name>-RecordValue`: the value for the DNS record.
3. `<domain-name>-RecordType`: the type of the DNS record (as of 2019-02-19, only CNAME).

If you use Route53 for DNS, you can use these attributes to generate
corresponding records in your HostedZone; you can use the companion resource
`DnsCertificateRecordSetGroup` to simplify.

### Example

The following yaml fragment create a SSL certificate for the domains
`test.com` and `hello.test.com` in the region us-east-1.

```yaml
TestComCertificate:
  Type: Custom::DnsCertificate
  Properties:
    ServiceToken: !ImportValue ForgeResources-DnsCertificate
    DomainName: test.com
    Region: us-east-1
    SubjectAlternativeNames:
    - hello.test.com
```
