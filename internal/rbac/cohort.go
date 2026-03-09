// Package rbac provides role-based access control for grlx, including
// cohort definitions that group sprouts by static membership, dynamic
// property matching, or boolean combinations of other cohorts.
package rbac

import (
	"errors"
	"fmt"
	"strings"

	"github.com/gogrlx/grlx/v2/internal/props"
)

// CohortType distinguishes how a cohort's membership is determined.
type CohortType string

const (
	// CohortTypeStatic contains an explicit list of sprout IDs.
	CohortTypeStatic CohortType = "static"
	// CohortTypeDynamic matches sprouts whose props satisfy a condition.
	CohortTypeDynamic CohortType = "dynamic"
	// CohortTypeCompound combines other cohorts with boolean logic.
	CohortTypeCompound CohortType = "compound"
)

// Operator defines boolean combinators for compound cohorts.
type Operator string

const (
	// OperatorAND returns the intersection of its operands.
	OperatorAND Operator = "AND"
	// OperatorOR returns the union of its operands.
	OperatorOR Operator = "OR"
	// OperatorEXCEPT returns Left minus Right.
	OperatorEXCEPT Operator = "EXCEPT"
)

var (
	ErrCohortNotFound    = errors.New("cohort not found")
	ErrCircularReference = errors.New("circular cohort reference detected")
	ErrInvalidCohort     = errors.New("invalid cohort definition")
	ErrInvalidOperator   = errors.New("invalid compound operator")
	ErrMissingOperands   = errors.New("compound cohort requires at least two operands")
)

// DynamicMatch describes a property-based membership rule.
// A sprout is a member if its property named PropName equals PropValue.
type DynamicMatch struct {
	PropName  string `json:"propName" yaml:"prop_name"`
	PropValue string `json:"propValue" yaml:"prop_value"`
}

// CompoundExpr describes a boolean combination of named cohorts.
type CompoundExpr struct {
	Operator Operator `json:"operator" yaml:"operator"`
	Operands []string `json:"operands" yaml:"operands"`
}

// Cohort is the primary unit of sprout grouping in grlx RBAC.
type Cohort struct {
	Name     string        `json:"name" yaml:"name"`
	Type     CohortType    `json:"type" yaml:"type"`
	Members  []string      `json:"members,omitempty" yaml:"members,omitempty"`
	Match    *DynamicMatch `json:"match,omitempty" yaml:"match,omitempty"`
	Compound *CompoundExpr `json:"compound,omitempty" yaml:"compound,omitempty"`
}

// Validate checks that the cohort definition is internally consistent.
func (c *Cohort) Validate() error {
	if c.Name == "" {
		return fmt.Errorf("%w: name is required", ErrInvalidCohort)
	}
	switch c.Type {
	case CohortTypeStatic:
		// Members list may be empty (no sprouts yet).
		return nil
	case CohortTypeDynamic:
		if c.Match == nil {
			return fmt.Errorf("%w: dynamic cohort %q requires a match rule", ErrInvalidCohort, c.Name)
		}
		if c.Match.PropName == "" {
			return fmt.Errorf("%w: dynamic cohort %q match requires prop_name", ErrInvalidCohort, c.Name)
		}
		return nil
	case CohortTypeCompound:
		if c.Compound == nil {
			return fmt.Errorf("%w: compound cohort %q requires a compound expression", ErrInvalidCohort, c.Name)
		}
		if err := validateOperator(c.Compound.Operator); err != nil {
			return fmt.Errorf("%w: cohort %q: %w", ErrInvalidCohort, c.Name, err)
		}
		if len(c.Compound.Operands) < 2 {
			return fmt.Errorf("%w: cohort %q", ErrMissingOperands, c.Name)
		}
		return nil
	default:
		return fmt.Errorf("%w: unknown type %q for cohort %q", ErrInvalidCohort, c.Type, c.Name)
	}
}

func validateOperator(op Operator) error {
	switch op {
	case OperatorAND, OperatorOR, OperatorEXCEPT:
		return nil
	default:
		return fmt.Errorf("%w: %q", ErrInvalidOperator, op)
	}
}

// Registry holds named cohorts and resolves membership queries.
type Registry struct {
	cohorts map[string]*Cohort
}

// NewRegistry creates an empty cohort registry.
func NewRegistry() *Registry {
	return &Registry{cohorts: make(map[string]*Cohort)}
}

// Register adds a cohort to the registry, replacing any existing cohort
// with the same name. It validates the cohort before registration.
func (r *Registry) Register(c *Cohort) error {
	if err := c.Validate(); err != nil {
		return err
	}
	r.cohorts[c.Name] = c
	return nil
}

