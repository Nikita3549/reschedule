package lessons

import (
	"reflect"
	"testing"
)

func TestChangesVisible(t *testing.T) {
	shown := testLesson("shown")
	hidden := testLesson("hidden")
	hidden.Hide = true
	unhidden := hidden
	unhidden.Hide = false

	got := Changes{
		Added:    []Lesson{shown, hidden},
		Canceled: []Lesson{hidden, shown},
		Changed: []Change{
			{Old: shown, New: shown},
			{Old: shown, New: hidden},
			{Old: hidden, New: unhidden},
		},
	}.Visible()

	want := Changes{
		Added:    []Lesson{shown},
		Canceled: []Lesson{shown},
		Changed: []Change{
			{Old: shown, New: shown},
			{Old: hidden, New: unhidden},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Visible() = %+v, want %+v", got, want)
	}
}

func TestChangesVisibleAllHidden(t *testing.T) {
	hidden := testLesson("hidden")
	hidden.Hide = true

	got := Changes{Added: []Lesson{hidden}}.Visible()
	if !got.Empty() {
		t.Errorf("Visible() = %+v, want empty", got)
	}
}
