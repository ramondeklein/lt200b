package main

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPostText(t *testing.T) {
	var got image.Image
	server := newPrinterServer("", func(img image.Image) error {
		got = img
		return nil
	})
	request := httptest.NewRequest(http.MethodPost, "/print?font-size=48", strings.NewReader("Hello"))
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status %d: %s", response.Code, response.Body)
	}
	if got == nil || got.Bounds().Dy() != textImageHeight {
		t.Fatalf("printed image %v", boundsOf(got))
	}
	if !strings.Contains(response.Body.String(), "Printed.") {
		t.Fatalf("body %q", response.Body.String())
	}
}

func TestFontSizeChangesTextWidth(t *testing.T) {
	small := postText(t, "/print?font-size=16", "W")
	large := postText(t, "/print?font-size=80", "W")
	if large.Bounds().Dx() <= small.Bounds().Dx() {
		t.Fatalf("large %d, small %d", large.Bounds().Dx(), small.Bounds().Dx())
	}
}

func TestPostImage(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 3, 2))
	src.Set(0, 0, color.Black)
	var buf bytes.Buffer
	if err := png.Encode(&buf, src); err != nil {
		t.Fatal(err)
	}
	var got image.Image
	server := newPrinterServer("", func(img image.Image) error {
		got = img
		return nil
	})
	request := httptest.NewRequest(http.MethodPost, "/print?font-size=nope", bytes.NewReader(buf.Bytes()))
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("invalid font-size status %d", response.Code)
	}

	request = httptest.NewRequest(http.MethodPost, "/print", bytes.NewReader(buf.Bytes()))
	response = httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status %d: %s", response.Code, response.Body)
	}
	if got == nil || got.Bounds().Dx() != 3 || got.Bounds().Dy() != 2 {
		t.Fatalf("image passed through as %v", boundsOf(got))
	}
}

func TestPostRejectsEmptyBody(t *testing.T) {
	server := newPrinterServer("", func(image.Image) error {
		t.Fatal("print was called")
		return nil
	})
	request := httptest.NewRequest(http.MethodPost, "/print", strings.NewReader(" \n"))
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status %d", response.Code)
	}
}

func TestPassword(t *testing.T) {
	printed := false
	server := newPrinterServer("secret", func(image.Image) error {
		printed = true
		return nil
	})

	denied := httptest.NewRecorder()
	server.ServeHTTP(denied, httptest.NewRequest(http.MethodPost, "/print", strings.NewReader("Hi")))
	if denied.Code != http.StatusUnauthorized || printed {
		t.Fatalf("open post status %d printed %v", denied.Code, printed)
	}
	if denied.Header().Get("WWW-Authenticate") == "" {
		t.Fatal("missing basic challenge")
	}

	wrong := httptest.NewRequest(http.MethodPost, "/print", strings.NewReader("Hi"))
	wrong.Header.Set("Authorization", "Bearer no")
	wrongResponse := httptest.NewRecorder()
	server.ServeHTTP(wrongResponse, wrong)
	if wrongResponse.Code != http.StatusUnauthorized || printed {
		t.Fatalf("wrong bearer status %d", wrongResponse.Code)
	}
	if wrongResponse.Header().Get("WWW-Authenticate") != "" {
		t.Fatal("bearer failure offered a basic challenge")
	}

	bearer := httptest.NewRequest(http.MethodPost, "/print", strings.NewReader("Hi"))
	bearer.Header.Set("Authorization", "Bearer secret")
	bearerResponse := httptest.NewRecorder()
	server.ServeHTTP(bearerResponse, bearer)
	if bearerResponse.Code != http.StatusOK || !printed {
		t.Fatalf("bearer status %d printed %v", bearerResponse.Code, printed)
	}

	printed = false
	basic := httptest.NewRequest(http.MethodPost, "/print", strings.NewReader("Hi"))
	basic.SetBasicAuth("lt200b", "secret")
	basicResponse := httptest.NewRecorder()
	server.ServeHTTP(basicResponse, basic)
	if basicResponse.Code != http.StatusOK || !printed {
		t.Fatalf("basic status %d printed %v", basicResponse.Code, printed)
	}
}

func TestIndexPage(t *testing.T) {
	open := httptest.NewRecorder()
	newPrinterServer("", func(image.Image) error { return nil }).ServeHTTP(open, httptest.NewRequest(http.MethodGet, "/", nil))
	if open.Code != http.StatusOK || !strings.Contains(open.Body.String(), "Print") || strings.Contains(open.Body.String(), `name="password"`) {
		t.Fatalf("open page status %d", open.Code)
	}

	locked := httptest.NewRecorder()
	newPrinterServer("secret", func(image.Image) error { return nil }).ServeHTTP(locked, httptest.NewRequest(http.MethodGet, "/", nil))
	body := locked.Body.String()
	if locked.Code != http.StatusOK || !strings.Contains(body, `name="password"`) || !strings.Contains(body, "Font size") {
		t.Fatalf("locked page missing fields")
	}
}

func TestCrossOriginRejected(t *testing.T) {
	server := newPrinterServer("", func(image.Image) error {
		t.Fatal("print was called")
		return nil
	})
	request := httptest.NewRequest(http.MethodPost, "/print", strings.NewReader("Hi"))
	request.Host = "printer.local:8080"
	request.Header.Set("Origin", "https://evil.example")
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("status %d", response.Code)
	}
}

func TestIsImage(t *testing.T) {
	if isImage([]byte("Hello world")) {
		t.Fatal("text detected as image")
	}
	if !isImage([]byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n', 0}) {
		t.Fatal("png not detected")
	}
	if !isImage([]byte{0xff, 0xd8, 0xff, 0}) {
		t.Fatal("jpeg not detected")
	}
	if !isImage([]byte("GIF89a")) {
		t.Fatal("gif not detected")
	}
}

func postText(t *testing.T, target, text string) image.Image {
	t.Helper()
	var got image.Image
	server := newPrinterServer("", func(img image.Image) error {
		got = img
		return nil
	})
	request := httptest.NewRequest(http.MethodPost, target, strings.NewReader(text))
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("%s status %d: %s", target, response.Code, response.Body)
	}
	return got
}

func boundsOf(img image.Image) image.Rectangle {
	if img == nil {
		return image.Rectangle{}
	}
	return img.Bounds()
}
