package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"sync"
	"syscall"
	"taskm/pkg/archiver"
	"taskm/pkg/logger"
	"taskm/pkg/storage"
	"taskm/pkg/task"
	"taskm/pkg/utils"
	"time"
)

const (
	storageDir = "storage"
	tasksFileName = "tasks.txt"
	archiverFileName = "archive.txt"
	logFileName = "log.txt"
	logBuffer = 100
	archiverInterval = 30 * time.Second
	defaultPriority = task.PriorityMedium
)

var (
	tasks []task.Task
	taskMutex sync.Mutex
	idGen *utils.IDGenerator
	appLogger *logger.Logger
	appArchiver *archiver.Archiver
	wg sync.WaitGroup
)

type byPriorityAndDate []task.Task

func (s byPriorityAndDate) Len() int {
	return len(s)
}

func (s byPriorityAndDate) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s byPriorityAndDate) Less(i, j int) bool {
	taskA := s[i]
	taskB := s[j]

	priorityA := taskA.PriorityValue()
	priorityB := taskB.PriorityValue()

	if priorityA != priorityB {
		// Высший приоритет идет раньше (High=3 > Medium=2 > Low=1)
		return priorityA > priorityB
	}
	// При равных приоритетах, более ранняя дата создания идет раньше
	return taskA.CreatedAt.Before(taskB.CreatedAt)
}

