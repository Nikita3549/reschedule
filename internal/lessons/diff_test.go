package lessons

import (
	"reflect"
	"testing"
)

func TestDiff(t *testing.T) {
	kept := testLesson("kept")
	canceled := testLesson("canceled")
	added := testLesson("added")
	old := testLesson("changed")
	moved := old
	moved.Room = ""

	got := Diff(
		[]Lesson{kept, canceled, old},
		[]Lesson{kept, added, moved},
	)

	want := Changes{
		Added:    []Lesson{added},
		Canceled: []Lesson{canceled},
		Changed:  []Change{{Old: old, New: moved}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Diff() = %+v, want %+v", got, want)
	}
}

func TestDiffEmpty(t *testing.T) {
	if got := Diff(nil, nil); !reflect.DeepEqual(got, Changes{}) {
		t.Errorf("Diff(nil, nil) = %+v, want empty", got)
	}
}
