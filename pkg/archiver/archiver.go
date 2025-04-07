package archiver

import (
	"context"
	"fmt"
	"os"
	"sync"
	"taskm/pkg/storage"
	"taskm/pkg/task"
	"time"
)

type Archiver struct {
	archiverPath string
	tasks        *[]task.Task
	taskMutex    *sync.Mutex
	wg           *sync.WaitGroup
}

// делаем что-то типа конструктора из ООП для архива
func NewArchiver(archiverPath string, tasks *[]task.Task, taskMutex *sync.Mutex, wg *sync.WaitGroup) *Archiver {
	return &Archiver{
		archiverPath: archiverPath,
		tasks:        tasks,
		taskMutex:    taskMutex,
		wg:           wg,
	}
}

func (a *Archiver) Run(ctx context.Context, interval time.Duration){
	a.wg.Add(1)
	go func(){
		defer a.wg.Done()
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		fmt.Println("Горутина архивации запущена.")

		for {
			select {
			case <- ticker.C:
				fmt.Println("Тик архивации.")
				err := a.archiveCompletedTasks()
				if err != nil {
					fmt.Fprintf(os.Stderr, "Ошибка архивации: %v\n", err)
				}
			case <- ctx.Done():
				fmt.Println("Контекст архивации отменен. Остановка.")
				fmt.Println("Горутина архивации остановлена.")
				return
			}
		}
	}()
}

func (a *Archiver) archiveCompletedTasks() error {
	a.taskMutex.Lock()
	tasksToArchive := []task.Task{}
	for _, t := range *a.tasks {
		if t.Done {
			tasksToArchive = append(tasksToArchive, t)
		}
	}
	a.taskMutex.Unlock()
	if len(tasksToArchive) == 0 {
		fmt.Println("Нет завершенных задач для архивации.")
		return nil
	}

	fmt.Printf("Архивируем %d завершенных задач.\n", len(tasksToArchive))

	err := storage.AppendTask(a.archiverPath, tasksToArchive)
	if err != nil {
		return fmt.Errorf("Не удалось добавить таски в архив %s: %w", a.archiverPath, err)
	}

	fmt.Printf("Архиватор: Успешно добавлено %d тасок в %s.\n", len(tasksToArchive), a.archiverPath)

	
	return nil

}