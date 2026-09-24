package main

import (
	"bytes"
	"crypto/subtle"
	"fmt"
	"html/template"
	"image"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultFontSize = 64
	maxFontSize     = 512
	maxBodyBytes    = 4 << 20
)

type printerServer struct {
	password string
	print    func(image.Image) error
	page     *template.Template
}

func newPrinterServer(password string, print func(image.Image) error) *printerServer {
	return &printerServer{
		password: password,
		print:    print,
		page:     template.Must(template.New("index").Parse(indexHTML)),
	}
}

func serve(addr, printerAddress, password string) error {
	handler := newPrinterServer(password, func(img image.Image) error {
		return printImage(printerAddress, img)
	})
	server := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       time.Minute,
		WriteTimeout:      2 * time.Minute,
	}
	fmt.Fprintf(os.Stderr, "listening on %s\n", addr)
	return server.ListenAndServe()
}

func (s *printerServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/":
		s.handleIndex(w, r)
	case r.Method == http.MethodPost && r.URL.Path == "/print":
		// Read the body before replying. Browsers send Expect: 100-continue and
		// will not accept a response until the body has been accepted.
		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxBodyBytes))
		if err != nil {
			http.Error(w, "body is too large", http.StatusBadRequest)
			return
		}
		if !s.authorized(r) {
			// A failed Bearer token must not include WWW-Authenticate. Browsers
			// treat that header as a Basic challenge and leave fetch() waiting
			// on a sign-in dialog.
			if r.Header.Get("Authorization") == "" {
				w.Header().Set("WWW-Authenticate", `Basic realm="lt200b", charset="UTF-8"`)
			}
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if !sameOrigin(r) {
			http.Error(w, "cross-origin request rejected", http.StatusForbidden)
			return
		}
		s.handlePrint(w, r, body)
	default:
		http.NotFound(w, r)
	}
}

func (s *printerServer) handleIndex(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	data := struct{ RequirePassword bool }{RequirePassword: s.password != ""}
	if err := s.page.Execute(w, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *printerServer) handlePrint(w http.ResponseWriter, r *http.Request, body []byte) {
	fontSize, err := parseFontSize(r.URL.Query().Get("font-size"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	img, err := imageFromBody(body, fontSize)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := s.print(img); err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	fmt.Fprintln(w, "Printed.")
}

func (s *printerServer) authorized(r *http.Request) bool {
	if s.password == "" {
		return true
	}
	if got := bearerToken(r); got != "" && subtleEqual(got, s.password) {
		return true
	}
	_, password, ok := r.BasicAuth()
	return ok && subtleEqual(password, s.password)
}

func bearerToken(r *http.Request) string {
	value := r.Header.Get("Authorization")
	const prefix = "Bearer "
	if len(value) < len(prefix) || !strings.EqualFold(value[:len(prefix)], prefix) {
		return ""
	}
	return strings.TrimSpace(value[len(prefix):])
}

func subtleEqual(got, want string) bool {
	if len(got) != len(want) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(got), []byte(want)) == 1
}

func sameOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	parsed, err := url.Parse(origin)
	if err != nil {
		return false
	}
	return strings.EqualFold(parsed.Host, r.Host)
}

func parseFontSize(value string) (int, error) {
	if value == "" {
		return defaultFontSize, nil
	}
	size, err := strconv.Atoi(value)
	if err != nil || size < 1 || size > maxFontSize {
		return 0, fmt.Errorf("font-size must be an integer from 1 to %d", maxFontSize)
	}
	return size, nil
}

func imageFromBody(body []byte, fontSize int) (image.Image, error) {
	if len(bytes.TrimSpace(body)) == 0 {
		return nil, fmt.Errorf("body is empty")
	}
	if isImage(body) {
		img, _, err := image.Decode(bytes.NewReader(body))
		if err != nil {
			return nil, fmt.Errorf("could not read image: %w", err)
		}
		return img, nil
	}
	text := strings.TrimPrefix(string(body), "\uFEFF")
	return createTextImage(strings.TrimRight(text, "\r\n"), fontSize)
}

func isImage(body []byte) bool {
	if len(body) >= 8 && bytes.Equal(body[:8], []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}) {
		return true
	}
	if len(body) >= 3 && body[0] == 0xff && body[1] == 0xd8 && body[2] == 0xff {
		return true
	}
	return len(body) >= 6 && (bytes.HasPrefix(body, []byte("GIF87a")) || bytes.HasPrefix(body, []byte("GIF89a")))
}

const indexHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>LetraTag</title>
<style>
  :root { color-scheme: light dark; }
  body { font-family: ui-sans-serif, system-ui, sans-serif; margin: 0; }
  main { max-width: 28rem; margin: 0 auto; padding: 2rem 1rem 3rem; }
  h1 { font-size: 1.4rem; margin: 0 0 0.4rem; }
  .hint { margin: 0; opacity: 0.75; }
  label { display: block; margin-top: 1rem; font-weight: 600; }
  textarea, input { box-sizing: border-box; width: 100%; margin-top: 0.35rem; font: inherit; font-size: 1rem; }
  textarea { min-height: 6rem; }
  button { margin-top: 1.25rem; font: inherit; font-size: 1rem; padding: 0.55rem 1rem; }
  #status { min-height: 1.4rem; }
  #status.error { color: #b42318; }
  @media (prefers-color-scheme: dark) {
    #status.error { color: #ffb4a8; }
  }
</style>
</head>
<body>
<main>
  <h1>LetraTag</h1>
  <p class="hint">Print text, or choose an image. An image is sent instead of the text.</p>
  <form id="form">
    <label>Text
      <textarea name="text" autofocus></textarea>
    </label>
    <label>Image
      <input name="image" type="file" accept="image/png,image/jpeg,image/gif">
    </label>
    <label>Font size
      <input name="font-size" type="number" min="1" max="512" value="64">
    </label>
    {{if .RequirePassword}}
    <label>Password
      <input name="password" type="password" autocomplete="current-password" required>
    </label>
    {{end}}
    <button type="button">Print</button>
    <p id="status" role="status"></p>
  </form>
</main>
<script>
const form = document.getElementById("form");
const status = document.getElementById("status");
const button = form.querySelector("button");
button.addEventListener("click", async () => {
  if (!form.reportValidity()) return;
  status.className = "";
  status.textContent = "Printing…";
  button.disabled = true;
  const data = new FormData(form);
  const file = data.get("image");
  const headers = {};
  const password = data.get("password");
  if (password) headers["Authorization"] = "Bearer " + password;
  let body;
  if (file instanceof File && file.size > 0) {
    body = await file.arrayBuffer();
  } else {
    body = data.get("text");
  }
  try {
    const response = await fetch("/print?font-size=" + encodeURIComponent(data.get("font-size") || "64"), {
      method: "POST",
      headers,
      body,
      credentials: "same-origin",
    });
    const message = (await response.text()).trim();
    status.textContent = message || (response.ok ? "Printed." : "Print failed");
    status.className = response.ok ? "" : "error";
  } catch (err) {
    status.textContent = err.message;
    status.className = "error";
  } finally {
    button.disabled = false;
  }
});
</script>
</body>
</html>
`
