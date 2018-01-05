package utility

import "net/http"

type recordingResponseWriter struct {
	http.ResponseWriter
	Code int
}

func NewRecordingResponseWriter(w http.ResponseWriter) *recordingResponseWriter {
	return &recordingResponseWriter{w, http.StatusOK}
}

func (lrw *recordingResponseWriter) WriteHeader(code int) {
	lrw.ResponseWriter.WriteHeader(code)
	lrw.Code = code
}
