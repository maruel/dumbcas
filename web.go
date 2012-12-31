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
)

// Converts an handler to log every HTTP request.
type LoggingHandler struct {
	handler http.Handler
	l       *log.Logger
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

func (l *LoggingHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	l_w := &loggingResponseWriter{ResponseWriter: w}
	l.handler.ServeHTTP(l_w, r)
	l.l.Printf("%s - %3d %6db %4s %s",
		r.RemoteAddr,
		l_w.status,
		l_w.length,
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

func Restrict(h http.Handler, m ...string) http.Handler {
	return restricted{h, m}
}

// localRedirect gives a Moved Permanently response.
// It does not convert relative paths to absolute paths like Redirect does.
func localRedirect(w http.ResponseWriter, r *http.Request, newPath string) {
	if q := r.URL.RawQuery; q != "" {
		newPath += "?" + q
	}
	w.Header().Set("Location", newPath)
	w.WriteHeader(http.StatusMovedPermanently)
}

type web struct {
	DefaultCommand
}

var cmdWeb = &web{
	DefaultCommand{
		UsageLine: "web",
		ShortDesc: "starts a web service to access the dumbcas",
		LongDesc:  "Serves each node as a full virtual tree of the archived files.",
		Flag:      GetCommonFlags(),
	},
}

func webMain(d DumbcasApplication, port int, ready chan<- net.Listener) error {
	cas, err := CommonFlag(d, false, true)
	if err != nil {
		return err
	}

	l := d.GetLog()
	nodes, err := LoadNodesTable(Root, cas, l)
	// TODO(maruel): Add back.
	//if !isDir(nodesDir) {
	//	return fmt.Errorf("Please archive something first into %s", Root)
	//}

	serveMux := http.NewServeMux()

	x := http.StripPrefix("/content/retrieve/default", cas)
	serveMux.Handle("/content/retrieve/default/", Restrict(x, "GET"))
	x = http.StripPrefix("/content/retrieve/nodes", nodes)
	serveMux.Handle("/content/retrieve/nodes/", Restrict(x, "GET"))
	serveMux.Handle("/", Restrict(http.RedirectHandler("/content/retrieve/nodes/", http.StatusFound), "GET"))

	s := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: &LoggingHandler{serveMux, l},
	}
	ls, e := net.Listen("tcp", s.Addr)
	if e != nil {
		return e
	}

	_, portStr, _ := net.SplitHostPort(ls.Addr().String())
	l.Printf("Serving %s on port %s", Root, portStr)

	if ready != nil {
		ready <- ls
	}
	return s.Serve(ls)
}

// Flags.
var webPort int

func init() {
	cmdWeb.Flag.IntVar(&webPort, "port", 8010, "port number")
}

func (c *web) Run(a Application, args []string) int {
	if len(args) != 0 {
		fmt.Fprintf(a.GetErr(), "%s: Unsupported arguments.\n", a.GetName())
		return 1
	}
	d := a.(DumbcasApplication)
	if err := webMain(d, webPort, nil); err != nil {
		fmt.Fprintf(a.GetErr(), "%s: %s\n", a.GetName(), err)
		return 1
	}
	// This is never executed.
	return 0
}
