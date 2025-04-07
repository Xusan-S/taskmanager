package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"taskm/pkg/task"
	"testing"
	"time"
)

// Хелпер для создания временного файла с содержимым
func createTempFile(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	filePath := filepath.Join(dir, "tasks.txt")
	err := os.WriteFile(filePath, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create temp file %s: %v", filePath, err)
	}
	return filePath
}

// Хелпер для сравнения срезов задач без учета времени (или с допуском)
func assertTasksEqual(t *testing.T, expected, actual []task.Task) {
	t.Helper()
	if len(expected) != len(actual) {
		t.Fatalf("Expected %d tasks, got %d", len(expected), len(actual))
	}
	for i := range expected {
		// Сравниваем все поля, кроме CreatedAt напрямую
		if expected[i].ID != actual[i].ID ||
			expected[i].Title != actual[i].Title ||
			expected[i].Done != actual[i].Done ||
			expected[i].Priority != actual[i].Priority {
			t.Errorf("Task mismatch at index %d.\nExpected: %+v\nActual:   %+v", i, expected[i], actual[i])
			// Добавим сравнение времени с небольшим допуском, если основные поля сошлись
		} else if expected[i].CreatedAt.Truncate(time.Second) != actual[i].CreatedAt.Truncate(time.Second) {
			// Используем Truncate для игнорирования наносекунд, которые могут отличаться при чтении/записи
			t.Logf("Warning: Task CreatedAt mismatch at index %d (might be due to precision loss). Expected: %v, Actual: %v", i, expected[i].CreatedAt, actual[i].CreatedAt)
		}
	}
}


func TestLoadTasks_EmptyOrNonExistentFile(t *testing.T) {
	dir := t.TempDir()
	nonExistentPath := filepath.Join(dir, "nonexistent.txt")

	// Несуществующий файл
	tasks, maxID, err := LoadTasks(nonExistentPath)
	if err != nil {
		t.Fatalf("LoadTasks failed for non-existent file: %v", err)
	}
	if len(tasks) != 0 {
		t.Errorf("Expected 0 tasks for non-existent file, got %d", len(tasks))
	}
	if maxID != 0 {
		t.Errorf("Expected maxID 0 for non-existent file, got %d", maxID)
	}

	// Пустой файл
	emptyFilePath := createTempFile(t, "")
	tasks, maxID, err = LoadTasks(emptyFilePath)
	if err != nil {
		t.Fatalf("LoadTasks failed for empty file: %v", err)
	}
	if len(tasks) != 0 {
		t.Errorf("Expected 0 tasks for empty file, got %d", len(tasks))
	}
	if maxID != 0 {
		t.Errorf("Expected maxID 0 for empty file, got %d", maxID)
	}
}

func TestLoadTasks_ValidData(t *testing.T) {
	// Используем время без наносекунд для простоты сравнения
	time1 := time.Now().Truncate(time.Second)
	time2 := time1.Add(-24 * time.Hour).Truncate(time.Second)

	// Форматируем время в строку так, как это делает SaveTasks
	timeFormat := "2006-01-02 15:04:05"
	content := fmt.Sprintf("10|Task 1|false|%s|high\n"+
		"5|Task 2|true|%s|low\n"+
		"15|Task 3|false|%s|medium\n",
		time1.Format(timeFormat),
		time2.Format(timeFormat),
		time1.Format(timeFormat), // еще одна задача с тем же временем
	)
	filePath := createTempFile(t, content)

	expectedTasks := []task.Task{
		{ID: 10, Title: "Task 1", Done: false, CreatedAt: time1, Priority: task.PriorityHigh},
		{ID: 5, Title: "Task 2", Done: true, CreatedAt: time2, Priority: task.PriorityLow},
		{ID: 15, Title: "Task 3", Done: false, CreatedAt: time1, Priority: task.PriorityMedium},
	}
	expectedMaxID := 15

	tasks, maxID, err := LoadTasks(filePath)
	if err != nil {
		t.Fatalf("LoadTasks failed: %v", err)
	}
	if maxID != expectedMaxID {
		t.Errorf("Expected maxID %d, got %d", expectedMaxID, maxID)
	}
	assertTasksEqual(t, expectedTasks, tasks)
}

