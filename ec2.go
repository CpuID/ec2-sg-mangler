package main

import (
	"errors"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/ec2"
	"log"
	"regexp"
	"sort"
	"strings"
)

func setAwsRegion(ec2metadata_client *ec2metadata.EC2Metadata, arg_config *ArgConfig) error {
	if len(arg_config.AwsRegion) == 0 {
		// Discover the region which this instance resides.
		use_region, err := ec2metadata_client.Region()
		if err != nil {
			return errors.New(fmt.Sprintf("Cannot retrieve AWS region from EC2 Metadata Service: %s\n", err.Error()))
		}
		arg_config.AwsRegion = use_region
	}
	if len(arg_config.AwsRegion) == 0 {
		return errors.New("We do not have an AWS Region specified (either via -r or the EC2 Metadata Service). Cannot proceed.")
	}
	return nil
}

// Retrieves the list of Public IPs of all EC2 instances attached to the nominated ASG.
func getAsgInstancePublicIps(asg_client *autoscaling.AutoScaling, ec2_client *ec2.EC2, asg_name string) ([]string, error) {
	// TODO: handle pagination using NextToken
	asg_params := &autoscaling.DescribeAutoScalingGroupsInput{
		AutoScalingGroupNames: []*string{
			aws.String(asg_name),
		},
		MaxRecords: aws.Int64(1),
	}

	asg_resp, err := asg_client.DescribeAutoScalingGroups(asg_params)

	if err != nil {
		return []string{}, err
	}
	if len(asg_resp.AutoScalingGroups) == 0 {
		return []string{}, errors.New(fmt.Sprintf("Cannot find Auto Scaling Group with Name '%s'", asg_name))
	} else if len(asg_resp.AutoScalingGroups) != 1 {
		return []string{}, errors.New("Invalid number of ASGs returned, expected 1")
	}
	if len(asg_resp.AutoScalingGroups[0].Instances) == 0 {
		log.Printf("No EC2 instances exist within the ASG '%s'.", asg_name)
		return []string{}, nil
	}

	var instance_ids []string
	for _, v1 := range asg_resp.AutoScalingGroups[0].Instances {
		// NOTE: instance health is not factored in here, generally we would want all healthy or unhealthy instances
		// to be permitted in this use case.
		instance_ids = append(instance_ids, *v1.InstanceId)
	}

	// Fetch the IPs for the Instance IDs found.
	// TODO: pagination using NextToken
	ec2_params := &ec2.DescribeInstancesInput{
		InstanceIds: aws.StringSlice(instance_ids),
	}

	ec2_resp, err := ec2_client.DescribeInstances(ec2_params)

	if err != nil {
		return []string{}, err
	}
	if len(ec2_resp.Reservations) == 0 {
		return []string{}, errors.New("No EC2 instances found, yet ASG says there are >0 instances?")
	}

	var instance_ips []string
	for _, v2 := range ec2_resp.Reservations {
		for _, v3 := range v2.Instances {
			if v3.PublicIpAddress != nil && len(*v3.PublicIpAddress) > 0 {
				instance_ips = append(instance_ips, *v3.PublicIpAddress)
			}
		}
	}

	return instance_ips, nil
}

// Returns the public IP assigned to this EC2 instance (via EC2 metadata service).
func getThisInstancePublicIp(ec2metadata_client *ec2metadata.EC2Metadata) (string, error) {
	if ec2metadata_client.Available() == false {
		return "", errors.New("Cannot use this EC2 instance IP address in the list, EC2 Metadata Service inaccessible.")
	}

	public_ipv4, err := ec2metadata_client.GetMetadata("public-ipv4")
	if err != nil {
		return "", err
	}
	return public_ipv4, nil
}

// There are a few formats that AWS specifies an IP Protocol,
// such as "tcp", "udp", "icmp", or the protocol numbers,
// and they may be uppercase or lowercase.
// Sanitise to lowercase and numeric.
// List: http://www.iana.org/assignments/protocol-numbers/protocol-numbers.xhtml
func sanitiseIpProtocol(input string) (string, error) {
	switch input {
	case "tcp":
		return "6", nil
	case "TCP":
		return "6", nil
	case "udp":
		return "17", nil
	case "UDP":
		return "17", nil
	case "icmp":
		return "1", nil
	case "ICMP":
		return "1", nil
	default:
		re := regexp.MustCompile("^([0-9])+$")
		if re.MatchString(input) == true {
			return input, nil
		} else {
			return "", errors.New("Invalid Protocol specified, this function only supports 'tcp', 'udp', 'icmp' (case insensitive) or numeric protocols.")
		}
	}
}

