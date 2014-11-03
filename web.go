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
	"fmt"
	"log"
	"net"
	"net/http"

	"github.com/maruel/subcommands"
)

var cmdWeb = &subcommands.Command{
	UsageLine: "web",
	ShortDesc: "starts a web service to access the dumbcas",
	LongDesc:  "Serves each node as a full virtual tree of the archived files.",
	CommandRun: func() subcommands.CommandRun {
		c := &webRun{}
		c.Init()
		c.Flags.IntVar(&c.port, "port", 8010, "port number")
		c.Flags.BoolVar(&c.local, "local", false, "only listed on localhost")
		return c
	},
}

type webRun struct {
	CommonFlags
	port  int
	local bool
}

// Converts an handler to log every HTTP request.
type loggingHandler struct {
	handler http.Handler
	log     *log.Logger
}

type loggingResponseWriter struct {
	http.ResponseWriter
	length int
	status int
}

func (l *loggingResponseWriter) Write(data []byte) (size int, err error) {
	size, err = l.ResponseWriter.Write(data)
	l.length += size
	return
}

func (l *loggingResponseWriter) WriteHeader(status int) {
	l.ResponseWriter.WriteHeader(status)
	l.status = status
}

func (l *loggingHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	lW := &loggingResponseWriter{ResponseWriter: w}
	l.handler.ServeHTTP(lW, r)
	l.log.Printf("%s - %3d %6db %4s %s",
		r.RemoteAddr,
		lW.status,
		lW.length,
		r.Method,
		r.RequestURI)
}

type restricted struct {
	http.Handler
	methods []string
}

// Restricts request to specific methods
func (d restricted) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	for _, method := range d.methods {
		if r.Method == method {
			d.Handler.ServeHTTP(w, r)
			return
		}
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	http.Error(w, "Invalid Method", http.StatusMethodNotAllowed)
	return
}

func restrict(h http.Handler, m ...string) http.Handler {
	return restricted{h, m}
}

func (c *webRun) main(d DumbcasApplication, ready chan<- net.Listener) error {
	if err := c.Parse(d, true); err != nil {
		return err
	}

	serveMux := http.NewServeMux()

	x := http.StripPrefix("/content/retrieve/default", c.cas)
	serveMux.Handle("/content/retrieve/default/", restrict(x, "GET"))
	x = http.StripPrefix("/content/retrieve/nodes", c.nodes)
	serveMux.Handle("/content/retrieve/nodes/", restrict(x, "GET"))
	serveMux.Handle("/", restrict(http.RedirectHandler("/content/retrieve/nodes/", http.StatusFound), "GET"))

	var addr string
	if c.local {
		addr = fmt.Sprintf("localhost:%d", c.port)
	} else {
		addr = fmt.Sprintf(":%d", c.port)
	}
	s := &http.Server{
		Addr:    addr,
		Handler: &loggingHandler{serveMux, d.GetLog()},
	}
	ls, e := net.Listen("tcp", s.Addr)
	if e != nil {
		return e
	}

	_, portStr, _ := net.SplitHostPort(ls.Addr().String())
	d.GetLog().Printf("Serving %s on port %s", c.Root, portStr)

	if ready != nil {
		ready <- ls
	}
	return s.Serve(ls)
}

func (c *webRun) Run(a subcommands.Application, args []string) int {
	if len(args) != 0 {
		fmt.Fprintf(a.GetErr(), "%s: Unsupported arguments.\n", a.GetName())
		return 1
	}
	d := a.(DumbcasApplication)
	if err := c.main(d, nil); err != nil {
		fmt.Fprintf(a.GetErr(), "%s: %s\n", a.GetName(), err)
		return 1
	}
	// This is never executed.
	return 0
}
