package queue

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sync"
	"time"

	"notification-service/internal/domain/entity"
	"notification-service/internal/domain/repository"
	"notification-service/pkg/logging"
)

// RetryStrategy define la estrategia de reintento
type RetryStrategy struct {
	// Número máximo de reintentos
	MaxRetries int
	// Intervalo base entre reintentos (se duplicará con cada intento) - backoff exponencial
	BaseInterval time.Duration
	// Factor por el que se multiplica el intervalo en cada reintento
	Multiplier float64
	// Intervalo máximo entre reintentos
	MaxInterval time.Duration
	// Jitter agrega una variación aleatoria al tiempo de espera
	Jitter float64
}

// DefaultRetryStrategy es la estrategia de reintento predeterminada
var DefaultRetryStrategy = RetryStrategy{
	MaxRetries:   5,
	BaseInterval: 500 * time.Millisecond,
	Multiplier:   2.0,
	MaxInterval:  1 * time.Minute,
	Jitter:       0.2,
}

// RetryableTask es una tarea que se puede reintentar
type RetryableTask interface {
	// Execute ejecuta la tarea y devuelve un error si falla
	Execute(ctx context.Context) error
	// GetID devuelve un identificador único para la tarea
	GetID() string
	// GetRetryCount devuelve el número de reintentos realizados
	GetRetryCount() int
	// IncrementRetryCount incrementa el contador de reintentos
	IncrementRetryCount()
	// OnSuccess se llama cuando la tarea se ejecuta correctamente
	OnSuccess(ctx context.Context)
	// OnFailure se llama cuando la tarea falla después de agotar todos los reintentos
	OnFailure(ctx context.Context, err error)
}

// RetryManager gestiona los reintentos de tareas fallidas
type RetryManager struct {
	deliveryRepo     repository.DeliveryRepository
	notificationRepo repository.NotificationRepository
	strategy         RetryStrategy
	tasks            map[string]RetryableTask
	mu               sync.RWMutex
	logger           *logging.Logger
	dlq              *DeadLetterQueue
	stopCh           chan struct{}
	wg               sync.WaitGroup
}

// NewRetryManager crea una nueva instancia de RetryManager
func NewRetryManager(
	deliveryRepo repository.DeliveryRepository,
	notificationRepo repository.NotificationRepository,
	logger *logging.Logger,
	strategy *RetryStrategy,
) *RetryManager {
	// Si no se proporciona una estrategia, usar la predeterminada
	if strategy == nil {
		s := DefaultRetryStrategy
		strategy = &s
	}

	return &RetryManager{
		deliveryRepo:     deliveryRepo,
		notificationRepo: notificationRepo,
		strategy:         *strategy,
		tasks:            make(map[string]RetryableTask),
		logger:           logger,
		dlq:              NewDeadLetterQueue(logger, 0, 0),
		stopCh:           make(chan struct{}),
	}
}

// Start inicia el RetryManager
func (m *RetryManager) Start(ctx context.Context) {
	m.wg.Add(1)
	go m.processPendingDeliveries(ctx)
}

// Stop detiene el RetryManager
func (m *RetryManager) Stop() {
	close(m.stopCh)
	m.wg.Wait()
}

// AddTask agrega una tarea al RetryManager
func (m *RetryManager) AddTask(task RetryableTask) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.tasks[task.GetID()] = task

	// Programar la ejecución inicial
	go m.executeWithRetry(context.Background(), task)
}

// executeWithRetry ejecuta una tarea con reintentos según la estrategia configurada
func (m *RetryManager) executeWithRetry(ctx context.Context, task RetryableTask) {
	taskID := task.GetID()
	retryCount := task.GetRetryCount()

	// Si ya se han agotado los reintentos, pasar a la cola de mensajes muertos
	if retryCount >= m.strategy.MaxRetries {
		m.logger.Warn("Maximum retries reached for task %s, moving to DLQ", taskID)
		m.moveToDeadLetterQueue(ctx, task)
		return
	}

	err := task.Execute(ctx)
	if err == nil {
		// La tarea se ejecutó correctamente
		m.logger.Debug("Task %s executed successfully", taskID)
		task.OnSuccess(ctx)

		// Eliminar la tarea de la lista
		m.mu.Lock()
		delete(m.tasks, taskID)
		m.mu.Unlock()

		return
	}

	// La tarea falló, programar un reintento
	m.logger.Warn("Task %s failed, will retry: %v", taskID, err)
	task.IncrementRetryCount()

	// Calcular el tiempo de espera para el próximo reintento
	nextRetry := m.calculateNextRetryDelay(task.GetRetryCount())

	// Programar el reintento
	time.AfterFunc(nextRetry, func() {
		// Verificar si la tarea todavía existe antes de reintentarla
		m.mu.RLock()
		_, exists := m.tasks[taskID]
		m.mu.RUnlock()

		if exists {
			m.executeWithRetry(ctx, task)
		}
	})
}

