# sesame-seed
AWS utility designed to be compiled for mission-critical implementation of essential functions such as `s3 cp` and `cloudwatch put-metric-data` in a cross-account environment for when you can't always depend on the `awscli` due to boto dependencies, etc. 

## Background
In designing a distributed system that pulls instruction sets from s3 buckets it was found that sometimes the awscli would break due to various botocore dependency issues. This would break the nodes ability to pull new instructions to fix the problem. Additionally, it was desired that no matter what the node would be able to send a heartbeat back to a central metrics account. 

At the time there was no compiled version of the awscli that would bypass these dependency problems. Therefore this project is an attempt to make a compiled version of the core mission-critical functions that the awscli was providing.

## Assumptions
The nodes in mind are in separate AWS accounts and have instance profiles that allow them to perform the pre-set actions in the binary.

For example--for s3 access--the instances have profiles with a path and name such as `/devopsdept/sesame-seed-role` which grant them access to `s3:GetObject` on a specific bucket in another account. The bucket in the other account is set to allow anyone in that account using that role ARN access to the bucket. Therefore no credentials options are supported currently with the s3 fuctions.

Another example--for metrics--due to the limitations on Cloudwatch metrics authentication either permanent creds or an explicit assume-role is required to push metrics cross-account. Since the author did not want to manage yet another set of perm creds it was decided to write this utility to assume the use of assume-role. Therefore the instance-profile on the node running this code should have the ability to assume the role specified in the `-assume-role-arn` parameter.

## Installation
Grab one of the binaries from the release section. 

## Usage

To download an object:
```
./sesame-seed -function s3download -s3bucket my-bucket-name -s3key /my/object/key -s3dest /path/on/disk
``` 

To put a cloudwatch metric:
```
./sesame-seed -function cwputmetric -cwnamespace my-heartbeats -cwvalue 1 -cwmetricname my-heartbeat -cwdimensions "Host=i-12345asdbewa123,HostType=webserver" -cwassumerolearn arn:aws:iam::123456789012:role/devopsdept/metrics-putter
```

# Contributing
Open a PR.
Tags will kick off travis build.