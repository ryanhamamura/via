package via

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
)

func TestStatic(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "sub"), 0755)
	os.WriteFile(filepath.Join(dir, "hello.txt"), []byte("hello world"), 0644)
	os.WriteFile(filepath.Join(dir, "sub", "nested.txt"), []byte("nested"), 0644)

	v := New()
	v.Static("/assets/", dir)

	t.Run("serves file", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/assets/hello.txt", nil)
		v.mux.ServeHTTP(w, r)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "hello world", w.Body.String())
	})

	t.Run("serves nested file", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/assets/sub/nested.txt", nil)
		v.mux.ServeHTTP(w, r)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "nested", w.Body.String())
	})

	t.Run("directory listing returns 404", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/assets/", nil)
		v.mux.ServeHTTP(w, r)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("subdirectory listing returns 404", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/assets/sub/", nil)
		v.mux.ServeHTTP(w, r)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("missing file returns 404", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/assets/nope.txt", nil)
		v.mux.ServeHTTP(w, r)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

func TestStaticAutoSlash(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "ok.txt"), []byte("ok"), 0644)

	v := New()
	v.Static("/files", dir) // no trailing slash

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/files/ok.txt", nil)
	v.mux.ServeHTTP(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "ok", w.Body.String())
}

func TestStaticFS(t *testing.T) {
	fsys := fstest.MapFS{
		"style.css":        {Data: []byte("body{}")},
		"js/app.js":        {Data: []byte("console.log('hi')")},
	}

	v := New()
	v.StaticFS("/static/", fsys)

	t.Run("serves file", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/static/style.css", nil)
		v.mux.ServeHTTP(w, r)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "body{}", w.Body.String())
	})

	t.Run("serves nested file", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/static/js/app.js", nil)
		v.mux.ServeHTTP(w, r)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "console.log('hi')", w.Body.String())
	})

	t.Run("directory listing returns 404", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/static/", nil)
		v.mux.ServeHTTP(w, r)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("missing file returns 404", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/static/nope.css", nil)
		v.mux.ServeHTTP(w, r)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

func TestStaticFSAutoSlash(t *testing.T) {
	fsys := fstest.MapFS{
		"ok.txt": {Data: []byte("ok")},
	}

	v := New()
	v.StaticFS("/embed", fsys) // no trailing slash

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/embed/ok.txt", nil)
	v.mux.ServeHTTP(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "ok", w.Body.String())
}

// Verify StaticFS accepts the fs.FS interface (compile-time check).
var _ fs.FS = fstest.MapFS{}
