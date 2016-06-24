package main

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/ec2"
	"gopkg.in/urfave/cli.v1"
	"log"
	"os"
	"regexp"
	"strings"
)

type ArgConfig struct {
	AwsRegion       string
	SecurityGroupId string
	// Port or ICMP Type
	From int
	// Port or ICMP Type
	To                   int
	Protocol             string
	ThisEc2Instance      bool
	AutoScalingGroupName string
}

func parseFlags(c *cli.Context) *ArgConfig {
	var result ArgConfig
	result.AwsRegion = c.String("r")
	re := regexp.MustCompile("^sg-([0-9a-z]{8})$")
	if re.MatchString(c.String("s")) == false {
		log.Printf("Error: -s must be specified as a Security Group ID. Example: sg-asdf1234\n\n")
		cli.ShowAppHelp(c)
		os.Exit(1)
	} else {
		result.SecurityGroupId = c.String("s")
	}
	if c.String("p") != "tcp" && c.String("p") != "udp" && c.String("p") != "icmp" {
		log.Printf("Error: -p must be one of 'tcp', 'udp', or 'icmp'.\n\n")
		cli.ShowAppHelp(c)
		os.Exit(1)
	} else {
		result.Protocol = c.String("p")
	}
	if result.Protocol == "tcp" || result.Protocol == "udp" {
		if c.Int("f") < 1 || c.Int("f") > 65535 {
			log.Printf("Error: -f must be a port number between 1 and 65535.\n\n")
			cli.ShowAppHelp(c)
			os.Exit(1)
		} else {
			result.From = c.Int("f")
		}
		if c.Int("t") < 1 || c.Int("t") > 65535 {
			log.Printf("Error: -t must be a port number between 1 and 65535.\n\n")
			cli.ShowAppHelp(c)
			os.Exit(1)
		} else {
			result.To = c.Int("t")
		}
	} else {
		if c.Int("f") < 0 || c.Int("f") > 255 {
			log.Printf("Error: -f must be an ICMP type between 0 and 255. See http://www.iana.org/assignments/icmp-parameters/icmp-parameters.xhtml\n\n")
			cli.ShowAppHelp(c)
			os.Exit(1)
		} else {
			result.From = c.Int("f")
		}
		if c.Int("t") < 0 || c.Int("t") > 255 {
			log.Printf("Error: -t must be an ICMP type between 0 and 255. See http://www.iana.org/assignments/icmp-parameters/icmp-parameters.xhtml\n\n")
			cli.ShowAppHelp(c)
			os.Exit(1)
		} else {
			result.To = c.Int("t")
		}
	}
	result.ThisEc2Instance = c.Bool("i")
	result.AutoScalingGroupName = c.String("a")
	return &result
}

func main() {
	app := cli.NewApp()
	app.Name = "ec2-sg-mangler"
	app.Version = "0.1.0"
	app.Usage = "Helper utility to manage the EC2 instance public IPs in an AWS Security Group"
	app.Action = func(c *cli.Context) error {
		arg_config := parseFlags(c)

		ec2metadata_client := ec2metadata.New(session.New())
		err := setAwsRegion(ec2metadata_client, arg_config)
		if err != nil {
			log.Fatalf(err.Error())
		}

		// Reusable config session object for AWS services with current region attached.
		aws_config_session := session.New(&aws.Config{Region: aws.String(arg_config.AwsRegion)})

		asg_client := autoscaling.New(aws_config_session)
		ec2_client := ec2.New(aws_config_session)

		var proposed_ips []string
		if len(arg_config.AutoScalingGroupName) > 0 {
			log.Printf("Fetching list of Auto Scaling Group IPs...\n")
			proposed_ips, err = getAsgInstancePublicIps(asg_client, ec2_client, arg_config.AutoScalingGroupName)
			if err != nil {
				log.Fatalf(err.Error())
			}
		}
		var this_instance_ip string
		if arg_config.ThisEc2Instance == true {
			log.Printf("Fetching the public IP of this EC2 instance...\n")
			this_instance_ip, err = getThisInstancePublicIp(ec2metadata_client)
			if err != nil {
				log.Fatalf(err.Error())
			}
			proposed_ips = append(proposed_ips, this_instance_ip)
		}
		proposed_ips = removeSliceDuplicates(proposed_ips)

		log.Printf("Fetching the IPs currently attached to Security Group...\n")
		sg_ips, err := getCurrentMatchingSgIps(ec2_client, arg_config.SecurityGroupId, arg_config.From, arg_config.To, arg_config.Protocol)
		if err != nil {
			log.Fatalf(err.Error())
		}

		sg_actions := reconcileIps(sg_ips, proposed_ips)
		log.Printf("Commencing SG changes (if any)...\n")
		if len(sg_actions.Add) > 0 {
			err = doAddSgIps(ec2_client, arg_config.SecurityGroupId, arg_config.From, arg_config.To, arg_config.Protocol, sg_actions.Add)
			if err != nil {
				log.Fatalf(err.Error())
			}
			log.Printf("Added Ingress IPs %s to SG %s.\n", strings.Join(sg_actions.Add, ", "), arg_config.SecurityGroupId)
		}
		if len(sg_actions.Remove) > 0 {
			err = doRemoveSgIps(ec2_client, arg_config.SecurityGroupId, arg_config.From, arg_config.To, arg_config.Protocol, sg_actions.Remove)
			if err != nil {
				log.Fatalf(err.Error())
			}
			log.Printf("Removed Ingress IPs %s to SG %s.\n", strings.Join(sg_actions.Remove, ", "), arg_config.SecurityGroupId)
		}
		log.Printf("All done.")
		return nil
	}
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "r",
			Value: "",
			Usage: "AWS Region",
		},
		cli.StringFlag{
			Name:  "s",
			Value: "",
			Usage: "Security Group ID (must exist already)",
		},
		cli.IntFlag{
			Name:  "f",
			Value: 1,
			Usage: "From Port Number OR ICMP Type (-1 for all ICMP types)",
		},
		cli.IntFlag{
			Name:  "t",
			Value: 1,
			Usage: "To Port Number OR ICMP Type (-1 for all ICMP types )",
		},
		cli.StringFlag{
			Name:  "p",
			Value: "tcp",
			Usage: "IP Protocol Name (may be one of 'tcp', 'udp', or 'icmp' currently)",
		},
		cli.BoolFlag{
			Name:  "i",
			Usage: "Add this EC2 instance public IP? Not required if this instance is part of the nominated ASG",
		},
		cli.StringFlag{
			Name:  "a",
			Value: "",
			Usage: "Add the public IPs for all EC2 instances in this ASG Name",
		},
	}

	app.Run(os.Args)
}
