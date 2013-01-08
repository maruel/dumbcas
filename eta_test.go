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
	"testing"
)

func equals(a []float64, b []float64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestEtaLinear1(t *testing.T) {
	derivate := 1.
	t.Parallel()
	tb := MakeTB(t)
	e := MakeEta(0.66, 10)
	expected := []float64{0, derivate, derivate, derivate, derivate, derivate, derivate, derivate, derivate, derivate, derivate, derivate, derivate, derivate, derivate}
	actual := []float64{}
	for i := 0; i < 15; i++ {
		e.AddExplicit(float64(i+20), int64(i+10)*1000000000)
		actual = append(actual, e.Get())
	}
	tb.Assertf(equals(actual, expected), "Difference:\n%#v\n%#v", actual, expected)
}

func TestEtaLinear2(t *testing.T) {
	derivate := 2.
	t.Parallel()
	tb := MakeTB(t)
	e := MakeEta(0.66, 10)
	expected := []float64{0, derivate, derivate, derivate, derivate, derivate, derivate, derivate, derivate, derivate, derivate, derivate, derivate, derivate, derivate}
	actual := []float64{}
	for i := 0; i < 15; i++ {
		e.AddExplicit(float64(2*i+20), int64(i+10)*1000000000)
		actual = append(actual, e.Get())
	}
	tb.Assertf(equals(actual, expected), "Difference:\n%#v\n%#v", actual, expected)
}

func TestEtaLinear2dot5(t *testing.T) {
	derivate := 2.5
	t.Parallel()
	tb := MakeTB(t)
	e := MakeEta(0.66, 10)
	expected := []float64{0, derivate, derivate, derivate, derivate, derivate, derivate, derivate, derivate, derivate, derivate, derivate, derivate, derivate, derivate}
	actual := []float64{}
	for i := 0; i < 15; i++ {
		e.AddExplicit(2.5*float64(i)+20, int64(i+10)*1000000000)
		actual = append(actual, e.Get())
	}
	tb.Assertf(equals(actual, expected), "Difference:\n%#v\n%#v", actual, expected)
}

func TestEtaExpo1(t *testing.T) {
	t.Parallel()
	tb := MakeTB(t)
	e := MakeEta(0.66, 10)

	// TODO(maruel): Totally untested.
	expected := []float64{0, 1, 1.6800000000000002, 2.8087999999999997, 4.233808, 5.8543132799999995, 7.6038467648, 9.438538864768, 11.32943565074688, 13.25742752949294, 15.25742752949294, 17.25742752949294, 19.25742752949294, 21.257427529492944, 23.257427529492944}
	actual := []float64{}
	for i := 0; i < 15; i++ {
		e.AddExplicit(float64(i*i)+20, int64(i+10)*1000000000)
		actual = append(actual, e.Get())
	}
	tb.Assertf(equals(actual, expected), "Difference:\n%#v\n%#v", actual, expected)
}

func TestEtaExpo2(t *testing.T) {
	t.Parallel()
	tb := MakeTB(t)
	e := MakeEta(0.33, 10)

	// TODO(maruel): Totally untested.
	expected := []float64{0, 1, 2.34, 4.122199999999999, 6.050325999999999, 8.026607579999999, 10.018780501399998, 12.016197565461999, 14.015345196602459, 16.01506391487881, 18.01506391487881, 20.01506391487881, 22.01506391487881, 24.015063914878812, 26.01506391487881}
	actual := []float64{}
	for i := 0; i < 15; i++ {
		e.AddExplicit(float64(i*i)+20, int64(i+10)*1000000000)
		actual = append(actual, e.Get())
	}
	tb.Assertf(equals(actual, expected), "Difference:\n%#v\n%#v", actual, expected)
}
