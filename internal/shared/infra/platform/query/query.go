package query

// ---------- Tipos de filtrado / paginación / ordenamiento ----------

// OffsetPagination para paginación clásica
type OffsetPagination struct {
	Limit  int
	Offset int
}

// CursorPagination para paginación tipo cursor
type CursorPagination struct {
	Limit     int
	Cursor    string // puede ser un timestamp o UUID serializado
	SortField string
	SortDesc  bool
}

// Interfaz genérica para paginación
type Pagination interface{}

// Sort indica campo y dirección.
type Sort struct {
	Field string // ej. "created_at", "nombre", "email"
	Desc  bool
}
