// Copyright 2009 The Go Authors. All rights reserved.
// Copyright 2012 The Gorilla Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package rpc

import (
	"errors"
	"net/http"
	"strconv"
	"testing"
)

type Service1Request struct {
	A int
	B int
}

type Service1Response struct {
	Result int
}

type Service1 struct {
}

func (t *Service1) Multiply(r *http.Request, req *Service1Request, res *Service1Response) error {
	res.Result = req.A * req.B
	return nil
}

func (t *Service1) MultiplyWithHeaders(r *http.Request, req *Service1Request, res *Service1Response, header http.Header) error {
	cookie := http.Cookie{Name: "mycookie", Value: "delicious"}
	header.Add("Set-Cookie", cookie.String())
	res.Result = req.A * req.B
	return nil
}

type Service2 struct {
}

func TestRegisterService(t *testing.T) {
	var err error
	s := NewServer()
	service1 := new(Service1)
	service2 := new(Service2)

	// Inferred name.
	err = s.RegisterService(service1, "")
	if err != nil {
		t.Errorf("Expected to be registered: Service1")
	} else if !s.HasMethod("Service1.Multiply") {
		t.Errorf("Expected to be registered: Service1.Multiply")
	} else if !s.HasMethod("Service1.MultiplyWithHeaders") {
		t.Errorf("Expected to be registered: Service1.MultiplyWithHeaders")
	}
	// Provided name.
	err = s.RegisterService(service1, "Foo")
	if err != nil {
		t.Errorf("Expected to be registered: Service1 as Foo")
	} else if !s.HasMethod("Foo.Multiply") {
		t.Errorf("Expected to be registered: Foo.Multiply")
	} else if !s.HasMethod("Foo.MultiplyWithHeaders") {
		t.Errorf("Expected to be registered: Foo.MultiplyWithHeaders")
	}
	// No methods.
	err = s.RegisterService(service2, "")
	if err == nil {
		t.Errorf("Expected error on service2")
	}
}

// MockCodec decodes to Service1.Multiply.
type MockCodec struct {
	A, B int
}

func (c MockCodec) NewRequest(r *http.Request) CodecRequest {
	return MockCodecRequest{c.A, c.B, r.URL.Path}
}

type MockCodecRequest struct {
	A, B   int
	method string
}

func (r MockCodecRequest) Method() (string, error) {
	return r.method, nil
}

func (r MockCodecRequest) ReadRequest(args interface{}) error {
	req := args.(*Service1Request)
	req.A, req.B = r.A, r.B
	return nil
}

func (r MockCodecRequest) WriteResponse(w http.ResponseWriter, reply interface{}) {
	res := reply.(*Service1Response)
	w.Write([]byte(strconv.Itoa(res.Result)))
}

func (r MockCodecRequest) WriteError(w http.ResponseWriter, status int, err error) {
	w.WriteHeader(status)
	w.Write([]byte(err.Error()))
}

type MockResponseWriter struct {
	header http.Header
	Status int
	Body   string
}

func NewMockResponseWriter() *MockResponseWriter {
	header := make(http.Header)
	return &MockResponseWriter{header: header}
}

func (w *MockResponseWriter) Header() http.Header {
	return w.header
}

func (w *MockResponseWriter) Write(p []byte) (int, error) {
	w.Body = string(p)
	if w.Status == 0 {
		w.Status = 200
	}
	return len(p), nil
}

func (w *MockResponseWriter) WriteHeader(status int) {
	w.Status = status
}