func main() {
	// Запускаем Контекст для Graceful Shutdown
	ctx, cancel :=context.WithCancel(context.Background())
	defer cancel()

	// Инициализация логгера
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func(){
		sig := <-sigChan
		fmt.Printf("\nСигнал получен: %v. Завершение работы...\n", sig)	
		cancel()
	}()

	taskPath := filepath.Join(storageDir, tasksFileName)
	archiverPath := filepath.Join(storageDir, archiverFileName)
	logPath := filepath.Join(storageDir, logFileName)

	if err := os.MkdirAll(storageDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Ошибка создания директории '%s' для хранения задач: %v\n", storageDir, err)
		os.Exit(1)
	}

	var logErr error
	appLogger, logErr = logger.NewLogger(logPath, logBuffer, &wg)
	if logErr != nil {
		fmt.Fprintf(os.Stderr, "Ошибка создания логгера: %v\n", logErr)
		os.Exit(1)
	}

	// Запускаем логгер
	appLogger.Run(ctx)

	appLogger.Log("Приложение запускается")

	// Загружаем задачи из файла
	var loadErr error
	var maxID int
	tasks, maxID, loadErr = storage.LoadTasks(taskPath)
	if loadErr != nil {
		appLogger.Log(fmt.Sprintf("Ошибка загрузки задач %v:", loadErr))
		tasks = []task.Task{}
	} else {
		appLogger.Log(fmt.Sprintf("Загружено %d задач. Максимальный ID задачи: %d", len(tasks), maxID))
	}

	idGen = utils.NewIDGenerator(maxID)

	appArchiver = archiver.NewArchiver(archiverPath, &tasks, &taskMutex, &wg)
	appArchiver.Run(ctx, archiverInterval)

	// Задаем флаги и читаем их с консоли
	addFlag := flag.String("add", "", "Добавить задачу")
	listFlag := flag.Bool("list", false, "Показать список задач")
	doneFlag := flag.Int("done", 0, "Отметить задачу как выполненную")
	deleteFlag := flag.Int ("delete", 0, "Удалить задачу")

	priorityFlag := flag.String("priority", defaultPriority, "Приоритет задачи (low, medium, high)")
	flag.Parse()

	actionTaken := false
	if *addFlag != "" {
		actionTaken = true
		handleAddTask(*addFlag, *priorityFlag)
	}
	if *listFlag {
		if actionTaken {
			fmt.Fprintln(os.Stderr, "Ошибка: нельзя использовать одновременно флаги -list с другими")
			appLogger.Log ("Ошибка: Использование нескольких флагов")
			os.Exit(1)
		}
		actionTaken = true
		handleListTasks()
	}
	if *doneFlag != 0 {
		if actionTaken {
			fmt.Fprintln(os.Stderr, "Ошибка: нельзя использовать одновременно флаги -done с другими")
			appLogger.Log ("Ошибка: Использование нескольких флагов")
			os.Exit(1)
		}
		actionTaken = true
		handleDoneTask(*doneFlag)
	}
	if *deleteFlag != 0 {
		if actionTaken {
			fmt.Fprintln(os.Stderr, "Ошибка: нельзя использовать одновременно флаги -delete с другими")
			appLogger.Log ("Ошибка: Использование нескольких флагов")
			os.Exit(1)
		}
		actionTaken = true
		handleDeleteTask(*deleteFlag)
	}

	if !actionTaken && len(os.Args) >1 {
		fmt.Fprintln(os.Stderr, "Ошибка: Не указаны флаги")
		fmt.Fprintln(os.Stderr, "Использовать: -add, -list, -done, -delete")
		appLogger.Log ("Ошибка: Не указаны флаги")
		os.Exit(1)
	}

	if !actionTaken {
		if len(os.Args) <= 1 {
			fmt.Println("TaskManager CLI")
			fmt.Println("Использовать: -add, -list, -done, -delete")
		}
	}

	<- ctx.Done()

	fmt.Println("\nЗавершение работы приложения...")

	fmt.Println("Ожидание завершения фоновых процессов...")
	waitTimeout := 10 * time.Second // Таймаут ожидания
	waitChan := make(chan struct{})
	go func(){
		wg.Wait() // Ждем wg.Done() от логгера и архиватора
		close(waitChan)
	}()

	select {
		case <-waitChan:
			fmt.Println("Фоновые процессы успешно завершены.")
		case <-time.After(waitTimeout):
			// Логгер уже может быть закрыт или недоступен, используем stderr
			fmt.Fprintf(os.Stderr, "Ошибка: время ожидания фоновых процессов истекло (%v)\n", waitTimeout)
			// НЕ НУЖНО: appLogger.Log("Время ожидания фоновых процессов истекло") // <-- УДАЛИТЬ
	}	
	

	// блокируем таск чтобы записать
	taskMutex.Lock()
	err := storage.SaveTasks(taskPath, tasks)
	// разблокируем таск
	taskMutex.Unlock()
	if err != nil {
		logMsg := fmt.Sprintf("Ошибка сохранения задач: %v", err)
		appLogger.Log(logMsg)
		fmt.Fprintf(os.Stderr, "%s\n", logMsg)
	} else {
		appLogger.Log ("Задачи сохранены в файл")
		fmt.Println("Задачи сохранены в файл")
	}

	if appLogger != nil { // Добавим проверку на nil на всякий случай
		appLogger.Close()
	}

	select {
		case <-waitChan:
			fmt.Println("Фоновые процессы завершены")
		case <-time.After(waitTimeout):
			fmt.Fprintf(os.Stderr, "Время ожидания фоновых процессов истекло")
	}

	appLogger.Close()
	fmt.Println("Приложение завершено")
}

// разбираемся с функциями для флагов

func handleAddTask(title, priority string) {
	switch priority {
	case task.PriorityHigh, task.PriorityMedium, task.PriorityLow:
		// все норм, ничего не делаем
	default:
		fmt.Fprintf(os.Stderr, "Внимание: неверный приоритет '%s'. Использован дефолтный '%s'\n", priority, defaultPriority)
		priority = defaultPriority
	}

	newTask := task.AddTask(idGen.NextID(), title, priority)

	taskMutex.Lock()
	tasks = append(tasks, newTask)
	taskMutex.Unlock()

	logMsg := fmt.Sprintf("Задача добавлена ID %d: \"%s\" приоритет: %s", newTask.ID, newTask.Title, newTask.Priority)
	appLogger.Log(logMsg)
	fmt.Printf("Task added with ID %d.\n", newTask.ID)
}

