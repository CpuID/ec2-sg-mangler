package main

import (
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/ec2"
	"gopkg.in/urfave/cli.v1"
	"log"
	"os"
	"regexp"
)

type ArgConfig struct {
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
	re := regexp.MustCompile("^sg-([0-9a-z]{8})$")
	if re.MatchString(c.String("sg")) == false {
		log.Fatalf("-s must be specified as a Security Group ID. Example: sg-asdf1234\n")
	} else {
		result.SecurityGroupId = c.String("s")
	}
	if c.String("p") != "tcp" || c.String("p") != "udp" || c.String("p") != "icmp" {
		log.Fatalf("-p must be one of 'tcp', 'udp', or 'icmp'.\n")
	} else {
		result.Protocol = c.String("p")
	}
	if result.Protocol == "tcp" || result.Protocol == "udp" {
		if c.Int("f") < 1 || c.Int("f") > 65535 {
			log.Fatalf("-f must be a port number between 1 and 65535.\n")
		} else {
			result.From = c.Int("f")
		}
		if c.Int("t") < 1 || c.Int("t") > 65535 {
			log.Fatalf("-t must be a port number between 1 and 65535.\n")
		} else {
			result.To = c.Int("t")
		}
	} else {
		if c.Int("f") < 0 || c.Int("f") > 255 {
			log.Fatalf("-f must be an ICMP type between 0 and 255. See http://www.iana.org/assignments/icmp-parameters/icmp-parameters.xhtml\n")
		} else {
			result.From = c.Int("f")
		}
		if c.Int("t") < 0 || c.Int("t") > 255 {
			log.Fatalf("-t must be an ICMP type between 0 and 255. See http://www.iana.org/assignments/icmp-parameters/icmp-parameters.xhtml\n")
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
	app.Action = func(c *cli.Context) {
		arg_config := parseFlags(c)
		// TODO: credentials handling? rely on env vars?
		asg_client := autoscaling.New(session.New())
		ec2_client := ec2.New(session.New())
		ec2metadata_client := ec2metadata.New(session.New())

		// TODO: some stuff
	}
	app.Flags = []cli.Flag{
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
