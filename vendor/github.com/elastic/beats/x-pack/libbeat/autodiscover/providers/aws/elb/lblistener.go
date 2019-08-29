// Licensed to Elasticsearch B.V. under one or more contributor
// license agreements. See the NOTICE file distributed with
// this work for additional information regarding copyright
// ownership. Elasticsearch B.V. licenses this file to you under
// the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package elb

import (
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"

	"github.com/elastic/beats/libbeat/common"
)

// lbListener is a tuple type representing an elasticloadbalancingv2.Listener and its associated elasticloadbalancingv2.LoadBalancer.
type lbListener struct {
	lb       *elasticloadbalancingv2.LoadBalancer
	listener *elasticloadbalancingv2.Listener
}

// toMap converts this lbListener into the form consumed as metadata in the autodiscovery process.
func (l *lbListener) toMap() common.MapStr {
	// We fully spell out listener_arn to avoid confusion with the ARN for the whole ELB
	m := common.MapStr{
		"listener_arn":       l.listener.ListenerArn,
		"load_balancer_arn":  safeStrp(l.lb.LoadBalancerArn),
		"host":               safeStrp(l.lb.DNSName),
		"protocol":           l.listener.Protocol,
		"type":               string(l.lb.Type),
		"scheme":             l.lb.Scheme,
		"availability_zones": l.azStrings(),
		"created":            l.lb.CreatedTime,
		"state":              l.stateMap(),
		"ip_address_type":    string(l.lb.IpAddressType),
		"security_groups":    l.lb.SecurityGroups,
		"vpc_id":             safeStrp(l.lb.VpcId),
		"ssl_policy":         l.listener.SslPolicy,
	}

	if l.listener.Port != nil {
		m["port"] = *l.listener.Port
	}

	return m
}

// safeStrp makes handling AWS *string types easier.
// The AWS lib never returns plain strings, always using pointers, probably for memory efficiency reasons.
// This is a bit odd, because strings are just pointers into byte arrays, however this is the choice they've made.
// This will return the plain version of the given string or an empty string if the pointer is null
func safeStrp(strp *string) string {
	if strp == nil {
		return ""
	}

	return *strp
}

func (l *lbListener) toCloudMap() common.MapStr {
	m := common.MapStr{}

	var azs []string
	for _, az := range l.lb.AvailabilityZones {
		azs = append(azs, *az.ZoneName)
	}
	m["availability_zone"] = azs
	m["provider"] = "aws"

	// The region is just an AZ with the last character removed
	firstAz := azs[0]
	m["region"] = firstAz[:len(firstAz)-2]

	return m
}

// arn returns a globally unique ID. In the case of an lbListener, that would be its listenerArn.
func (l *lbListener) arn() string {
	return *l.listener.ListenerArn
}

// azStrings transforms the weird list of availability zone string pointers to a slice of plain strings.
func (l *lbListener) azStrings() []string {
	azs := l.lb.AvailabilityZones
	res := make([]string, 0, len(azs))
	for _, az := range azs {
		res = append(res, *az.ZoneName)
	}
	return res
}

// stateMap converts the State part of the lb struct into a friendlier map with 'reason' and 'code' fields.
func (l *lbListener) stateMap() (stateMap common.MapStr) {
	state := l.lb.State
	stateMap = common.MapStr{}
	if state.Reason != nil {
		stateMap["reason"] = *state.Reason
	}
	stateMap["code"] = state.Code
	return stateMap
}
