package archiver

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"taskm/pkg/storage" // Нужно для чтения архива
	"taskm/pkg/task"
	"testing"
	"time"
	"reflect"
)

// Хелпер для создания временного файла
func createEmptyTempFile(t *testing.T, dir, name string) string {
	t.Helper()
	filePath := filepath.Join(dir, name)
	f, err := os.Create(filePath)
	if err != nil {
		t.Fatalf("Failed to create temp file %s: %v", filePath, err)
	}
	f.Close()
	return filePath
}


func TestNewArchiver(t *testing.T) {
	dir := t.TempDir()
	archivePath := filepath.Join(dir, "archive.txt")
	var tasks []task.Task
	var taskMutex sync.Mutex
	var wg sync.WaitGroup

	archiver := NewArchiver(archivePath, &tasks, &taskMutex, &wg)

	if archiver == nil {
		t.Fatal("NewArchiver returned nil")
	}
	if archiver.archiverPath != archivePath {
		t.Errorf("Expected archiverPath '%s', got '%s'", archivePath, archiver.archiverPath)
	}
	if archiver.tasks != &tasks {
		t.Error("Tasks slice not set correctly")
	}
	if archiver.taskMutex != &taskMutex {
		t.Error("Task mutex not set correctly")
	}
	if archiver.wg != &wg {
		t.Error("WaitGroup not set correctly")
	}
}


func TestArchiveCompletedTasks_NoTasks(t *testing.T) {
	dir := t.TempDir()
	archivePath := createEmptyTempFile(t, dir, "archive_no_tasks.txt")
	tasks := []task.Task{} // Пустой список задач
	var taskMutex sync.Mutex
	var wg sync.WaitGroup // Не используется напрямую в этой функции, но нужен для NewArchiver

	archiver := NewArchiver(archivePath, &tasks, &taskMutex, &wg)

	err := archiver.archiveCompletedTasks()
	if err != nil {
		t.Fatalf("archiveCompletedTasks failed: %v", err)
	}

	// Проверяем, что архивный файл остался пустым
	content, err := os.ReadFile(archivePath)
	if err != nil {
		t.Fatalf("Failed to read archive file: %v", err)
	}
	if len(content) > 0 {
		t.Errorf("Expected archive file to be empty, but got content: %s", string(content))
	}
}

func TestArchiveCompletedTasks_NoCompletedTasks(t *testing.T) {
	dir := t.TempDir()
	archivePath := createEmptyTempFile(t, dir, "archive_no_completed.txt")
	tasks := []task.Task{
		task.AddTask(1, "Task 1", task.PriorityHigh), // Не завершена
		task.AddTask(2, "Task 2", task.PriorityLow),  // Не завершена
	}
	var taskMutex sync.Mutex
	var wg sync.WaitGroup

	archiver := NewArchiver(archivePath, &tasks, &taskMutex, &wg)

	err := archiver.archiveCompletedTasks()
	if err != nil {
		t.Fatalf("archiveCompletedTasks failed: %v", err)
	}

	// Проверяем, что архивный файл остался пустым
	content, err := os.ReadFile(archivePath)
	if err != nil {
		t.Fatalf("Failed to read archive file: %v", err)
	}
	if len(content) > 0 {
		t.Errorf("Expected archive file to be empty, but got content: %s", string(content))
	}
}


