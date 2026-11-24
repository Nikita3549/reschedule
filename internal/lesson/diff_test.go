package lesson

import (
	"reflect"
	"testing"
)

func TestDiff(t *testing.T) {
	kept := testLesson("kept")
	canceled := testLesson("canceled")
	added := testLesson("added")
	fresh := testLesson("fresh")
	old := testLesson("changed")
	moved := old
	moved.Room = ""

	got := Diff(
		[]Lesson{kept, canceled, old},
		[]Lesson{kept, added, fresh, moved},
		map[string]struct{}{added.ID: {}, kept.ID: {}, moved.ID: {}},
	)

	want := Changes{
		NewWeekLessons: []Lesson{fresh},
		Added:          []Lesson{added},
		Canceled:       []Lesson{canceled},
		Changed:        []Change{{Old: old, New: moved}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Diff() = %+v, want %+v", got, want)
	}
}

func TestDiffWithoutWeeks(t *testing.T) {
	kept := testLesson("kept")
	first := testLesson("first")
	second := testLesson("second")

	got := Diff([]Lesson{kept}, []Lesson{kept, first, second}, nil)

	want := Changes{NewWeekLessons: []Lesson{first, second}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Diff() = %+v, want %+v", got, want)
	}
}

func TestDiffEmpty(t *testing.T) {
	if got := Diff(nil, nil, nil); !reflect.DeepEqual(got, Changes{}) {
		t.Errorf("Diff(nil, nil, nil) = %+v, want empty", got)
	}
}