// calculateNextRetryDelay calcula el tiempo de espera para el próximo reintento
func (m *RetryManager) calculateNextRetryDelay(retryCount int) time.Duration {
	// Aplicar backoff exponencial
	delay := m.strategy.BaseInterval * time.Duration(math.Pow(m.strategy.Multiplier, float64(retryCount)))

	// Aplicar límite máximo
	if delay > m.strategy.MaxInterval {
		delay = m.strategy.MaxInterval
	}

	// Aplicar jitter para evitar tormentas de reintentos
	if m.strategy.Jitter > 0 {
		jitter := float64(delay) * m.strategy.Jitter
		delay = time.Duration(float64(delay) - jitter/2 + jitter*float64(time.Now().UnixNano()%1000)/1000)
	}

	return delay
}

// moveToDeadLetterQueue mueve una tarea a la cola de mensajes muertos
func (m *RetryManager) moveToDeadLetterQueue(ctx context.Context, task RetryableTask) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Eliminar la tarea de la lista
	delete(m.tasks, task.GetID())

	// Agregar a la cola de mensajes muertos
	m.dlq.AddTask(task)

	// Llamar al manejador de error
	task.OnFailure(ctx, errors.New("maximum retries exceeded"))
}

// processPendingDeliveries procesa las entregas pendientes de la base de datos
func (m *RetryManager) processPendingDeliveries(ctx context.Context) {
	defer m.wg.Done()

	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopCh:
			return
		case <-ticker.C:
			// Obtener entregas pendientes
			deliveries, err := m.deliveryRepo.GetPendingForRetry(ctx, m.strategy.MaxRetries)
			if err != nil {
				m.logger.Error("Error getting pending deliveries: %v", err)
				continue
			}

			for _, delivery := range deliveries {
				// Obtener la notificación correspondiente
				notification, err := m.notificationRepo.GetByID(ctx, delivery.NotificationID)
				if err != nil {
					m.logger.Error("Error getting notification %s: %v", delivery.NotificationID, err)
					continue
				}

				// Crear una tarea de entrega
				task := &DeliveryTask{
					delivery:     delivery,
					notification: notification,
					deliveryRepo: m.deliveryRepo,
					logger:       m.logger,
				}

				// Agregar la tarea al RetryManager
				m.AddTask(task)
			}
		}
	}
}

// DeliveryTask es una implementación de RetryableTask para entregas de notificaciones
type DeliveryTask struct {
	delivery     *entity.DeliveryTracking
	notification *entity.Notification
	deliveryRepo repository.DeliveryRepository
	logger       *logging.Logger
	attempts     int
}

// Execute ejecuta la tarea de entrega
func (t *DeliveryTask) Execute(ctx context.Context) error {
	// En una implementación real, aquí se enviaría la notificación a través del canal correspondiente
	// Por ahora, simplemente simularemos el envío
	t.logger.Info("Retrying delivery of notification %s to device %s via %s (attempt %d)",
		t.delivery.NotificationID, t.delivery.DeviceID, t.delivery.Channel, t.GetRetryCount()+1)

	// Actualizar el estado de la entrega en la base de datos
	return t.deliveryRepo.UpdateStatus(ctx, t.delivery.ID, entity.DeliveryStatusPending)
}

// GetID devuelve un identificador único para la tarea
func (t *DeliveryTask) GetID() string {
	return fmt.Sprintf("delivery_%s", t.delivery.ID)
}

// GetRetryCount devuelve el número de reintentos realizados
func (t *DeliveryTask) GetRetryCount() int {
	return t.delivery.RetryCount
}

// IncrementRetryCount incrementa el contador de reintentos
func (t *DeliveryTask) IncrementRetryCount() {
	t.attempts++
}

// OnSuccess se llama cuando la tarea se ejecuta correctamente
func (t *DeliveryTask) OnSuccess(ctx context.Context) {
	// Marcar la entrega como exitosa en la base de datos
	t.deliveryRepo.MarkAsSent(ctx, t.delivery.ID)
	t.logger.Info("Successfully delivered notification %s to device %s",
		t.delivery.NotificationID, t.delivery.DeviceID)
}

// OnFailure se llama cuando la tarea falla después de agotar todos los reintentos
func (t *DeliveryTask) OnFailure(ctx context.Context, err error) {
	// Marcar la entrega como fallida en la base de datos
	t.deliveryRepo.MarkAsFailed(ctx, t.delivery.ID, err.Error())
	t.logger.Error("Failed to deliver notification %s to device %s after %d attempts: %v",
		t.delivery.NotificationID, t.delivery.DeviceID, t.GetRetryCount(), err)
}

// DeadLetterQueue es una cola para mensajes que no pudieron ser entregados después de múltiples intentos
/* type DeadLetterQueue struct {
	tasks  map[string]RetryableTask
	mu     sync.RWMutex
	logger *logging.Logger
} */

// NewDeadLetterQueue crea una nueva instancia de DeadLetterQueue
/* func NewDeadLetterQueue(logger *logging.Logger) *DeadLetterQueue {
	return &DeadLetterQueue{
		tasks:  make(map[string]RetryableTask),
		logger: logger,
	}
}
*/
// AddTask agrega una tarea a la cola de mensajes muertos
/* func (q *DeadLetterQueue) AddTask(task RetryableTask) {
	q.mu.Lock()
	defer q.mu.Unlock()

	taskID := task.GetID()
	q.tasks[taskID] = task
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

// RemoveTask elimina una tarea de la cola de mensajes muertos
func (q *DeadLetterQueue) RemoveTask(taskID string) {
	q.mu.Lock()
	defer q.mu.Unlock()

	delete(q.tasks, taskID)
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
} */
