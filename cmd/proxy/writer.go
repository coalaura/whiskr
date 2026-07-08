package main

import "net/http"

type FlushingWriter struct {
	wr http.ResponseWriter
	fl http.Flusher
}

func (fw FlushingWriter) Write(p []byte) (int, error) {
	n, err := fw.wr.Write(p)
	if fw.fl != nil {
		fw.fl.Flush()
	}

	return n, err
}
