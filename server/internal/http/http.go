package http

import "index/suffixarray"

type Handler interface {
	// TODO: add specification
	ServeHTTP(ResponseWriter, *Request)
}

type muxEntry struct {
	h       Handler
	pattern string
}

type ServeMux struct {
	m map[string]muxEntry
	s *suffixarray.Index
}

func NewServeMux() *ServeMux {
	return new(ServeMux)
}

func (mux *ServeMux) HandleFunc(pattern string, handler func(ResponseWriter, *Request)) {
}
