// Copyright (C) 2026 魇珩Gr3yPh4ntom
//
// This file is part of OierToolKit.
//
// OierToolKit is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// OierToolKit is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with OierToolKit.  If not, see <https://www.gnu.org/licenses/>.

package main

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestExtractUpdateTarGz(t *testing.T) {
	dir := t.TempDir()
	archive := filepath.Join(dir, "update.tar.gz")
	f, err := os.Create(archive)
	if err != nil {
		t.Fatal(err)
	}
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	for _, name := range []string{"otk", "otk-gui"} {
		body := []byte("new " + name)
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0755, Size: int64(len(body))}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write(body); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	dst := t.TempDir()
	files, err := extractUpdateArchive(archive, dst)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"otk", "otk-gui"} {
		data, err := os.ReadFile(files[name])
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != "new "+name {
			t.Fatalf("%s = %q", name, data)
		}
	}
}

func TestExtractUpdateZip(t *testing.T) {
	dir := t.TempDir()
	archive := filepath.Join(dir, "update.zip")
	f, err := os.Create(archive)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	for _, name := range []string{"otk.exe", "otk-gui.exe"} {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte("new " + name)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	dst := t.TempDir()
	files, err := extractUpdateArchive(archive, dst)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"otk.exe", "otk-gui.exe"} {
		data, err := os.ReadFile(files[name])
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != "new "+name {
			t.Fatalf("%s = %q", name, data)
		}
	}
}

func TestDownloadUpdateProgress(t *testing.T) {
	const body = "hello update"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Length", "12")
		_, _ = io.WriteString(w, body)
	}))
	defer srv.Close()

	dst := filepath.Join(t.TempDir(), "update.bin")
	var done, total int64
	if err := downloadOneUpdate(srv.URL, dst, func(d, n int64) {
		done, total = d, n
	}); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(dst); err != nil || string(data) != body {
		t.Fatalf("downloaded %q, err %v", data, err)
	}
	if done != int64(len(body)) || total != int64(len(body)) {
		t.Fatalf("progress = %d/%d, want %d", done, total, len(body))
	}
}
