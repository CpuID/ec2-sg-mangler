package main

import (
	"flag"
	"gopkg.in/urfave/cli.v1"
	"reflect"
	"testing"
)

func TestParseFlags(t *testing.T) {
	expected_result := new(ArgConfig)
	expected_result.SecurityGroupId = "sg-4321zasd"
	expected_result.From = 80
	expected_result.To = 85
	expected_result.Protocol = "tcp"
	expected_result.ThisEc2Instance = false
	expected_result.AutoScalingGroupName = "some-asg"

	set1 := flag.NewFlagSet("test1", 0)
	set1.String("s", "sg-4321zasd", "doc")
	set1.String("f", "80", "doc")
	set1.String("t", "85", "doc")
	set1.String("p", "tcp", "doc")
	set1.String("a", "some-asg", "doc")
	context1 := cli.NewContext(nil, set1, nil)

	result := parseFlags(context1)

	if reflect.DeepEqual(result, expected_result) != true {
		t.Errorf("Expected %+v, got %+v\n", expected_result, result)
	}
}
