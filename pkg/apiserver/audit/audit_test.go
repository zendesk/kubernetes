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
	"reflect"
	"testing"
	"io/ioutil"
)

type simpleResponseWriter struct {
	http.ResponseWriter
}

func (*simpleResponseWriter) WriteHeader(code int) {}

type fancyResponseWriter struct {
	simpleResponseWriter
}

func (*fancyResponseWriter) CloseNotify() <-chan bool { return nil }

func (*fancyResponseWriter) Flush() {}

func (*fancyResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) { return nil, nil, nil }

func TestConstructResponseWriter(t *testing.T) {
	out := ioutil.Discard // Need something that's an io.Writer
	actual := constructResponseWriter(&simpleResponseWriter{}, out, "whatever")
	switch v := actual.(type) {
	case *auditResponseWriter:
		break
	default:
		t.Errorf("Expected auditResponseWriter, got %v", reflect.TypeOf(v))
	}

	actual = constructResponseWriter(&fancyResponseWriter{}, out, "whatever")
	switch v := actual.(type) {
	case *fancyResponseWriterDelegator:
		break
	default:
		t.Errorf("Expected fancyResponseWriterDelegator, got %v", reflect.TypeOf(v))
	}
}
