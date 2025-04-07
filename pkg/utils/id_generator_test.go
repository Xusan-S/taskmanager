package utils

import (
	"sync"
	"testing"
)

func TestNewIDGenerator(t *testing.T) {
	initialMaxID := 10
	g := NewIDGenerator(initialMaxID)

	if g == nil {
		t.Fatal("NewIDGenerator returned nil")
	}
	// Первый ID должен быть initialMaxID + 1
	expectedFirstID := initialMaxID + 1
	if g.nextID != expectedFirstID {
		t.Errorf("Expected nextID to be %d, got %d", expectedFirstID, g.nextID)
	}
}

func TestNextID(t *testing.T) {
	g := NewIDGenerator(0) // Начинаем с 1

	firstID := g.NextID()
	if firstID != 1 {
		t.Errorf("Expected first ID to be 1, got %d", firstID)
	}

	secondID := g.NextID()
	if secondID != 2 {
		t.Errorf("Expected second ID to be 2, got %d", secondID)
	}

	thirdID := g.NextID()
	if thirdID != 3 {
		t.Errorf("Expected third ID to be 3, got %d", thirdID)
	}
}

func TestUpdateGenerator(t *testing.T) {
	g := NewIDGenerator(5) // nextID будет 6

	// Обновление меньшим или равным ID не должно ничего менять
	g.UpdateGenerator(4)
	if g.nextID != 6 {
		t.Errorf("Expected nextID to remain 6 after update with 4, got %d", g.nextID)
	}
	g.UpdateGenerator(5)
	if g.nextID != 6 {
		t.Errorf("Expected nextID to remain 6 after update with 5, got %d", g.nextID)
	}

	// Обновление большим ID должно изменить nextID
	g.UpdateGenerator(10)
	if g.nextID != 11 { // Должен стать 10 + 1
		t.Errorf("Expected nextID to become 11 after update with 10, got %d", g.nextID)
	}

	// Проверка следующего ID после обновления
	next := g.NextID()
	if next != 11 {
		t.Errorf("Expected NextID() after update to return 11, got %d", next)
	}
	if g.nextID != 12 {
		t.Errorf("Expected internal nextID to become 12 after NextID() call, got %d", g.nextID)
	}
}

func TestIDGeneratorConcurrency(t *testing.T) {
	g := NewIDGenerator(0)
	numGoroutines := 100
	idsPerGoroutine := 10
	totalIDs := numGoroutines * idsPerGoroutine
	generatedIDs := make(map[int]bool)
	var mu sync.Mutex // Мьютекс для безопасного доступа к generatedIDs
	var wg sync.WaitGroup

	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < idsPerGoroutine; j++ {
				id := g.NextID()
				mu.Lock()
				if _, exists := generatedIDs[id]; exists {
					t.Errorf("Duplicate ID generated: %d", id)
				}
				generatedIDs[id] = true
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	if len(generatedIDs) != totalIDs {
		t.Errorf("Expected %d unique IDs, but got %d", totalIDs, len(generatedIDs))
	}

	// Проверяем, что следующий ID корректен
	expectedNextID := totalIDs + 1
	if g.nextID != expectedNextID {
		 t.Errorf("Expected final nextID to be %d, got %d", expectedNextID, g.nextID)
	}
}