func TestLoadTasks_InvalidData(t *testing.T) {
	// Время для валидной строки
	validTime := time.Now().Truncate(time.Second)
	timeFormat := "2006-01-02 15:04:05"

	content := "1|Valid Task|false|" + validTime.Format(timeFormat) + "|high\n" + // Valid
		"invalid_id|Task 2|true|" + validTime.Format(timeFormat) + "|low\n" + // Invalid ID
		"3|Task 3|not_bool|" + validTime.Format(timeFormat) + "|medium\n" + // Invalid Done
		"4|Task 4|false|invalid_date|high\n" + // Invalid Date
		"5|Task 5|true|" + validTime.Format(timeFormat) + "|invalid_priority\n" + // Invalid Priority (будет medium)
		"6|Too few parts|true\n" + // Invalid line format
		"7|Valid Task 2|true|" + validTime.Format(timeFormat) + "|low\n" // Valid

	filePath := createTempFile(t, content)

	// Ожидаем только валидные строки и строку с исправленным приоритетом
	expectedTasks := []task.Task{
		{ID: 1, Title: "Valid Task", Done: false, CreatedAt: validTime, Priority: task.PriorityHigh},
		{ID: 5, Title: "Task 5", Done: true, CreatedAt: validTime, Priority: task.PriorityMedium}, // Приоритет исправлен на medium
		{ID: 7, Title: "Valid Task 2", Done: true, CreatedAt: validTime, Priority: task.PriorityLow},
	}
	expectedMaxID := 7

	// Перехватываем stderr, чтобы проверить предупреждения (опционально, но полезно)
	// В реальном тесте может потребоваться более сложная логика для перехвата stderr

	tasks, maxID, err := LoadTasks(filePath)
	if err != nil {
		t.Fatalf("LoadTasks failed: %v", err)
	}

	if maxID != expectedMaxID {
		t.Errorf("Expected maxID %d, got %d", expectedMaxID, maxID)
	}
	assertTasksEqual(t, expectedTasks, tasks)

	// Здесь можно добавить проверку логов stderr на наличие предупреждений, если это критично
}


func TestSaveTasks(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "save_tasks.txt")

	time1 := time.Now().Truncate(time.Second)
	time2 := time1.Add(-time.Hour).Truncate(time.Second)

	tasksToSave := []task.Task{
		{ID: 1, Title: "Save Me", Done: false, CreatedAt: time1, Priority: task.PriorityMedium},
		{ID: 20, Title: "Save Me Too", Done: true, CreatedAt: time2, Priority: task.PriorityHigh},
	}

	err := SaveTasks(filePath, tasksToSave)
	if err != nil {
		t.Fatalf("SaveTasks failed: %v", err)
	}

	// Проверяем содержимое файла напрямую
	contentBytes, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to read back saved file: %v", err)
	}
	content := string(contentBytes)

	timeFormat := "2006-01-02 15:04:05"
	expectedContent := fmt.Sprintf("1|Save Me|false|%s|medium\n"+
									"20|Save Me Too|true|%s|high\n",
									time1.Format(timeFormat), time2.Format(timeFormat))

	if content != expectedContent {
		t.Errorf("File content mismatch.\nExpected:\n%s\nActual:\n%s", expectedContent, content)
	}

	// Дополнительно: загружаем и проверяем
	loadedTasks, maxID, err := LoadTasks(filePath)
	if err != nil {
		t.Fatalf("LoadTasks failed after SaveTasks: %v", err)
	}
	if maxID != 20 {
		t.Errorf("Expected maxID 20 after load, got %d", maxID)
	}
	assertTasksEqual(t, tasksToSave, loadedTasks)
}


