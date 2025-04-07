package logger

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"
)

const logTimeFormat = "2006-01-02 15:04:05"

type Logger struct {
	logChan  chan string
	filePath string
	file     *os.File
	wg       *sync.WaitGroup
	mu sync.Mutex
	closed bool

}

// делаем что-то типа конструктора из ООП для логгера
func NewLogger(filePath string, bufferSize int, wg *sync.WaitGroup) (*Logger, error) {
	file, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("Не удалось открыть файл для логирования %s: %w", filePath, err)
	}

	l := &Logger{
		logChan:  make(chan string, bufferSize),
		filePath: filePath,
		file:     file,
		wg:       wg,
	}

	return l, nil
}

// Запускаем горутину, которая будет слушать канал и записывать в файл
func (l *Logger) Run(ctx context.Context){
	l.wg.Add(1)
	fmt.Println("Logger started")
	go func() {
		defer l.wg.Done()
		defer l.file.Close()
		fmt.Println("Logger goroutine is runnning")
	LogLoop:
		for{
			select{
			case msg, ok := <- l.logChan:
				if !ok {
					fmt.Println("Log channel closed")
					break LogLoop	
				}
				timestamp := time.Now().Format(logTimeFormat)
				logEntry := fmt.Sprintf("[%s] %s\n", timestamp, msg)
				if _, err := l.file.WriteString(logEntry); err != nil {
					fmt.Printf("Ошибка записи в лог файл %s: %v\n", l.filePath, err)
				}
			
			case <- ctx.Done():
				fmt.Println("Logger context done")
				close(l.logChan)
				for msg := range l.logChan {
					timestamp := time.Now().Format(logTimeFormat)
					logEntry := fmt.Sprintf("[%s] %s\n", timestamp, msg)
					if _, err := l.file.WriteString(logEntry); err != nil {
						fmt.Printf("Ошибка записи в лог файл %s: %v\n", l.filePath, err)
					}
				}
				fmt.Println("Logger goroutine is done")
				break LogLoop
			}
		}
		fmt.Println("Logger goroutine stopped")
	}()
}

func (l *Logger) Log(message string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closed {
        fmt.Fprintf(os.Stderr, "Attempted to log after logger was closed: %s\n", message)
        return
    }
	select {
	case l.logChan <- message:
	default: 
	fmt.Fprintf(os.Stderr, "Лог канал переполнен, пропускаем сообщение: %s\n", message)
	}
}

func (l *Logger) Close() {
	l.mu.Lock()
    defer l.mu.Unlock()
    if l.closed {
        return
    }
    l.closed = true
    fmt.Println("Logger Close() called (channel closing handled by context).")
}
