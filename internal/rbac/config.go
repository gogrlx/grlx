package rbac

import (
	"fmt"

	"github.com/taigrr/jety"
)

// LoadCohortsFromConfig reads the "cohorts" section from the farmer config
// (via jety) and returns a populated Registry. It does not fail on an
// empty or missing cohorts section — it simply returns an empty registry.
func LoadCohortsFromConfig() (*Registry, error) {
	registry := NewRegistry()

	raw := jety.GetStringMap("cohorts")
	if len(raw) == 0 {
		return registry, nil
	}

	for name, v := range raw {
		cohort, err := parseCohortEntry(name, v)
		if err != nil {
			return nil, fmt.Errorf("parsing cohort %q: %w", name, err)
		}
		if err := registry.Register(cohort); err != nil {
			return nil, fmt.Errorf("registering cohort %q: %w", name, err)
		}
	}

	return registry, nil
}

func parseCohortEntry(name string, raw any) (*Cohort, error) {
	m, ok := raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%w: cohort %q value is not a map", ErrInvalidCohort, name)
	}

	cohort := &Cohort{Name: name}

	typeStr, _ := m["type"].(string)
	switch CohortType(typeStr) {
	case CohortTypeStatic:
		cohort.Type = CohortTypeStatic
		cohort.Members = parseStringSlice(m["members"])
	case CohortTypeDynamic:
		cohort.Type = CohortTypeDynamic
		match, err := parseDynamicMatch(m["match"])
		if err != nil {
			return nil, fmt.Errorf("cohort %q: %w", name, err)
		}
		cohort.Match = match
	case CohortTypeCompound:
		cohort.Type = CohortTypeCompound
		compound, err := parseCompoundExpr(m["compound"])
		if err != nil {
			return nil, fmt.Errorf("cohort %q: %w", name, err)
		}
		cohort.Compound = compound
	default:
		return nil, fmt.Errorf("%w: unknown type %q for cohort %q", ErrInvalidCohort, typeStr, name)
	}

	return cohort, nil
}

func parseStringSlice(v any) []string {
	if v == nil {
		return nil
	}
	switch s := v.(type) {
	case []any:
		result := make([]string, 0, len(s))
		for _, item := range s {
			if str, ok := item.(string); ok {
				result = append(result, str)
			}
		}
		return result
	case []string:
		return s
	default:
		return nil
	}
}

func parseDynamicMatch(v any) (*DynamicMatch, error) {
	if v == nil {
		return nil, fmt.Errorf("%w: dynamic match is nil", ErrInvalidCohort)
	}
	m, ok := v.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%w: match is not a map", ErrInvalidCohort)
	}
	propName, _ := m["prop_name"].(string)
	propValue, _ := m["prop_value"].(string)
	if propName == "" {
		return nil, fmt.Errorf("%w: match requires prop_name", ErrInvalidCohort)
	}
	return &DynamicMatch{
		PropName:  propName,
		PropValue: propValue,
	}, nil
}

func parseCompoundExpr(v any) (*CompoundExpr, error) {
	if v == nil {
		return nil, fmt.Errorf("%w: compound expression is nil", ErrInvalidCohort)
	}
	m, ok := v.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%w: compound is not a map", ErrInvalidCohort)
	}
	opStr, _ := m["operator"].(string)
	op := Operator(opStr)
	if err := validateOperator(op); err != nil {
		return nil, err
	}
	operands := parseStringSlice(m["operands"])
	if len(operands) < 2 {
		return nil, fmt.Errorf("%w", ErrMissingOperands)
	}
	return &CompoundExpr{
		Operator: op,
		Operands: operands,
	}, nil
}