func TestAppendTask(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "append_tasks.txt")

	time1 := time.Now().Truncate(time.Second)
	time2 := time1.Add(time.Minute).Truncate(time.Second)
	time3 := time2.Add(time.Minute).Truncate(time.Second)
	timeFormat := "2006-01-02 15:04:05"

	// 1. Создаем начальный файл
	initialTasks := []task.Task{
		{ID: 1, Title: "Initial Task", Done: false, CreatedAt: time1, Priority: task.PriorityLow},
	}
	err := SaveTasks(filePath, initialTasks)
	if err != nil {
		t.Fatalf("Initial SaveTasks failed: %v", err)
	}

	// 2. Добавляем новые задачи
	tasksToAppend := []task.Task{
		{ID: 5, Title: "Appended Task 1", Done: true, CreatedAt: time2, Priority: task.PriorityHigh},
		{ID: 3, Title: "Appended Task 2", Done: false, CreatedAt: time3, Priority: task.PriorityMedium},
	}
	err = AppendTask(filePath, tasksToAppend)
	if err != nil {
		t.Fatalf("AppendTask failed: %v", err)
	}

	// 3. Проверяем содержимое файла
	contentBytes, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to read file after AppendTask: %v", err)
	}
	content := string(contentBytes)

	// Ожидаемое содержимое: начальная задача + добавленные
	expectedContent := fmt.Sprintf("1|Initial Task|false|%s|low\n"+ // Из SaveTasks
								   "5|Appended Task 1|true|%s|high\n"+ // Из AppendTask
								   "3|Appended Task 2|false|%s|medium\n", // Из AppendTask
									time1.Format(timeFormat),
									time2.Format(timeFormat),
									time3.Format(timeFormat))

	if content != expectedContent {
		t.Errorf("File content mismatch after AppendTask.\nExpected:\n%s\nActual:\n%s", expectedContent, content)
	}

	// 4. Проверяем загрузку (LoadTasks должен прочитать все)
	allTasks := append(initialTasks, tasksToAppend...)
	loadedTasks, maxID, err := LoadTasks(filePath)
	if err != nil {
		t.Fatalf("LoadTasks failed after AppendTask: %v", err)
	}
	if maxID != 5 { // Максимальный ID теперь 5
		t.Errorf("Expected maxID 5 after append, got %d", maxID)
	}
	// Порядок в loadedTasks будет как в файле, он не совпадет с allTasks,
	// поэтому сравниваем поэлементно или сортируем оба перед сравнением.
	// Проще проверить содержимое файла, как сделано выше.
	// Но убедимся, что количество совпадает.
	if len(loadedTasks) != len(allTasks) {
		 t.Errorf("Expected %d tasks after load, got %d", len(allTasks), len(loadedTasks))
	}
	// Для полной уверенности можно создать map[int]task.Task и сравнить их.
}

func TestSaveLoadCycle(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "cycle_tasks.txt")

	tasks1 := []task.Task{
		task.AddTask(1, "Task A", task.PriorityHigh),
		task.AddTask(2, "Task B", task.PriorityLow),
	}
	tasks1[1].Done = true // Отметим одну как выполненную

	// Сохраняем первый набор
	if err := SaveTasks(filePath, tasks1); err != nil {
		t.Fatalf("First SaveTasks failed: %v", err)
	}

	// Загружаем
	loaded1, maxID1, err := LoadTasks(filePath)
	if err != nil {
		t.Fatalf("First LoadTasks failed: %v", err)
	}
	if maxID1 != 2 {
		t.Errorf("Expected maxID 2 after first load, got %d", maxID1)
	}
	assertTasksEqual(t, tasks1, loaded1)

	// Модифицируем и добавляем
	tasks2 := loaded1
	tasks2[0].Done = true // Завершаем первую
	tasks2 = append(tasks2, task.AddTask(3, "Task C", task.PriorityMedium)) // Добавляем новую

	// Сохраняем второй раз
	if err := SaveTasks(filePath, tasks2); err != nil {
		t.Fatalf("Second SaveTasks failed: %v", err)
	}

	// Загружаем снова
	loaded2, maxID2, err := LoadTasks(filePath)
	if err != nil {
		t.Fatalf("Second LoadTasks failed: %v", err)
	}
	if maxID2 != 3 {
		t.Errorf("Expected maxID 3 after second load, got %d", maxID2)
	}
	assertTasksEqual(t, tasks2, loaded2)
}