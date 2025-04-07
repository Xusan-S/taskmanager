package logger

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// Хелпер для чтения файла лога
func readLogFile(t *testing.T, filePath string) string {
	t.Helper()
	content, err := os.ReadFile(filePath)
	if err != nil {
		// Если файл еще не создан или пуст, это может быть нормально в некоторых тестах
		if os.IsNotExist(err) {
			return ""
		}
		t.Fatalf("Failed to read log file %s: %v", filePath, err)
	}
	return string(content)
}

func TestNewLogger(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "test.log")
	bufferSize := 10
	var wg sync.WaitGroup

	lg, err := NewLogger(logPath, bufferSize, &wg)
	if err != nil {
		t.Fatalf("NewLogger failed: %v", err)
	}
	if lg == nil {
		t.Fatal("NewLogger returned nil logger")
	}
	defer lg.file.Close() // Закрываем файл, так как Run не запускался

	// Проверяем, что файл создан
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		t.Errorf("Log file %s was not created", logPath)
	}
	if cap(lg.logChan) != bufferSize {
		t.Errorf("Expected log channel capacity %d, got %d", bufferSize, cap(lg.logChan))
	}
	if lg.wg != &wg {
		t.Error("WaitGroup was not set correctly")
	}
}

func TestLogger_RunAndLog(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "run_test.log")
	bufferSize := 10
	var wg sync.WaitGroup

	lg, err := NewLogger(logPath, bufferSize, &wg)
	if err != nil {
		t.Fatalf("NewLogger failed: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel() // Отменяем контекст в конце теста

	lg.Run(ctx) // Запускаем горутину логгера

	// Логируем сообщения
	msg1 := "Test message 1"
	msg2 := "Another message"
	lg.Log(msg1)
	lg.Log(msg2)

	// Даем время горутине обработать сообщения
	// В реальных сценариях лучше использовать каналы или другие примитивы синхронизации,
	// но для простого теста небольшой задержки часто достаточно.
	time.Sleep(100 * time.Millisecond)

	// Останавливаем логгер через контекст
	cancel()

	// Ждем завершения горутины логгера
	// Устанавливаем таймаут, чтобы тест не завис навсегда
	waitChan := make(chan struct{})
	go func() {
		wg.Wait()
		close(waitChan)
	}()

	select {
	case <-waitChan:
		// Горутина завершилась успешно
	case <-time.After(2 * time.Second): // Таймаут ожидания
		t.Fatal("Timed out waiting for logger goroutine to finish")
	}

	// Проверяем содержимое файла лога
	logContent := readLogFile(t, logPath)

	if !strings.Contains(logContent, msg1) {
		t.Errorf("Log content does not contain message: '%s'", msg1)
	}
	if !strings.Contains(logContent, msg2) {
		t.Errorf("Log content does not contain message: '%s'", msg2)
	}
	// Проверяем формат (хотя бы для одной строки)
	if !strings.Contains(logContent, fmt.Sprintf("] %s", msg1)) || !strings.Contains(logContent, "[") {
		t.Errorf("Log content does not seem to have the expected format [timestamp] message. Content:\n%s", logContent)
	}
}


func TestLogger_Close(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "close_test.log")
	bufferSize := 5
	var wg sync.WaitGroup

	lg, err := NewLogger(logPath, bufferSize, &wg)
	if err != nil {
		t.Fatalf("NewLogger failed: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	// Не отменяем контекст сразу

	lg.Run(ctx) // Запускаем горутину

	lg.Log("Message before close")
	time.Sleep(50 * time.Millisecond) // Дать время записать

	lg.Close() // Вызываем Close

	// Попытка записи после Close (должна быть проигнорирована или вызвать stderr)
	lg.Log("Message after close")

	// Даем немного времени, чтобы убедиться, что сообщение "after close" не попало
	time.Sleep(50 * time.Millisecond)

	// Теперь отменяем контекст, чтобы горутина завершилась
	cancel()

	// Ждем завершения
	waitChan := make(chan struct{})
	go func() {
		wg.Wait()
		close(waitChan)
	}()
	select {
	case <-waitChan:
		// OK
	case <-time.After(1 * time.Second):
		t.Fatal("Timed out waiting for logger goroutine to finish after Close and cancel")
	}

	// Проверяем лог
	logContent := readLogFile(t, logPath)
	if !strings.Contains(logContent, "Message before close") {
		t.Error("Log should contain message sent before Close()")
	}
	if strings.Contains(logContent, "Message after close") {
		t.Error("Log should NOT contain message sent after Close()")
	}
}

func TestLogger_LogFullBuffer(t *testing.T) {
	// Этот тест проверяет, что логгер не блокируется при переполнении буфера,
	// хотя само сообщение об ошибке идет в stderr и его сложно перехватить стандартными средствами.
	dir := t.TempDir()
	logPath := filepath.Join(dir, "full_buffer_test.log")
	bufferSize := 2 // Маленький буфер
	var wg sync.WaitGroup

	lg, err := NewLogger(logPath, bufferSize, &wg)
	if err != nil {
		t.Fatalf("NewLogger failed: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	lg.Run(ctx) // Запускаем горутину

	// Заполняем буфер + еще одно сообщение, которое должно вызвать переполнение
	// (Горутина может успеть обработать что-то, поэтому отправляем больше)
	for i := 0; i < bufferSize+5; i++ {
		lg.Log(fmt.Sprintf("Message %d", i))
	}

	// Даем время на обработку и потенциальный вывод ошибки переполнения в stderr
	time.Sleep(100 * time.Millisecond)

	// Отменяем и ждем
	cancel()
	waitChan := make(chan struct{})
	go func(){
		wg.Wait()
		close(waitChan)
	}()
	select{
	case <- waitChan:
		//OK
	case <- time.After(1*time.Second):
		t.Fatal("Timed out waiting for logger")
	}

	// Проверяем, что какие-то сообщения все же записались
	logContent := readLogFile(t, logPath)
	if len(logContent) == 0 {
		t.Error("Log file is empty, expected some messages to be written")
	}
	// Проверить наличие ошибки переполнения в stderr в рамках теста сложно.
	t.Log("Test assumes buffer overflow message was printed to stderr (if applicable)")
}