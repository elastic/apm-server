// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package pubsub

import "encoding/json"

// SubscriberPosition holds information for the subscriber to resume after the
// recently observed sampled trace IDs.
//
// The zero value is valid, and can be used to subscribe to all sampled trace IDs.
type SubscriberPosition struct {
	// observedSeqnos maps index names to its greatest observed _seq_no.
	observedSeqnos map[string]int64
}

// MarshalJSON marshals the subscriber position as JSON, for persistence.
func (p SubscriberPosition) MarshalJSON() ([]byte, error) {
	return json.Marshal(p.observedSeqnos)
}

// UnmarshalJSON unmarshals the subscriber position from JSON.
func (p *SubscriberPosition) UnmarshalJSON(data []byte) error {
	return json.Unmarshal(data, &p.observedSeqnos)
}

func copyPosition(pos SubscriberPosition) SubscriberPosition {
	observedSeqnos := make(map[string]int64, len(pos.observedSeqnos))
	for index, seqno := range pos.observedSeqnos {
		observedSeqnos[index] = seqno
	}
	pos.observedSeqnos = observedSeqnos
	return pos
}