// Get returns a cohort by name.
func (r *Registry) Get(name string) (*Cohort, error) {
	c, ok := r.cohorts[name]
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrCohortNotFound, name)
	}
	return c, nil
}

// List returns the names of all registered cohorts.
func (r *Registry) List() []string {
	names := make([]string, 0, len(r.cohorts))
	for name := range r.cohorts {
		names = append(names, name)
	}
	return names
}

// Resolve evaluates a named cohort and returns the set of sprout IDs
// that belong to it. allSproutIDs must contain every known sprout ID
// (needed to evaluate dynamic cohorts). It detects circular references.
func (r *Registry) Resolve(name string, allSproutIDs []string) (map[string]bool, error) {
	visited := make(map[string]bool)
	return r.resolve(name, allSproutIDs, visited)
}

func (r *Registry) resolve(name string, allSproutIDs []string, visited map[string]bool) (map[string]bool, error) {
	if visited[name] {
		return nil, fmt.Errorf("%w: %q", ErrCircularReference, name)
	}
	visited[name] = true

	c, ok := r.cohorts[name]
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrCohortNotFound, name)
	}

	switch c.Type {
	case CohortTypeStatic:
		return resolveStatic(c), nil
	case CohortTypeDynamic:
		return resolveDynamic(c, allSproutIDs), nil
	case CohortTypeCompound:
		return r.resolveCompound(c, allSproutIDs, visited)
	default:
		return nil, fmt.Errorf("%w: unknown type %q", ErrInvalidCohort, c.Type)
	}
}

func resolveStatic(c *Cohort) map[string]bool {
	result := make(map[string]bool, len(c.Members))
	for _, id := range c.Members {
		result[id] = true
	}
	return result
}

func resolveDynamic(c *Cohort, allSproutIDs []string) map[string]bool {
	result := make(map[string]bool)
	for _, sproutID := range allSproutIDs {
		getProp := props.GetStringPropFunc(sproutID)
		val := getProp(c.Match.PropName)
		if matchesPropValue(val, c.Match.PropValue) {
			result[sproutID] = true
		}
	}
	return result
}

// matchesPropValue checks if a sprout's property value matches the expected value.
// It supports exact match and glob-style prefix/suffix wildcards.
func matchesPropValue(actual, expected string) bool {
	if expected == "*" {
		return actual != ""
	}
	if strings.HasPrefix(expected, "*") && strings.HasSuffix(expected, "*") {
		return strings.Contains(actual, expected[1:len(expected)-1])
	}
	if strings.HasPrefix(expected, "*") {
		return strings.HasSuffix(actual, expected[1:])
	}
	if strings.HasSuffix(expected, "*") {
		return strings.HasPrefix(actual, expected[:len(expected)-1])
	}
	return actual == expected
}

func (r *Registry) resolveCompound(c *Cohort, allSproutIDs []string, visited map[string]bool) (map[string]bool, error) {
	if len(c.Compound.Operands) < 2 {
		return nil, fmt.Errorf("%w: cohort %q", ErrMissingOperands, c.Name)
	}

	// Resolve first operand.
	// Copy visited map for each operand to allow shared references (but
	// still detect direct cycles through our own name).
	result, err := r.resolve(c.Compound.Operands[0], allSproutIDs, copyVisited(visited))
	if err != nil {
		return nil, fmt.Errorf("resolving operand %q of cohort %q: %w", c.Compound.Operands[0], c.Name, err)
	}

	for _, operandName := range c.Compound.Operands[1:] {
		operandSet, err := r.resolve(operandName, allSproutIDs, copyVisited(visited))
		if err != nil {
			return nil, fmt.Errorf("resolving operand %q of cohort %q: %w", operandName, c.Name, err)
		}

		switch c.Compound.Operator {
		case OperatorAND:
			result = intersect(result, operandSet)
		case OperatorOR:
			result = union(result, operandSet)
		case OperatorEXCEPT:
			result = except(result, operandSet)
		}
	}

	return result, nil
}

func copyVisited(m map[string]bool) map[string]bool {
	c := make(map[string]bool, len(m))
	for k, v := range m {
		c[k] = v
	}
	return c
}

func intersect(a, b map[string]bool) map[string]bool {
	result := make(map[string]bool)
	for id := range a {
		if b[id] {
			result[id] = true
		}
	}
	return result
}

func union(a, b map[string]bool) map[string]bool {
	result := make(map[string]bool, len(a)+len(b))
	for id := range a {
		result[id] = true
	}
	for id := range b {
		result[id] = true
	}
	return result
}

func except(a, b map[string]bool) map[string]bool {
	result := make(map[string]bool, len(a))
	for id := range a {
		if !b[id] {
			result[id] = true
		}
	}
	return result
}
