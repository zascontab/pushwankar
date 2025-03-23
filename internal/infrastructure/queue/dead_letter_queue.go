package queue

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"notification-service/pkg/logging"
)

// DeadLetterQueue es una cola para mensajes que no pudieron ser entregados después de múltiples intentos
type DeadLetterQueue struct {
	tasks       map[string]RetryableTask
	mu          sync.RWMutex
	logger      *logging.Logger
	maxCapacity int
	retention   time.Duration
	entryTimes  map[string]time.Time
}

// NewDeadLetterQueue crea una nueva instancia de DeadLetterQueue
func NewDeadLetterQueue(logger *logging.Logger, maxCapacity int, retention time.Duration) *DeadLetterQueue {
	dlq := &DeadLetterQueue{
		tasks:       make(map[string]RetryableTask),
		logger:      logger,
		maxCapacity: maxCapacity,
		entryTimes:  make(map[string]time.Time),
		retention:   retention,
	}

	// Iniciar rutina de limpieza si se especifica un período de retención
	if retention > 0 {
		go dlq.startCleanupTimer()
	}

	return dlq
}

// AddTask agrega una tarea a la cola de mensajes muertos
func (q *DeadLetterQueue) AddTask(task RetryableTask) {
	q.mu.Lock()
	defer q.mu.Unlock()

	taskID := task.GetID()
	
	// Si la cola está llena, eliminar la tarea más antigua
	if q.maxCapacity > 0 && len(q.tasks) >= q.maxCapacity {
		var oldestID string
		var oldestTime time.Time
		
		// Primera vez, inicializar con la hora actual
		if len(q.entryTimes) == 0 {
			oldestTime = time.Now()
		}
		
		// Buscar la tarea más antigua
		for id, entryTime := range q.entryTimes {
			if oldestID == "" || entryTime.Before(oldestTime) {
				oldestID = id
				oldestTime = entryTime
			}
		}
		
		// Eliminar la tarea más antigua
		if oldestID != "" {
			delete(q.tasks, oldestID)
			delete(q.entryTimes, oldestID)
			q.logger.Warn("Removed oldest task %s from Dead Letter Queue due to capacity limit", oldestID)
		}
	}
	
	// Agregar la nueva tarea
	q.tasks[taskID] = task
	q.entryTimes[taskID] = time.Now()
	q.logger.Warn("Task %s added to Dead Letter Queue", taskID)
}

// GetTasks devuelve todas las tareas en la cola de mensajes muertos
func (q *DeadLetterQueue) GetTasks() []RetryableTask {
	q.mu.RLock()
	defer q.mu.RUnlock()

	tasks := make([]RetryableTask, 0, len(q.tasks))
	for _, task := range q.tasks {
		tasks = append(tasks, task)
	}

	return tasks
}

// GetTask obtiene una tarea específica de la cola
func (q *DeadLetterQueue) GetTask(taskID string) (RetryableTask, bool) {
	q.mu.RLock()
	defer q.mu.RUnlock()
	
	task, exists := q.tasks[taskID]
	return task, exists
}

// RemoveTask elimina una tarea de la cola de mensajes muertos
func (q *DeadLetterQueue) RemoveTask(taskID string) {
	q.mu.Lock()
	defer q.mu.Unlock()

	delete(q.tasks, taskID)
	delete(q.entryTimes, taskID)
	q.logger.Info("Task %s removed from Dead Letter Queue", taskID)
}

// RetryTask reintenta una tarea de la cola de mensajes muertos
func (q *DeadLetterQueue) RetryTask(ctx context.Context, taskID string) error {
	q.mu.Lock()
	task, exists := q.tasks[taskID]
	q.mu.Unlock()

	if !exists {
		return fmt.Errorf("task %s not found in Dead Letter Queue", taskID)
	}

	// Intentar ejecutar la tarea directamente
	err := task.Execute(ctx)
	if err != nil {
		return fmt.Errorf("failed to execute task %s: %w", taskID, err)
	}

	// Si la ejecución es exitosa, eliminar de la cola
	task.OnSuccess(ctx)
	q.RemoveTask(taskID)
	
	return nil
}

// RetryAllTasks reintenta todas las tareas en la cola
func (q *DeadLetterQueue) RetryAllTasks(ctx context.Context) (int, int, error) {
	tasks := q.GetTasks()
	
	if len(tasks) == 0 {
		return 0, 0, nil
	}
	
	successCount := 0
	failCount := 0
	
	for _, task := range tasks {
		taskID := task.GetID()
		err := q.RetryTask(ctx, taskID)
		if err != nil {
			q.logger.Error("Failed to retry task %s: %v", taskID, err)
			failCount++
		} else {
			successCount++
		}
	}
	
	if failCount > 0 {
		return successCount, failCount, errors.New("some tasks failed to retry")
	}
	
	return successCount, failCount, nil
}

// startCleanupTimer inicia una rutina para limpiar tareas expiradas
func (q *DeadLetterQueue) startCleanupTimer() {
	ticker := time.NewTicker(q.retention / 2) // Revisar 2 veces en el período de retención
	defer ticker.Stop()
	
	for range ticker.C {
		q.cleanupExpiredTasks()
	}
}

// cleanupExpiredTasks elimina las tareas que exceden el período de retención
func (q *DeadLetterQueue) cleanupExpiredTasks() {
	now := time.Now()
	expiredTasks := []string{}
	
	q.mu.RLock()
	for taskID, entryTime := range q.entryTimes {
		if now.Sub(entryTime) > q.retention {
			expiredTasks = append(expiredTasks, taskID)
		}
	}
	q.mu.RUnlock()
	
	if len(expiredTasks) > 0 {
		q.mu.Lock()
		for _, taskID := range expiredTasks {
			delete(q.tasks, taskID)
			delete(q.entryTimes, taskID)
		}
		q.mu.Unlock()
		
		q.logger.Info("Cleaned up %d expired tasks from Dead Letter Queue", len(expiredTasks))
	}
}
