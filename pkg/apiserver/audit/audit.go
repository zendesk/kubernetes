/*
Copyright 2016 The Kubernetes Authors All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package audit

import (
	"bufio"
	"net"
	"net/http"

	"github.com/golang/glog"
	"github.com/pborman/uuid"

	"k8s.io/kubernetes/pkg/api"
	utilnet "k8s.io/kubernetes/pkg/util/net"
)

// auditResponseWriter implements http.ResponseWriter interface.
type auditResponseWriter struct {
	http.ResponseWriter
	id string
}

func (a *auditResponseWriter) WriteHeader(code int) {
	glog.Infof("AUDIT: id=%q response=\"%d\"", a.id, code)
	a.ResponseWriter.WriteHeader(code)
}

var _ http.ResponseWriter = &auditResponseWriter{}

// fancyResponseWriterDelegator implements http.CloseNotifier, http.Flusher and
// http.Hijacker which are needed to make certain http operation (eg. watch, rsh, etc)
// working.
type fancyResponseWriterDelegator struct {
	*auditResponseWriter
}

func (f *fancyResponseWriterDelegator) CloseNotify() <-chan bool {
	return f.ResponseWriter.(http.CloseNotifier).CloseNotify()
}

func (f *fancyResponseWriterDelegator) Flush() {
	f.ResponseWriter.(http.Flusher).Flush()
}

func (f *fancyResponseWriterDelegator) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return f.ResponseWriter.(http.Hijacker).Hijack()
}

var _ http.CloseNotifier = &fancyResponseWriterDelegator{}
var _ http.Flusher = &fancyResponseWriterDelegator{}
var _ http.Hijacker = &fancyResponseWriterDelegator{}

// WithAudit is responsible for logging audit information for all the
// request coming to server. Each audit log contains two entries:
// 1. the request line containing:
//    - unique id allowing to match the response line (see 2)
//    - source ip of the request
//    - HTTP method being invoked
//    - original user invoking the operation
//    - impersonated user for the operation
//    - namespace of the request or <none>
//    - uri is the full URI as requested
// 2. the response line containing the unique id from 1 and response code
func WithAudit(handler http.Handler, requestContextMapper api.RequestContextMapper) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		ctx, _ := requestContextMapper.Get(req)
		user, _ := api.UserFrom(ctx)
		asuser := req.Header.Get("Impersonate-User")
		if len(asuser) == 0 {
			asuser = "<self>"
		}
		namespace := api.NamespaceValue(ctx)
		if len(namespace) == 0 {
			namespace = "<none>"
		}
		id := uuid.NewRandom().String()

		glog.Infof("AUDIT: id=%q ip=%q method=%q user=%q as=%q namespace=%q uri=%q",
			id, utilnet.GetClientIP(req), req.Method, user.GetName(), asuser, namespace, req.URL)
		handler.ServeHTTP(constructResponseWriter(w, id), req)
	})
}

func constructResponseWriter(responseWriter http.ResponseWriter, id string) http.ResponseWriter {
	delegate := &auditResponseWriter{ResponseWriter: responseWriter, id: id}
	// check if the ResponseWriter we're wrapping is the fancy one we need
	// or if the basic is sufficient
	_, cn := responseWriter.(http.CloseNotifier)
	_, fl := responseWriter.(http.Flusher)
	_, hj := responseWriter.(http.Hijacker)
	if cn && fl && hj {
		return &fancyResponseWriterDelegator{delegate}
	}
	return delegate
}
