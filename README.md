# ec2-sg-mangler

*Helper utility to manage the EC2 instance public IPs in an AWS Security Group*

[![Build Status](https://travis-ci.org/CpuID/ec2-sg-mangler.svg?branch=master)](https://travis-ci.org/CpuID/ec2-sg-mangler)

# Summary

If you have some infrastructure built within a VPC, and some legacy infrastructure outside a VPC ([EC2-Classic](http://docs.aws.amazon.com/AWSEC2/latest/UserGuide/using-vpc.html#differences-ec2-classic-vpc)),
you may have a requirement to send traffic from VPC'ed resources to EC2-Classic resources.

This gets more complicated if the VPC'ed resources are EC2 instances in an Auto Scaling Group, as the source public IP
address/es can change and are not predictable (and you don't want to add [all AWS Public Subnets](https://ip-ranges.amazonaws.com/ip-ranges.json) to your SG as valid sources).

Instead, this utility can manage the Source IP Addresses for an AWS Security Group, for a list of protocols and ports
respectively. The EC2 instances in question may be a single instance (such as the current instance, determined via the EC2
metadata service), or all EC2 instance public IP addresses for a given Auto Scaling Group.

It is designed to be idempotent, if the IP list matches the Security Group, no changes will be made.

Note this utility assumes [ClassicLink](http://docs.aws.amazon.com/AWSEC2/latest/UserGuide/vpc-classiclink.html) is not an
option for whatever reason (eg. you are using 10.x/8 subnets within your VPCs), in such a scenario this utility would
not be required theoretically.

# Use Cases

The initial design of this utility was to run within Docker containers, just prior to starting the required application
that needed access to a remote resource in EC2-Classic. The Docker hosts all residing within an EC2 Auto Scaling Group.

# Limitations

This utility currently only supports a single Security Group, which if you have more than 50 (VPC) / 100 (EC2-Classic) instances
in an ASG, will be an issue.

A future addition would be to provide a list of SG IDs, and spread the IP address list between them to overcome the AWS restriction of
50 inbound + 50 outbound (100 total) rules on VPC based SGs, and 100 rules total for EC2-Classic based SGs.

Also, make sure you nominate independent SGs for use with this utility from all other rules, as rules will be added and removed automatically.
**Any IPs that are no longer part of the nominated list (EC2 instance or ASG instances) with a matching from/to port and protocol
will be removed from the SG.**

# IAM Policy

For this utility to operate, it needs an IAM policy attached to the credentials in use, to modify the Security Group.

Some notes:
* [DescribeAutoScalingGroups](http://docs.aws.amazon.com/AutoScaling/latest/APIReference/API_DescribeAutoScalingGroups.html) API operations do not support
resource-level permissions, so you need `*` and cannot use a specific ASG ARN.
* [DescribeSecurityGroups](http://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_DescribeSecurityGroups.html) API operations do not support
resource-level permissions, so you need `*` and cannot use a specific ASG ARN.

A basic example is available below:

```
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Sid": "Stmt1466742712000",
            "Effect": "Allow",
            "Action": [
                "autoscaling:DescribeAutoScalingGroups"
            ],
            "Resource": [
                "*"
            ]
        },
        {
            "Sid": "Stmt1466742785000",
            "Effect": "Allow",
            "Action": [
                "ec2:DescribeInstances",
                "ec2:DescribeSecurityGroups"
            ],
            "Resource": [
                "*"
            ]
        },
        {
            "Sid": "Stmt1466742815000",
            "Effect": "Allow",
            "Action": [
                "ec2:AuthorizeSecurityGroupIngress",
                "ec2:RevokeSecurityGroupIngress"
            ],
            "Resource": [
                "arn:aws:ec2:us-east-1:XXXXXXXXXXXX:security-group/sg-asdf1234"
            ]
        }
    ]
}
```

# Configuration

Configuration is performed via CLI arguments, and self documenting using `--help`:

```
NAME:
   ec2-sg-mangler - Helper utility to manage the EC2 instance public IPs in an AWS Security Group

USAGE:
   ec2-sg-mangler [global options] command [command options] [arguments...]

VERSION:
   0.1.0

COMMANDS:
GLOBAL OPTIONS:
   -r value   AWS Region
   -s value   Security Group ID (must exist already)
   -f value   From Port Number OR ICMP Type (-1 for all ICMP types) (default: 1)
   -t value   To Port Number OR ICMP Type (-1 for all ICMP types ) (default: 1)
   -p value   IP Protocol Name (may be one of 'tcp', 'udp', or 'icmp' currently) (default: "tcp")
   -i     Add this EC2 instance public IP? Not required if this instance is part of the nominated ASG
   -a value   Add the public IPs for all EC2 instances in this ASG Name
   --help, -h   show help
   --version, -v  print the version
```

AWS Credentials should be fed in using an IAM Instance Profile/Role, or the environment variables `AWS_ACCESS_KEY_ID` and `AWS_SECRET_ACCESS_KEY`.

# Use within Alpine Linux Docker image

If you are using a lightweight Docker image such as [Alpine Linux](https://hub.docker.com/_/alpine/) as your base,
there are a few prerequisites for your Dockerfile:

* Symlink MUSL libc like so: `mkdir /lib64 && ln -s /lib/libc.musl-x86_64.so.1 /lib64/ld-linux-x86-64.so.2`
  * Without it you will get an error like `sh: /ec2-sg-mangler: not found`
* Install the [ca-certificates](http://pkgs.alpinelinux.org/packages?name=ca-certificates&branch=&repo=&arch=&maintainer=) package using `apk`
  * Without it you will get an error like `caused by: Post https://autoscaling.us-east-1.amazonaws.com/: x509: failed to load system roots and no roots provided`

# Building

`go get -d && go build` should produce a single executable. Binary releases are also available [here](https://github.com/CpuID/ec2-sg-mangler/releases)
if you prefer.


