/* Copyright 2012 Marc-Antoine Ruel. Licensed under the Apache License, Version
2.0 (the "License"); you may not use this file except in compliance with the
License.  You may obtain a copy of the License at
http://www.apache.org/licenses/LICENSE-2.0. Unless required by applicable law or
agreed to in writing, software distributed under the License is distributed on
an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express
or implied. See the License for the specific language governing permissions and
limitations under the License. */

package main

import (
	"time"
)

type Eta struct {
	damping    float64
	values     []float64 // Appends a index 0.
	timestamps []int64
}

func MakeEta(damping float64, length int) *Eta {
	return &Eta{
		damping,
		make([]float64, length),
		make([]int64, length),
	}
}

// Inserts the new value at index 0.
func (e *Eta) Add(value float64) {
	e.AddExplicit(value, time.Now().UTC().UnixNano())
}

// |now| is in nano seconds and must not be 0 or negative.
func (e *Eta) AddExplicit(value float64, now int64) {
	copy(e.values[1:], e.values)
	copy(e.timestamps[1:], e.timestamps)
	e.values[0] = value
	e.timestamps[0] = now
}

// Gets the progress per timestamp delta in seconds (and not nanoseconds).
func (e *Eta) Get() float64 {
	derivate := make([]float64, len(e.values)-1)
	max := 0
	for i := 0; i < len(derivate); i++ {
		if e.timestamps[i+1] == 0 {
			break
		}
		// Convert nanoseconds into seconds.
		duration := float64(e.timestamps[i+1]-e.timestamps[i]) / 1000000000.
		derivate[i] = (e.values[i+1] - e.values[i]) / duration
		max = i + 1
	}
	if max == 0 {
		return 0
	}
	// Alpha damp the values going backward.
	damped := derivate[max-1]
	for i := max - 2; i >= 0; i-- {
		damped = e.damping*damped + (derivate[i] * (1 - e.damping))
	}
	/*
		fmt.Printf("derivate: %v\n", derivate)
		fmt.Printf("values: %v\n", e.values)
		fmt.Printf("timestamps: %v\n", e.timestamps)
	*/
	return damped
}