func handleListTasks() {
	taskMutex.Lock()
	// Создаем копию для сортировки, чтобы быстро освободить мьютекс
	tasksCopy := make([]task.Task, len(tasks))
	copy(tasksCopy, tasks)
	taskMutex.Unlock()

	if len(tasksCopy) == 0 {
		fmt.Println("No tasks found.")
		appLogger.Log("Listed tasks: None found.")
		return
	}

	
	sort.Sort(byPriorityAndDate(tasksCopy))


	fmt.Println("-------------------- TASKS --------------------")
	currentPriority := ""
	for _, t := range tasksCopy {
		if t.Priority != currentPriority {
			// Выводим заголовок группы приоритета при его смене
			fmt.Printf("\n--- Priority: %s ---\n", t.Priority)
			currentPriority = t.Priority
		}
		status := "Pending"
		if t.Done {
			status = "Done"
		}
		// Форматируем время создания для лучшей читаемости при выводе
		createdStr := t.CreatedAt.Format("2006-01-02 15:04") // Используем формат без секунд для вывода
		fmt.Printf("  ID: %-4d | Status: %-7s | Created: %s | Title: %s\n", t.ID, status, createdStr, t.Title)
	}
	fmt.Println("-----------------------------------------------")
	appLogger.Log(fmt.Sprintf("Listed %d tasks.", len(tasksCopy)))
}

func handleDoneTask(id int) {
	taskMutex.Lock() // Блокируем перед доступом к tasks
	defer taskMutex.Unlock() // Гарантируем разблокировку при выходе

	found := false
	for i := range tasks { // Используем индекс для модификации
		if tasks[i].ID == id {
			if tasks[i].Done {
				fmt.Printf("Task with ID %d is already marked as done.\n", id)
				appLogger.Log(fmt.Sprintf("Attempted to mark task ID %d as done, but it was already done.", id))
				return // Выходим из функции, так как задача уже выполнена
			}
			tasks[i].Done = true // Модифицируем задачу в срезе
			found = true
			logMsg := fmt.Sprintf("Marked task ID %d as done: \"%s\"", tasks[i].ID, tasks[i].Title)
			appLogger.Log(logMsg)
			fmt.Printf("Marked task with ID %d as done.\n", id)
			break // Выходим из цикла, так как нашли и обработали задачу
		}
	}

	if !found {
		fmt.Fprintf(os.Stderr, "Error: Task with ID %d not found.\n", id)
		appLogger.Log(fmt.Sprintf("Error: Failed to mark task ID %d as done - not found.", id))
	}
}

func handleDeleteTask(id int) {
	taskMutex.Lock() // Блокируем перед доступом к tasks
	defer taskMutex.Unlock() // Гарантируем разблокировку при выходе

	foundIndex := -1
	var deletedTaskTitle string // Для логирования

	for i, t := range tasks {
		if t.ID == id {
			foundIndex = i
			deletedTaskTitle = t.Title // Сохраняем для лога
			break
		}
	}

	if foundIndex != -1 {
		// Эффективное удаление из среза
		tasks[foundIndex] = tasks[len(tasks)-1] // Копируем последний элемент на место удаляемого
		// tasks[len(tasks)-1] = nil // Опционально: обнуляем последний элемент для сборщика мусора (для срезов указателей) - здесь не строго нужно
		tasks = tasks[:len(tasks)-1]            // Уменьшаем длину среза на 1

		logMsg := fmt.Sprintf("Deleted task ID %d: \"%s\"", id, deletedTaskTitle)
		appLogger.Log(logMsg)
		fmt.Printf("Deleted task with ID %d.\n", id)
	} else {
		fmt.Fprintf(os.Stderr, "Error: Task with ID %d not found for deletion.\n", id)
		appLogger.Log(fmt.Sprintf("Error: Failed to delete task ID %d - not found.", id))
	}
}