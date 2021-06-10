// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package config

// Receiver is the configuration of a receiver. Specific receivers must implement this
// interface and will typically embed ReceiverSettings struct or a struct that extends it.
// Embedded validatable will force each receiver to implement Validate() function.
type Receiver interface {
	identifiable
	validatable
}

// Receivers is a map of names to Receivers.
type Receivers map[ComponentID]Receiver

// ReceiverSettings defines common settings for a receiver configuration.
// Specific receivers can embed this struct and extend it with more fields if needed.
// It is highly recommended to "override" the Validate() function.
// When embedded in the receiver config it must be with `mapstructure:",squash"` tag.
type ReceiverSettings struct {
	id ComponentID `mapstructure:"-"`
}

// NewReceiverSettings return a new ReceiverSettings with the given ComponentID.
func NewReceiverSettings(id ComponentID) ReceiverSettings {
	return ReceiverSettings{id: ComponentID{typeVal: id.Type(), nameVal: id.Name()}}
}

var _ Receiver = (*ReceiverSettings)(nil)

// ID returns the receiver ComponentID.
func (rs *ReceiverSettings) ID() ComponentID {
	return rs.id
}

// SetIDName sets the receiver name.
func (rs *ReceiverSettings) SetIDName(idName string) {
	rs.id.nameVal = idName
}

// Validate validates the configuration and returns an error if invalid.
func (rs *ReceiverSettings) Validate() error {
	return nil
}
