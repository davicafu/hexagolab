package domain

// ---------------- Operadores ----------------

type Operator string

const (
	OpEq    Operator = "="
	OpGt    Operator = ">"
	OpGte   Operator = ">="
	OpLt    Operator = "<"
	OpLte   Operator = "<="
	OpLike  Operator = "LIKE"
	OpILike Operator = "ILIKE"
)

type LogicalOperator string

const (
	OpAnd LogicalOperator = "AND"
	OpOr  LogicalOperator = "OR"
)

// ---------------- Criterion ----------------

// Criterion describe una condición neutral de filtrado
type Criterion struct {
	Field string
	Op    Operator
	Value interface{}
}

// ---------------- Criteria interface ----------------

// Criteria permite transformar filtros a condiciones neutrales
type Criteria interface {
	ToConditions() []Criterion
}

// ---------------- Composite Criteria ----------------

type CompositeCriteria struct {
	Operator  LogicalOperator
	Criterias []Criteria
}

func (c CompositeCriteria) ToConditions() []Criterion {
	var all []Criterion
	for _, crit := range c.Criterias {
		all = append(all, crit.ToConditions()...)
	}
	return all
}

// ---------------- Helpers ----------------

// And crea un CompositeCriteria con operador AND
func And(criterias ...Criteria) CompositeCriteria {
	return CompositeCriteria{Operator: OpAnd, Criterias: criterias}
}

// Or crea un CompositeCriteria con operador OR
func Or(criterias ...Criteria) CompositeCriteria {
	return CompositeCriteria{Operator: OpOr, Criterias: criterias}
}
