// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package labels

import (
	"encoding/binary"
	"io"
	"math"
	"sort"

	"github.com/elastic/apm-data/model/modelpb"
)

type AggregatedGlobalLabels struct {
	labelKeys        []string
	Labels           modelpb.Labels
	numericLabelKeys []string
	NumericLabels    modelpb.NumericLabels
}

func (a *AggregatedGlobalLabels) Write(w io.Writer) {
	for _, key := range a.labelKeys {
		label := a.Labels[key]
		io.WriteString(w, key)
		if label.Value != "" {
			io.WriteString(w, label.Value)
			continue
		}
		for _, v := range label.Values {
			io.WriteString(w, v)
		}
	}
	for _, key := range a.numericLabelKeys {
		label := a.NumericLabels[key]
		io.WriteString(w, key)
		if label.Value != 0 {
			var b [8]byte
			binary.LittleEndian.PutUint64(b[:], math.Float64bits(label.Value))
			w.Write(b[:])
			continue
		}
		for _, v := range label.Values {
			var b [8]byte
			binary.LittleEndian.PutUint64(b[:], math.Float64bits(v))
			w.Write(b[:])
		}
	}
}

func (a *AggregatedGlobalLabels) Read(event *modelpb.APMEvent) {
	// Remove global labels for RUM services to avoid explosion of metric groups
	// to track for servicetxmetrics.
	// For consistency, this will remove labels for other aggregated metrics as well.
	switch event.GetAgent().GetName() {
	case "rum-js", "js-base", "android/java", "iOS/swift":
		return
	}

	for k, v := range event.Labels {
		if !v.Global {
			continue
		}
		if a.Labels == nil {
			a.Labels = make(modelpb.Labels)
		}
		if len(v.Values) > 0 {
			a.Labels.SetSlice(k, v.Values)
		} else {
			a.Labels.Set(k, v.Value)
		}
		a.labelKeys = append(a.labelKeys, k)
	}
	for k, v := range event.NumericLabels {
		if !v.Global {
			continue
		}
		if a.NumericLabels == nil {
			a.NumericLabels = make(modelpb.NumericLabels)
		}
		if len(v.Values) > 0 {
			a.NumericLabels.SetSlice(k, v.Values)
		} else {
			a.NumericLabels.Set(k, v.Value)
		}
		a.numericLabelKeys = append(a.numericLabelKeys, k)
	}
	sort.Strings(a.labelKeys)
	sort.Strings(a.numericLabelKeys)
}

func (a *AggregatedGlobalLabels) Equals(x *AggregatedGlobalLabels) bool {
	return equalLabels(a.Labels, x.Labels) && equalNumericLabels(a.NumericLabels, x.NumericLabels)
}

// equalLabels returns true if the labels are equal. The Global property is
// ignored since only global labels are compared.
func equalLabels(l, labels modelpb.Labels) bool {
	if len(l) != len(labels) {
		return false
	}
	for key, localV := range l {
		v, ok := labels[key]
		if !ok {
			return false
		}
		// If the slice value is set, ignore the Value field.
		if len(v.Values) == 0 && v.Value != localV.Value {
			return false
		}
		if len(v.Values) != len(localV.Values) {
			return false
		}
		for i, value := range v.Values {
			if localV.Values[i] != value {
				return false
			}
		}
	}
	return true
}

// equalNumericLabels returns true if the labels are equal. The Global property
// is ignored since only global labels are compared.
func equalNumericLabels(l, labels modelpb.NumericLabels) bool {
	if len(l) != len(labels) {
		return false
	}
	for key, localV := range l {
		v, ok := labels[key]
		if !ok {
			return false
		}
		// If the slice value is set, ignore the Value field.
		if len(v.Values) == 0 && v.Value != localV.Value {
			return false
		}
		if len(v.Values) != len(localV.Values) {
			return false
		}
		for i, value := range v.Values {
			if localV.Values[i] != value {
				return false
			}
		}
	}
	return true
}
