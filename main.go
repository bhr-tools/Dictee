package main

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

//go:embed index.html
var page []byte

//go:embed icon.svg
var icon []byte

const maxUpload = 200 << 20

var errBadInput = errors.New("bad input")

func main() {
	host := flag.String("host", "127.0.0.1", "listen address")
	port := flag.Int("port", 9910, "listen port")
	whisper := flag.String("whisper", "http://127.0.0.1:8080", "whisper-server base URL")
	lang := flag.String("lang", "fr", "transcription language")
	prompt := flag.String("prompt", "Maître, assignation, ordonnance, mise en demeure, conclusions, tribunal judiciaire, article du code civil.", "vocabulary hint passed to whisper")
	flag.Parse()

	busy := make(chan struct{}, 1) // one transcription at a time: whisper saturates the CPU
	client := &http.Client{Timeout: 20 * time.Minute}

	// Served without a password (Caddy exempts /icons/), for home screens and dashboards.
	http.HandleFunc("GET /icons/icon.svg", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/svg+xml")
		w.Write(icon)
	})

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path != "/":
			http.NotFound(w, r)
		case r.Method == http.MethodGet:
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Write(page)
		case r.Method == http.MethodPost:
			busy <- struct{}{}
			defer func() { <-busy }()
			text, err := transcribe(client, *whisper, *lang, *prompt, w, r)
			if errors.Is(err, errBadInput) {
				log.Print(err)
				http.Error(w, "Fichier audio illisible.", http.StatusBadRequest)
				return
			} else if err != nil {
				log.Print(err)
				http.Error(w, "Transcription indisponible.", http.StatusBadGateway)
				return
			}
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			io.WriteString(w, Format(text))
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	srv := &http.Server{Addr: fmt.Sprintf("%s:%d", *host, *port), ReadHeaderTimeout: 10 * time.Second}
	log.Fatal(srv.ListenAndServe())
}

// transcribe converts the uploaded audio to 16 kHz mono WAV and sends it to whisper-server.
func transcribe(c *http.Client, whisper, lang, prompt string, w http.ResponseWriter, r *http.Request) (string, error) {
	r.Body = http.MaxBytesReader(w, r.Body, maxUpload)
	f, _, err := r.FormFile("file")
	if err != nil {
		return "", fmt.Errorf("%w: %v", errBadInput, err)
	}
	defer f.Close()

	dir, err := os.MkdirTemp("", "dictee")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(dir) // audio never outlives the request
	in, out := filepath.Join(dir, "in"), filepath.Join(dir, "out.wav")
	src, err := os.Create(in)
	if err != nil {
		return "", err
	}
	_, err = io.Copy(src, f)
	if cerr := src.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return "", err
	}
	if msg, err := exec.CommandContext(r.Context(), "ffmpeg", "-nostdin", "-v", "error", "-i", in, "-ar", "16000", "-ac", "1", out).CombinedOutput(); err != nil {
		return "", fmt.Errorf("%w: ffmpeg: %v: %s", errBadInput, err, msg)
	}

	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	fw, _ := mw.CreateFormFile("file", "audio.wav")
	wav, err := os.Open(out)
	if err != nil {
		return "", err
	}
	defer wav.Close()
	io.Copy(fw, wav)
	for k, v := range map[string]string{"temperature": "0.0", "response_format": "json", "language": lang, "prompt": prompt} {
		mw.WriteField(k, v)
	}
	mw.Close()

	req, _ := http.NewRequestWithContext(r.Context(), http.MethodPost, whisper+"/inference", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	resp, err := c.Do(req)
	if err != nil {
		return "", fmt.Errorf("whisper-server injoignable: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("whisper-server: %s", resp.Status)
	}
	var res struct {
		Text string `json:"text"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return "", err
	}
	return res.Text, nil
}