func TestServeHTTP(t *testing.T) {
	const (
		A = 2
		B = 3
	)
	expected := A * B

	s := NewServer()
	s.RegisterService(new(Service1), "")
	s.RegisterCodec(MockCodec{A, B}, "mock")
	r, err := http.NewRequest("POST", "Service1.Multiply", nil)
	if err != nil {
		t.Fatal(err)
	}
	r.Header.Set("Content-Type", "mock; dummy")
	w := NewMockResponseWriter()
	s.ServeHTTP(w, r)
	if w.Status != 200 {
		t.Errorf("Status was %d, should be 200.", w.Status)
	}
	if w.Body != strconv.Itoa(expected) {
		t.Errorf("Response body was %s, should be %s.", w.Body, strconv.Itoa(expected))
	}

	// Test wrong Content-Type
	r.Header.Set("Content-Type", "invalid")
	w = NewMockResponseWriter()
	s.ServeHTTP(w, r)
	if w.Status != 415 {
		t.Errorf("Status was %d, should be 415.", w.Status)
	}
	if w.Body != "rpc: unrecognized Content-Type: invalid" {
		t.Errorf("Wrong response body.")
	}

	// Test omitted Content-Type; codec should default to the sole registered one.
	r.Header.Del("Content-Type")
	w = NewMockResponseWriter()
	s.ServeHTTP(w, r)
	if w.Status != 200 {
		t.Errorf("Status was %d, should be 200.", w.Status)
	}
	if w.Body != strconv.Itoa(expected) {
		t.Errorf("Response body was %s, should be %s.", w.Body, strconv.Itoa(expected))
	}

	// Test method that sets headers
	r, err = http.NewRequest("POST", "Service1.MultiplyWithHeaders", nil)
	if err != nil {
		t.Fatal(err)
	}
	r.Header.Set("Content-Type", "mock; dummy")
	w = NewMockResponseWriter()
	s.ServeHTTP(w, r)
	if w.Status != 200 {
		t.Errorf("Status was %d, should be 200.", w.Status)
	}
	if w.Body != strconv.Itoa(expected) {
		t.Errorf("Response body was %s, should be %s.", w.Body, strconv.Itoa(expected))
	}
	hval := w.header.Get("Set-Cookie")
	if hval != "mycookie=delicious" {
		t.Errorf("HTT header Set-Cookie was %s, should be mycookie=delicious", hval)
	}
}

func TestInterception(t *testing.T) {
	const (
		A = 2
		B = 3
	)
	expected := A * B

	r2, err := http.NewRequest("POST", "mocked/request", nil)
	if err != nil {
		t.Fatal(err)
	}

	s := NewServer()
	s.RegisterService(new(Service1), "")
	s.RegisterCodec(MockCodec{A, B}, "mock")
	s.RegisterInterceptFunc(func(i *RequestInfo) *http.Request {
		return r2
	})
	s.RegisterValidateRequestFunc(func(info *RequestInfo, v interface{}) error { return nil })
	s.RegisterAfterFunc(func(i *RequestInfo) {
		if i.Request != r2 {
			t.Errorf("Request was %v, should be %v.", i.Request, r2)
		}
	})

	r, err := http.NewRequest("POST", "Service1.Multiply", nil)
	if err != nil {
		t.Fatal(err)
	}
	r.Header.Set("Content-Type", "mock; dummy")
	w := NewMockResponseWriter()
	s.ServeHTTP(w, r)
	if w.Status != 200 {
		t.Errorf("Status was %d, should be 200.", w.Status)
	}
	if w.Body != strconv.Itoa(expected) {
		t.Errorf("Response body was %s, should be %s.", w.Body, strconv.Itoa(expected))
	}
}
func TestValidationSuccessful(t *testing.T) {
	const (
		A = 2
		B = 3

		expected = A * B
	)

	validate := func(info *RequestInfo, v interface{}) error { return nil }

	s := NewServer()
	s.RegisterService(new(Service1), "")
	s.RegisterCodec(MockCodec{A, B}, "mock")
	s.RegisterValidateRequestFunc(validate)

	r, err := http.NewRequest("POST", "Service1.Multiply", nil)
	if err != nil {
		t.Fatal(err)
	}
	r.Header.Set("Content-Type", "mock; dummy")
	w := NewMockResponseWriter()
	s.ServeHTTP(w, r)
	if w.Status != 200 {
		t.Errorf("Status was %d, should be 200.", w.Status)
	}
	if w.Body != strconv.Itoa(expected) {
		t.Errorf("Response body was %s, should be %s.", w.Body, strconv.Itoa(expected))
	}
}

func TestValidationFails(t *testing.T) {
	const expected = "this instance only supports zero values"

	validate := func(r *RequestInfo, v interface{}) error {
		req := v.(*Service1Request)
		if req.A != 0 || req.B != 0 {
			return errors.New(expected)
		}
		return nil
	}

	s := NewServer()
	s.RegisterService(new(Service1), "")
	s.RegisterCodec(MockCodec{1, 2}, "mock")
	s.RegisterValidateRequestFunc(validate)

	r, err := http.NewRequest("POST", "Service1.Multiply", nil)
	if err != nil {
		t.Fatal(err)
	}
	r.Header.Set("Content-Type", "mock; dummy")
	w := NewMockResponseWriter()
	s.ServeHTTP(w, r)
	if w.Status != 400 {
		t.Errorf("Status was %d, should be 200.", w.Status)
	}
	if w.Body != expected {
		t.Errorf("Response body was %s, should be %s.", w.Body, expected)
	}
}
