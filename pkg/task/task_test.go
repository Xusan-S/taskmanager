package task

import (
	"testing"
	"time"
)

func TestAddTask(t *testing.T) {
	title := "Test Task"
	id := 1

	testCases := []struct {
		name             string
		priority         string
		expectedPriority string
	}{
		{"High priority", PriorityHigh, PriorityHigh},
		{"Medium priority", PriorityMedium, PriorityMedium},
		{"Low priority", PriorityLow, PriorityLow},
		{"Empty priority", "", PriorityMedium}, // Проверка дефолтного
		{"Invalid priority", "urgent", PriorityMedium}, // Проверка дефолтного при невалидном значении (хотя AddTask сам не валидирует, это делает main)
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Используем фиксированное время для предсказуемости
			before := time.Now()
			task := AddTask(id, title, tc.priority)
			after := time.Now()

			if task.ID != id {
				t.Errorf("Expected ID %d, got %d", id, task.ID)
			}
			if task.Title != title {
				t.Errorf("Expected Title '%s', got '%s'", title, task.Title)
			}
			if task.Priority != tc.expectedPriority {
				t.Errorf("Expected Priority '%s', got '%s'", tc.expectedPriority, task.Priority)
			}
			if task.Done {
				t.Errorf("Expected Done to be false, got true")
			}
			// Проверяем, что время создания находится в ожидаемом интервале
			if task.CreatedAt.Before(before) || task.CreatedAt.After(after) {
				t.Errorf("Expected CreatedAt to be between %v and %v, got %v", before, after, task.CreatedAt)
			}
		})
	}
}

func TestTask_AddTaskMethod(t *testing.T) {
	// Тестируем метод AddTask, отмечая его потенциальную избыточность
	initialTask := Task{ID: 10, Title: "Initial", Priority: PriorityLow, Done: true}
	newTitle := "Updated Task"
	newPriority := PriorityHigh

	// Используем фиксированное время
	before := time.Now()
	initialTask.AddTask(newTitle, newPriority) // Вызываем метод
	after := time.Now()

	if initialTask.ID != 10 {
		t.Errorf("Expected ID to remain 10, got %d", initialTask.ID) // Метод не меняет ID
	}
	if initialTask.Title != newTitle {
		t.Errorf("Expected Title to be updated to '%s', got '%s'", newTitle, initialTask.Title)
	}
	if initialTask.Priority != newPriority {
		t.Errorf("Expected Priority to be updated to '%s', got '%s'", newPriority, initialTask.Priority)
	}
	if initialTask.Done {
		t.Errorf("Expected Done to be set to false, got true") // Метод сбрасывает Done
	}
	if initialTask.CreatedAt.Before(before) || initialTask.CreatedAt.After(after) {
		t.Errorf("Expected CreatedAt to be updated between %v and %v, got %v", before, after, initialTask.CreatedAt) // Метод обновляет время
	}
}


func TestPriorityValue(t *testing.T) {
	testCases := []struct {
		priority string
		expected int
	}{
		{PriorityHigh, 3},
		{PriorityMedium, 2},
		{PriorityLow, 1},
		{"", 0},          // Неизвестный приоритет
		{"invalid", 0},   // Неизвестный приоритет
	}

	for _, tc := range testCases {
		task := Task{Priority: tc.priority}
		if value := task.PriorityValue(); value != tc.expected {
			t.Errorf("For priority '%s', expected value %d, got %d", tc.priority, tc.expected, value)
		}
	}
}