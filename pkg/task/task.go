package task

import (
	//"flag"
	"time"
)

const(
	PriorityHigh = "high"
	PriorityMedium = "medium"
	PriorityLow = "low"
)

type Task struct {
	CreatedAt time.Time
	Title     string
	Priority  string
	ID        int
	Done      bool
}

func (t *Task) AddTask(title string, priority string) {
	t.Title = title
	t.Done = false
	t.CreatedAt = time.Now()
	t.Priority = priority
}

func AddTask(id int, title string, priority string) Task {
	if priority != PriorityHigh && priority != PriorityMedium && priority != PriorityLow {
        priority = PriorityMedium
    }
	return Task{
		ID: 	  id,
		Title:   title,
		Done:    false,
		CreatedAt: time.Now(),
		Priority: priority,
	}
}

func (t *Task) PriorityValue() int {
	switch t.Priority {
	case PriorityHigh:
		return 3
	case PriorityMedium:
		return 2
	case PriorityLow:
		return 1
	default:
		return 0
	}
}