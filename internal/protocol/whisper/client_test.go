package whisper_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/odsod/recorder/internal/protocol/whisper"
)

func TestTranscribe_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}

		if err := r.ParseMultipartForm(10 << 20); err != nil { //nolint:gosec // test code
			t.Fatal(err)
		}

		if model := r.FormValue("model"); model != "whisper-1" {
			t.Errorf("expected model whisper-1, got %s", model)
		}
		if rf := r.FormValue("response_format"); rf != "json" {
			t.Errorf("expected response_format json, got %s", rf)
		}

		file, header, err := r.FormFile("file")
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = file.Close() }()

		if header.Filename != "test.wav" {
			t.Errorf("expected filename test.wav, got %s", header.Filename)
		}
		data, _ := io.ReadAll(file)
		if string(data) != "fake wav data" {
			t.Errorf("unexpected file content: %q", string(data))
		}

		resp := map[string]string{"text": " Hello   world "}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	client := whisper.New(srv.Client(), whisper.Config{
		URL:     srv.URL,
		Timeout: 5 * time.Second,
	})

	resp, err := client.Transcribe(context.Background(), whisper.TranscribeRequest{
		WAVData: []byte("fake wav data"), Filename: "test.wav",
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Text != "Hello world" {
		t.Errorf("expected 'Hello world', got %q", resp.Text)
	}
}

func TestTranscribe_NonOKStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("server error"))
	}))
	defer srv.Close()

	client := whisper.New(srv.Client(), whisper.Config{
		URL:     srv.URL,
		Timeout: 5 * time.Second,
	})

	_, err := client.Transcribe(context.Background(), whisper.TranscribeRequest{
		WAVData: []byte("data"), Filename: "f.wav",
	})
	if err == nil {
		t.Fatal("expected error for non-OK status")
	}
	te := &whisper.TranscriptionError{}
	ok := errors.As(err, &te)
	if !ok {
		t.Fatalf("expected TranscriptionError, got %T: %v", err, err)
	}
	if te.StatusCode != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", te.StatusCode)
	}
	if te.Error() != "whisper: status 500" {
		t.Errorf("unexpected error string: %q", te.Error())
	}
}

func TestTranscribe_EmptyText(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]string{"text": "   "}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	client := whisper.New(srv.Client(), whisper.Config{
		URL:     srv.URL,
		Timeout: 5 * time.Second,
	})

	resp, err := client.Transcribe(context.Background(), whisper.TranscribeRequest{
		WAVData: []byte("data"), Filename: "f.wav",
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Text != "" {
		t.Errorf("expected empty string, got %q", resp.Text)
	}
}

func TestTranscribe_WhitespaceNormalization(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]string{"text": "one  two\t\tthree\n\nfour"}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	client := whisper.New(srv.Client(), whisper.Config{
		URL:     srv.URL,
		Timeout: 5 * time.Second,
	})

	resp, err := client.Transcribe(context.Background(), whisper.TranscribeRequest{
		WAVData: []byte("data"), Filename: "f.wav",
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Text != "one two three four" {
		t.Errorf("expected normalized whitespace, got %q", resp.Text)
	}
}
