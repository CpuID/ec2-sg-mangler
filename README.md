# ec2-sg-mangler

*Helper utility to manage the EC2 instance public IPs in an AWS Security Group*

# Summary

If you have some infrastructure built within a VPC, and some legacy infrastructure outside a VPC ([EC2-Classic](http://docs.aws.amazon.com/AWSEC2/latest/UserGuide/using-vpc.html#differences-ec2-classic-vpc)),
you may have a requirement to send traffic from VPC'ed resources to EC2-Classic resources.

This gets more complicated if the VPC'ed resources are EC2 instances in an Auto Scaling Group, as the source public IP
address/es can change and are not predictable (and you don't want to add [all AWS Public Subnets](https://ip-ranges.amazonaws.com/ip-ranges.json) to your SG as valid sources).

Instead, this utility can manage the Source IP Addresses for an AWS Security Group, for a list of protocols and ports
respectively. The EC2 instances in question may be a single instance (such as the current instance, determined via the EC2
metadata service), or all EC2 instance public IP addresses for a given Auto Scaling Group.

Note this utility assumes [ClassicLink](http://docs.aws.amazon.com/AWSEC2/latest/UserGuide/vpc-classiclink.html) is not an
option for whatever reason (eg. you are using 10.x/8 subnets within your VPCs), in such a scenario this utility would
not be required theoretically.

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

A basic example is available below:

```

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

# Building

`go get -d && go build` should produce a single executable.


