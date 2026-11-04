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
	"os"
	"path/filepath"
	"testing"
)

func TestSampleDataSize(t *testing.T) {
	p := project{Dir: t.TempDir()}
	if err := os.WriteFile(filepath.Join(p.Dir, "1.in"), []byte("input"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(p.Dir, "1.ans"), []byte("output!"), 0644); err != nil {
		t.Fatal(err)
	}
	if got, want := sampleDataSize(p, "1"), int64(len("input")+len("output!")); got != want {
		t.Fatalf("sampleDataSize = %d, want %d", got, want)
	}
}
