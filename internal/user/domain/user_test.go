package domain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestUser_Age(t *testing.T) {
	tests := []struct {
		name     string
		birth    time.Time
		expected int
	}{
		{
			name:     "cumpleaños ya pasado este año",
			birth:    time.Date(time.Now().Year()-30, 1, 1, 0, 0, 0, 0, time.UTC),
			expected: 30,
		},
		{
			name:     "cumpleaños aún no ha pasado este año",
			birth:    time.Date(time.Now().Year()-25, 12, 31, 0, 0, 0, 0, time.UTC),
			expected: 24, // aún no cumplió 25 este año
		},
		{
			name:     "cumpleaños hoy",
			birth:    time.Date(time.Now().Year()-40, time.Now().Month(), time.Now().Day(), 0, 0, 0, 0, time.UTC),
			expected: 40,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user := &User{
				Email:     "test@example.com",
				Nombre:    "Test",
				BirthDate: tt.birth,
			}
			assert.Equal(t, tt.expected, user.Age())
		})
	}
}
