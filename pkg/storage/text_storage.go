package storage

import (
	"bufio"
	"fmt"
	"os"
	//"sort"
	"strconv"
	"strings"
	"taskm/pkg/task"
	"time"
)

const(
	taskFileFormat = "%d|%s|%t|%s|%s\n"
	timeFormat = "2006-01-02 15:04:05"
)

func LoadTasks(filePath string) ([]task.Task, int, error){
	file, err := os.Open(filePath)
	if err != nil {
		if os.IsNotExist(err){
			return []task.Task{}, 0, nil
		}
		return nil, 0, fmt.Errorf("Не удалось открыть файл %s: %w", filePath, err)
	}
	defer file.Close()

	tasks := []task.Task{}
	scanner := bufio.NewScanner(file)
	maxID := 0

	lineNumber := 0
	for scanner.Scan(){
		lineNumber++
		line := scanner.Text()
		// Делаем разделение на пять частей(ID, Title, Done, CreatedAt, Priority)
		parts := strings.SplitN(line, "|", 5)
		// Проверяем, точно ли у нас пять частей
		if len(parts) != 5 {
			fmt.Fprintf(os.Stderr, "Предупреждение: пропуск неверной строки %d в %s: %s\n", lineNumber,filePath, line)
			continue
		}

		// Преобразуем ID в int
		id, err := strconv.Atoi(parts[0])
		if err != nil {
			fmt.Fprintf(os.Stderr, "Предупреждение: пропуск неверной строки %d из-за неверного ID '%s': %v\n", lineNumber, parts[0], err)
			continue
		}

		title := parts[1]

		done, err := strconv.ParseBool(parts[2])
		if err != nil {
			fmt.Fprintf(os.Stderr, "Предупреждение: пропуск неверной строки %d из-за неверного статуса выполнения '%s': %v\n", lineNumber, parts[2], err)
			continue
		}

		created, err := time.Parse(timeFormat, parts[3])
		if err != nil {
			fmt.Fprintf(os.Stderr, "Предупреждение: пропуск неверной строки %d из-за неверной даты '%s': %v\n", lineNumber, parts[3], err)
			continue
		}

		priority := parts[4]
		// проверяем, правильный ли приоритет 
		switch priority {
		case task.PriorityHigh, task.PriorityMedium, task.PriorityLow:
			// в этом случае, все нормальное, ничего не делаем
		default:
			// ошибка, если приоритет не 1, 2 или 3
			fmt.Fprintf(os.Stderr, "Предупреждение: пропуск неверной строки %d из-за неверного приоритета '%s', будет использовано medium\n", lineNumber, priority)
			priority = task.PriorityMedium
		}

		newTask := task.Task{
			ID:        id,
			Title:     title,
			Done:      done,
			CreatedAt: created,
			Priority:  priority,
		}
		
		tasks = append(tasks, newTask)

		if id > maxID {
			maxID = id
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, 0, fmt.Errorf("Ошибка чтения файла %s: %w", filePath, err)
	}

	return tasks, maxID, nil
}

// SaveTasks сохраняет (измененные) задачи в файл
func SaveTasks(filePath string, tasks []task.Task) error {
	// Сделаем временный файл, чтобы избежать потери данных
	tempFilePath := filePath + ".tmp"
	file, err := os.Create(tempFilePath)
	if err != nil {
		return fmt.Errorf("Не удалось создать временный файл %s: %w", tempFilePath, err)
	}

	writer := bufio.NewWriter(file)
	for _, t := range tasks {
		// будем записывать задачи сразу
		line := fmt.Sprintf(taskFileFormat,
			t.ID,
			t.Title,
			t.Done,
			t.CreatedAt.Format(timeFormat),
			t.Priority,
		)
		_, err := writer.WriteString(line)
		if err != nil {
			// если ошибка, закроем файл и удалим временный файл
			file.Close()
			os.Remove(tempFilePath)
			return fmt.Errorf("Ошибка записи задачи %d во временный файл %w", t.ID, err)
		}
	}


	// Flush записывает все данные из буфера в файл
	if err := writer.Flush(); err != nil {
		file.Close()
		os.Remove(tempFilePath)
		return fmt.Errorf("Ошибка записи во временный файл %s: %w", tempFilePath, err)
	}

	// теперь файл можно закрыать
	if err := file.Close(); err != nil {
		os.Remove(tempFilePath)
		return fmt.Errorf("Ошибка закрытия временного файла %s: %w", tempFilePath, err)
	}

	// Переименуем временный файл в основной
    /*
		здесь логика такая: мы загружем таски из файла с помощью LoadTasks
		потом делаем временный файл где будут происходить изменения
		потом переименуем временный файл в основной
		если что-то пойдет не так, то у нас останется старый файл

	*/

	if err := os.Rename(tempFilePath, filePath); err != nil {
		os.Remove(tempFilePath)
		return fmt.Errorf("Ошибка переименования временного файла %s в %s: %w", tempFilePath, filePath, err)
	}
	return nil
}

// AppendTask добавляет задачу в конец файла
func AppendTask(filePath string, tasks []task.Task) error {
	file, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("Не удалось открыть архивный файл %s для записи: %w", filePath, err)
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	for _, t := range tasks {
		line := fmt.Sprintf(taskFileFormat,
			t.ID,
			t.Title,
			t.Done,
			t.CreatedAt.Format(timeFormat),
			t.Priority,
		)
		_, err := writer.WriteString(line)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Ошибка записи задачи %d в архивный файл %v", t.ID, err)
		}
	}

	// Flush записывает все данные из буфера в файл
	if err := writer.Flush(); err != nil {
		return fmt.Errorf("Ошибка записи в архивный файл %s: %w", filePath, err)
	}
	return nil
}