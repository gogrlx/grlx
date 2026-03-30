package serve

import (
	"compress/gzip"
	"io"
	"net/http"
	"strings"
	"sync"
)

var gzipPool = sync.Pool{
	New: func() any {
		w, _ := gzip.NewWriterLevel(io.Discard, gzip.BestSpeed)
		return w
	},
}

// compressibleTypes lists MIME types that benefit from compression.
// Pre-hashed assets (JS/CSS/fonts) are already small via minification,
// but gzip still saves ~60-70% on the wire.
var compressibleTypes = map[string]bool{
	"text/html":                 true,
	"text/css":                  true,
	"text/plain":                true,
	"text/javascript":           true,
	"application/javascript":    true,
	"application/json":          true,
	"application/xml":           true,
	"image/svg+xml":             true,
	"application/manifest+json": true,
}

type gzipResponseWriter struct {
	http.ResponseWriter
	writer     *gzip.Writer
	statusCode int
	sniffBuf   []byte
	sniffDone  bool
}

func (g *gzipResponseWriter) WriteHeader(code int) {
	g.statusCode = code
	// Don't write header yet — wait for sniff to decide Content-Type.
}

func (g *gzipResponseWriter) Write(b []byte) (int, error) {
	if !g.sniffDone {
		g.sniffBuf = append(g.sniffBuf, b...)
		if len(g.sniffBuf) < 512 {
			// Keep buffering until we have enough to sniff.
			return len(b), nil
		}
		return len(b), g.flush()
	}
	if g.writer != nil {
		return g.writer.Write(b)
	}
	return g.ResponseWriter.Write(b)
}

func (g *gzipResponseWriter) flush() error {
	g.sniffDone = true

	ct := g.ResponseWriter.Header().Get("Content-Type")
	if ct == "" {
		ct = http.DetectContentType(g.sniffBuf)
	}
	// Strip parameters (e.g., "; charset=utf-8").
	base := ct
	if i := strings.Index(base, ";"); i >= 0 {
		base = base[:i]
	}
	base = strings.TrimSpace(base)

	if compressibleTypes[base] {
		g.ResponseWriter.Header().Set("Content-Encoding", "gzip")
		g.ResponseWriter.Header().Del("Content-Length")
		if g.statusCode == 0 {
			g.statusCode = http.StatusOK
		}
		g.ResponseWriter.WriteHeader(g.statusCode)
		g.writer.Reset(g.ResponseWriter)
		_, err := g.writer.Write(g.sniffBuf)
		return err
	}

	// Not compressible — write raw.
	if g.statusCode == 0 {
		g.statusCode = http.StatusOK
	}
	g.ResponseWriter.WriteHeader(g.statusCode)
	_, err := g.ResponseWriter.Write(g.sniffBuf)
	return err
}

func (g *gzipResponseWriter) Close() error {
	if !g.sniffDone && len(g.sniffBuf) > 0 {
		if err := g.flush(); err != nil {
			return err
		}
	}
	if g.writer != nil {
		return g.writer.Close()
	}
	return nil
}

// WithGzip wraps a handler to gzip-compress responses when the client
// supports it and the content type is compressible.
func WithGzip(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			next.ServeHTTP(w, r)
			return
		}

		gz := gzipPool.Get().(*gzip.Writer)
		defer gzipPool.Put(gz)

		grw := &gzipResponseWriter{
			ResponseWriter: w,
			writer:         gz,
		}
		defer grw.Close()

		// Prevent upstream from setting Vary if we handle it.
		w.Header().Add("Vary", "Accept-Encoding")

		next.ServeHTTP(grw, r)
	})
}
