package lessons

func Diff(stored, fetched []Lesson) Changes {
	storedByID := make(map[string]Lesson, len(stored))
	for _, l := range stored {
		storedByID[l.ID] = l
	}

	fetchedByID := make(map[string]Lesson, len(fetched))
	for _, l := range fetched {
		fetchedByID[l.ID] = l
	}

	var changes Changes
	for _, l := range fetched {
		if _, ok := storedByID[l.ID]; !ok {
			changes.Added = append(changes.Added, l)
		}
	}
	for _, l := range stored {
		fresh, ok := fetchedByID[l.ID]
		if !ok {
			changes.Canceled = append(changes.Canceled, l)
			continue
		}
		if !l.Equal(fresh) {
			changes.Changed = append(changes.Changed, Change{
				Old: l,
				New: fresh,
			})
		}
	}

	return changes
}

func (l Lesson) Equal(o Lesson) bool {
	return l.ID == o.ID &&
		l.StartAt.Equal(o.StartAt) &&
		l.EndAt.Equal(o.EndAt) &&
		l.Subject == o.Subject &&
		l.Kind == o.Kind &&
		l.Room == o.Room &&
		l.Location == o.Location &&
		l.Teacher == o.Teacher
}