func TestArchiveCompletedTasks_WithCompletedTasks(t *testing.T) {
	dir := t.TempDir()
	archivePath := createEmptyTempFile(t, dir, "archive_with_completed.txt")
	time1 := time.Now().Truncate(time.Second)
	time2 := time1.Add(-time.Hour)

	tasks := []task.Task{
		{ID: 1, Title: "Completed Task 1", Done: true, CreatedAt: time1, Priority: task.PriorityHigh},
		{ID: 2, Title: "Pending Task", Done: false, CreatedAt: time1, Priority: task.PriorityMedium},
		{ID: 3, Title: "Completed Task 2", Done: true, CreatedAt: time2, Priority: task.PriorityLow},
	}
	initialTasksCopy := make([]task.Task, len(tasks)) // Копия для проверки неизменности
	copy(initialTasksCopy, tasks)

	var taskMutex sync.Mutex
	var wg sync.WaitGroup

	archiver := NewArchiver(archivePath, &tasks, &taskMutex, &wg)

	err := archiver.archiveCompletedTasks()
	if err != nil {
		t.Fatalf("archiveCompletedTasks failed: %v", err)
	}

	// Проверяем, что исходный список задач НЕ изменился (т.к. архиватор только копирует)
	if len(tasks) != len(initialTasksCopy) {
		t.Fatalf("Original tasks slice length changed from %d to %d", len(initialTasksCopy), len(tasks))
	}
	for i := range tasks {
		if !reflect.DeepEqual(tasks[i], initialTasksCopy[i]) {
			t.Errorf("Original task at index %d was modified.\nExpected: %+v\nActual:   %+v", i, initialTasksCopy[i], tasks[i])
		}
	}


	// Проверяем содержимое архивного файла
	// Используем storage.LoadTasks для удобства
	archivedTasks, maxID, err := storage.LoadTasks(archivePath)
	if err != nil {
		t.Fatalf("Failed to load tasks from archive file %s: %v", archivePath, err)
	}

	expectedArchivedTasks := []task.Task{
		tasks[0], // Completed Task 1
		tasks[2], // Completed Task 2
	}
	expectedMaxID := 3 // Максимальный ID среди заархивированных

	if maxID != expectedMaxID {
		t.Errorf("Expected maxID in archive %d, got %d", expectedMaxID, maxID)
	}
	// storage.LoadTasks не гарантирует порядок, поэтому сравним размеры
	// и проверим наличие нужных ID
	if len(archivedTasks) != len(expectedArchivedTasks) {
		t.Fatalf("Expected %d tasks in archive, got %d", len(expectedArchivedTasks), len(archivedTasks))
	}

	foundIDs := make(map[int]bool)
	for _, at := range archivedTasks {
		foundIDs[at.ID] = true
	}

	for _, et := range expectedArchivedTasks {
		if !foundIDs[et.ID] {
			t.Errorf("Expected archived task with ID %d not found in archive file", et.ID)
		}
		// Можно добавить более детальное сравнение полей, если нужно
	}

	// Альтернативно, можно прочитать файл как строки и проверить их
	content, _ := os.ReadFile(archivePath)
	contentStr := string(content)
	timeFormat := "2006-01-02 15:04:05"
	expectedLine1 := fmt.Sprintf("%d|%s|%t|%s|%s", tasks[0].ID, tasks[0].Title, tasks[0].Done, tasks[0].CreatedAt.Format(timeFormat), tasks[0].Priority)
	expectedLine3 := fmt.Sprintf("%d|%s|%t|%s|%s", tasks[2].ID, tasks[2].Title, tasks[2].Done, tasks[2].CreatedAt.Format(timeFormat), tasks[2].Priority)

	if !strings.Contains(contentStr, expectedLine1) {
		t.Errorf("Archive content missing expected line: %s", expectedLine1)
	}
 	if !strings.Contains(contentStr, expectedLine3) {
		t.Errorf("Archive content missing expected line: %s", expectedLine3)
	}
	if strings.Contains(contentStr, "Pending Task"){
		 t.Errorf("Archive content contains non-completed task title")
	}
}


// Тестирование Run сложно из-за тикера. Можно протестировать только запуск и остановку.
func TestArchiver_Run_ContextCancel(t *testing.T) {
	dir := t.TempDir()
	archivePath := createEmptyTempFile(t, dir, "archive_run_cancel.txt")
	tasks := []task.Task{}
	var taskMutex sync.Mutex
	var wg sync.WaitGroup

	archiver := NewArchiver(archivePath, &tasks, &taskMutex, &wg)

	ctx, cancel := context.WithCancel(context.Background())

	// Запускаем Run в горутине
	archiver.Run(ctx, 100*time.Millisecond) // Используем короткий интервал для теста

	// Убедимся, что wg был увеличен
	// Это немного хак, но показывает, что Add(1) был вызван
	wg.Add(1) // Добавляем еще один, чтобы Wait не сработал сразу, если Run завершится мгновенно
	finished := make(chan struct{})
	go func(){
		wg.Wait()
		close(finished)
	}()

	// Даем немного времени поработать (хотя бы один тик)
	time.Sleep(150 * time.Millisecond)

	// Отменяем контекст
	cancel()

	// Убираем добавленный нами счетчик
	wg.Done()

	// Ждем завершения горутины архиватора
	select {
	case <-finished:
		// Горутина завершилась, как ожидалось
	case <-time.After(2 * time.Second): // Таймаут
		t.Fatal("Timed out waiting for archiver goroutine to finish after context cancel")
	}

	// Дополнительно можно проверить логи (если бы они были) или состояние
}