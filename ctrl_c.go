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
	"os"
	"os/signal"
	"sync/atomic"
)

// If non-zero, all processing should be interrupted.
var interrupted int32

// Continuously sends true once Ctrl-C was intercepted.
var InterruptedChannel <-chan bool

// Initialize an handler to handle SIGINT, which is normally sent on Ctrl-C.
func HandleCtrlC() {
	chanSignal := make(chan os.Signal)
	chanStop := make(chan bool)
	InterruptedChannel = chanStop

	go func() {
		<-chanSignal
		atomic.StoreInt32(&interrupted, 1)
		for {
			chanStop <- true
		}
	}()

	signal.Notify(chanSignal, os.Interrupt)
}

// Returns true once an interrupt signal was received.
func IsInterrupted() bool {
	return atomic.LoadInt32(&interrupted) != 0
}
