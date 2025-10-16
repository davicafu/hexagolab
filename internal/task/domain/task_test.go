package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

// TestTask_Complete valida que el método Complete() funcione correctamente.
func TestTask_Complete(t *testing.T) {
	// Arrange: Preparamos el estado inicial del objeto.
	task := &Task{
		ID:        uuid.New(),
		Status:    TaskPending,
		UpdatedAt: time.Now().UTC().Add(-1 * time.Hour), // Una hora en el pasado
	}
	initialUpdateTime := task.UpdatedAt

	// Act: Ejecutamos el método que queremos probar.
	task.Complete()

	// Assert: Verificamos que el resultado es el esperado.
	assert.Equal(t, TaskCompleted, task.Status, "El estado debería ser 'completed'")
	assert.True(t, task.UpdatedAt.After(initialUpdateTime), "La fecha de actualización (UpdatedAt) debería haberse modificado")
}

// TestTask_Fail valida que el método Fail() funcione correctamente.
func TestTask_Fail(t *testing.T) {
	// Arrange
	task := &Task{
		ID:        uuid.New(),
		Status:    TaskPending,
		UpdatedAt: time.Now().UTC().Add(-1 * time.Hour),
	}
	initialUpdateTime := task.UpdatedAt

	// Act
	task.Fail()

	// Assert
	assert.Equal(t, TaskFailed, task.Status, "El estado debería ser 'failed'")
	assert.True(t, task.UpdatedAt.After(initialUpdateTime), "La fecha de actualización (UpdatedAt) debería haberse modificado")
}

// TestTask_Update valida que el método Update() actualice los campos correctos.
func TestTask_Update(t *testing.T) {
	// Arrange
	task := &Task{
		ID:          uuid.New(),
		Title:       "Título Original",
		Description: "Descripción Original",
		UpdatedAt:   time.Now().UTC().Add(-1 * time.Hour),
	}
	initialUpdateTime := task.UpdatedAt
	newTitle := "Título Actualizado"
	newDescription := "Descripción Actualizada"

	// Act
	task.Update(newTitle, newDescription)

	// Assert
	assert.Equal(t, newTitle, task.Title, "El título debería haberse actualizado")
	assert.Equal(t, newDescription, task.Description, "La descripción debería haberse actualizado")
	assert.True(t, task.UpdatedAt.After(initialUpdateTime), "La fecha de actualización (UpdatedAt) debería haberse modificado")
}
