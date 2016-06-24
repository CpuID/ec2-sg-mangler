package main

import (
	"reflect"
	"sort"
	"testing"
)

func TestReconcileIps(t *testing.T) {
	// NOTE: all slices here are sorted by hand, expectation is all results will be sorted as well.
	test_sg_ips := []string{
		"172.31.0.1",
		"172.31.1.1",
		"172.31.2.1",
		"172.31.3.1",
	}
	test_proposed_ips := []string{
		"172.31.0.1",
		"172.31.3.1",
		"172.31.4.1",
		"172.31.5.1",
	}
	expected_result := SgActions{
		Add: []string{
			"172.31.4.1",
			"172.31.5.1",
		},
		Remove: []string{
			"172.31.1.1",
			"172.31.2.1",
		},
	}
	// NOTE: these sorts are not numeric, we would need a custom sorter to do octet level IP address sorts.
	// as long as we are consistent, a non-issue currently.
	sort.Strings(test_sg_ips)
	sort.Strings(test_proposed_ips)
	result := reconcileIps(test_sg_ips, test_proposed_ips)
	if reflect.DeepEqual(result, expected_result) == false {
		t.Errorf("Expected %+v, got %+v\n", expected_result, result)
	}
}

func TestSanitiseIpProtocol(t *testing.T) {
	tests := map[string]string{
		"tcp":  "6",
		"TCP":  "6",
		"udp":  "17",
		"icmp": "1",
		"4":    "4",
	}
	for k, v := range tests {
		result, err := sanitiseIpProtocol(k)
		if err != nil {
			t.Errorf("Error: %s\n", err.Error())
		} else if result != v {
			t.Errorf("Expected %s, got %s\n", v, result)
		}
	}
}