// Returns the IPs that have ingress rules that match the from/to port and protocol
// on the SG currently.
func getCurrentMatchingSgIps(ec2_client *ec2.EC2, sg_id string, from int, to int, protocol string) ([]string, error) {
	params := &ec2.DescribeSecurityGroupsInput{
		GroupIds: []*string{
			aws.String(sg_id),
		},
	}

	resp, err := ec2_client.DescribeSecurityGroups(params)

	if err != nil {
		return []string{}, err
	}
	if len(resp.SecurityGroups) != 1 {
		return []string{}, errors.New(fmt.Sprintf("Cannot find the Security Group ID '%s' - does it exist?", sg_id))
	}

	var result_ips []string
	// We only care about ingress for current use cases.
	for _, v1 := range resp.SecurityGroups[0].IpPermissions {
		if len(v1.IpRanges) > 0 {
			// Slightly inefficient order of loop and conditionals, but easier to output exclusions in log.
			for _, v2 := range v1.IpRanges {
				sanitise_v1_ip_protocol, err := sanitiseIpProtocol(*v1.IpProtocol)
				if err != nil {
					return []string{}, err
				}
				sanitise_protocol, err := sanitiseIpProtocol(protocol)
				if err != nil {
					return []string{}, err
				}
				if int(*v1.FromPort) == from && int(*v1.ToPort) == to && sanitise_v1_ip_protocol == sanitise_protocol {
					// As we expect only /32s here, error on anything else.
					split_ip_range := strings.Split(*v2.CidrIp, "/")
					if len(split_ip_range) != 2 {
						return []string{}, errors.New(fmt.Sprintf("Invalid CIDR Range (%s) returned from DescribeSecurityGroups API", v2.CidrIp))
					}
					if split_ip_range[1] != "32" {
						log.Printf("Excluding %s from matched IP list, not a /32\n", v2.CidrIp)
					}
					result_ips = append(result_ips, split_ip_range[0])
				} else {
					log.Printf("Excluding %s from matched IP list, unexpected from/to/protocol (unrelated rules).\n", v2.CidrIp)
				}
			}
		}
	}
	return result_ips, nil
}

type SgActions struct {
	Add    []string
	Remove []string
}

// Reconcile the IP list between the SG, and what is proposed to be in use.
func reconcileIps(sg_ips []string, proposed_ips []string) SgActions {
	// Ensure no duplicates.
	sg_ips = removeSliceDuplicates(sg_ips)
	proposed_ips = removeSliceDuplicates(proposed_ips)

	var result SgActions
	for _, v1 := range sg_ips {
		if stringInSlice(v1, proposed_ips) == false {
			result.Remove = append(result.Remove, v1)
		}
	}
	for _, v2 := range proposed_ips {
		if stringInSlice(v2, sg_ips) == false {
			result.Add = append(result.Add, v2)
		}
	}
	// NOTE: these sorts are not numeric, we would need a custom sorter to do octet level IP address sorts.
	// as long as we are consistent, a non-issue currently.
	sort.Strings(result.Add)
	sort.Strings(result.Remove)
	return result
}

func doAddSgIps(ec2_client *ec2.EC2, sg_id string, from int, to int, protocol string, sg_ip_adds []string) error {
	var ip_ranges []*ec2.IpRange
	for _, v := range sg_ip_adds {
		ip_range := new(ec2.IpRange)
		ip_range.CidrIp = aws.String(fmt.Sprintf("%s/32", v))
		ip_ranges = append(ip_ranges, ip_range)
	}
	params := &ec2.AuthorizeSecurityGroupIngressInput{
		GroupId: aws.String(sg_id),
		IpPermissions: []*ec2.IpPermission{
			{
				FromPort:   aws.Int64(int64(from)),
				IpProtocol: aws.String(protocol),
				IpRanges:   ip_ranges,
				ToPort:     aws.Int64(int64(to)),
			},
		},
	}

	_, err := ec2_client.AuthorizeSecurityGroupIngress(params)

	if err != nil {
		return err
	}
	return nil
}

func doRemoveSgIps(ec2_client *ec2.EC2, sg_id string, from int, to int, protocol string, sg_ip_adds []string) error {
	var ip_ranges []*ec2.IpRange
	for _, v := range sg_ip_adds {
		ip_range := new(ec2.IpRange)
		ip_range.CidrIp = aws.String(fmt.Sprintf("%s/32", v))
		ip_ranges = append(ip_ranges, ip_range)
	}
	params := &ec2.RevokeSecurityGroupIngressInput{
		GroupId: aws.String(sg_id),
		IpPermissions: []*ec2.IpPermission{
			{
				FromPort:   aws.Int64(int64(from)),
				IpProtocol: aws.String(protocol),
				IpRanges:   ip_ranges,
				ToPort:     aws.Int64(int64(to)),
			},
		},
	}

	_, err := ec2_client.RevokeSecurityGroupIngress(params)

	if err != nil {
		return err
	}
	return nil
}
