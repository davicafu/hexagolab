// en internal/infra/storage/filesystem/task_storage.go
package filesystem

import (
	"context"
	"encoding/json"
	"os"
	"sync"

	taskDomain "github.com/davicafu/hexagolab/internal/task/domain"
	"github.com/google/uuid"
)

// JSONTaskStorage es un adaptador outbound que guarda las tareas en un fichero JSON.
type JSONTaskStorage struct {
	filePath string
	mu       sync.Mutex // Mutex para evitar race conditions al leer/escribir el archivo.
}

// NewJSONTaskStorage es el constructor.
func NewJSONTaskStorage(filePath string) *JSONTaskStorage {
	return &JSONTaskStorage{
		filePath: filePath,
	}
}

// Save añade una nueva tarea al fichero JSON.
// Si el fichero no existe, lo crea.
func (s *JSONTaskStorage) Save(ctx context.Context, task *taskDomain.Task) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 1. Leer todas las tareas existentes.
	tasks, err := s.getAllTasksFromFile()
	// Si el error es que el fichero no existe, empezamos con una lista vacía.
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	// 2. Añadir la nueva tarea a la lista.
	tasks = append(tasks, task)

	// 3. Serializar la lista completa a JSON con formato indentado.
	data, err := json.MarshalIndent(tasks, "", "  ")
	if err != nil {
		return err
	}

	// 4. Escribir (sobrescribiendo) el fichero completo.
	return os.WriteFile(s.filePath, data, 0644)
}

// GetAll recupera todas las tareas del fichero JSON.
func (s *JSONTaskStorage) GetAll(ctx context.Context) ([]*taskDomain.Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.getAllTasksFromFile()
}

// getTaskByID busca una tarea por su ID en el fichero. (Ejemplo adicional)
func (s *JSONTaskStorage) GetTaskByID(ctx context.Context, id uuid.UUID) (*taskDomain.Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	tasks, err := s.getAllTasksFromFile()
	if err != nil {
		return nil, err
	}

	for _, task := range tasks {
		if task.ID == id {
			return task, nil
		}
	}

	return nil, taskDomain.ErrTaskNotFound // Reutilizamos el error de dominio
}

// getAllTasksFromFile es un helper interno no concurrente.
func (s *JSONTaskStorage) getAllTasksFromFile() ([]*taskDomain.Task, error) {
	data, err := os.ReadFile(s.filePath)
	if err != nil {
		// Si el fichero no existe, devolvemos una lista vacía sin error.
		if os.IsNotExist(err) {
			return []*taskDomain.Task{}, nil
		}
		return nil, err
	}

	// Si el fichero está vacío, también devolvemos una lista vacía.
	if len(data) == 0 {
		return []*taskDomain.Task{}, nil
	}

	var tasks []*taskDomain.Task
	if err := json.Unmarshal(data, &tasks); err != nil {
		return nil, err
	}

	return tasks, nil
}
