package filter

import (
	"fmt"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/kqns91/kube-watcher/pkg/watcher"
)

// CELFilter represents a CEL-based filter
type CELFilter struct {
	expression string
	program    cel.Program
}

// NewCELFilter creates a new CEL filter from an expression
func NewCELFilter(expression string) (*CELFilter, error) {
	// Create CEL environment with event variable
	env, err := cel.NewEnv(
		cel.Variable("event", cel.DynType),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create CEL environment: %w", err)
	}

	// Parse and check the expression
	ast, issues := env.Compile(expression)
	if issues != nil && issues.Err() != nil {
		return nil, fmt.Errorf("failed to compile CEL expression: %w", issues.Err())
	}

	// Create program
	program, err := env.Program(ast)
	if err != nil {
		return nil, fmt.Errorf("failed to create CEL program: %w", err)
	}

	return &CELFilter{
		expression: expression,
		program:    program,
	}, nil
}

// Evaluate evaluates the CEL expression against an event
func (f *CELFilter) Evaluate(event *watcher.Event) (bool, error) {
	// Convert event to CEL-compatible map
	eventMap := eventToMap(event)

	// Evaluate the expression
	out, _, err := f.program.Eval(map[string]interface{}{
		"event": eventMap,
	})
	if err != nil {
		return false, fmt.Errorf("failed to evaluate CEL expression: %w", err)
	}

	// Convert result to boolean
	result, ok := out.(types.Bool)
	if !ok {
		return false, fmt.Errorf("CEL expression did not return a boolean value")
	}

	return bool(result), nil
}

// eventToMap converts a watcher.Event to a map for CEL evaluation
func eventToMap(event *watcher.Event) map[string]interface{} {
	m := map[string]interface{}{
		"kind":      event.Kind,
		"namespace": event.Namespace,
		"name":      event.Name,
		"eventType": event.EventType,
		"labels":    event.Labels,
		"reason":    event.Reason,
		"message":   event.Message,
		"status":    event.Status,
	}

	// Add replicas info if available
	if event.Replicas != nil {
		m["replicas"] = map[string]interface{}{
			"desired": event.Replicas.Desired,
			"ready":   event.Replicas.Ready,
			"current": event.Replicas.Current,
		}
	}

	// Add containers info if available
	if len(event.Containers) > 0 {
		containers := make([]map[string]interface{}, len(event.Containers))
		for i, c := range event.Containers {
			containers[i] = map[string]interface{}{
				"name":  c.Name,
				"image": c.Image,
			}
		}
		m["containers"] = containers
	}

	// Add service type if available
	if event.ServiceType != "" {
		m["serviceType"] = event.ServiceType
	}

	return m
}

// Expression returns the CEL expression string
func (f *CELFilter) Expression() string {
	return f.expression
}

// EvaluateExpression evaluates a simple CEL expression for testing/validation purposes
func EvaluateExpression(expression string, event *watcher.Event) (ref.Val, error) {
	filter, err := NewCELFilter(expression)
	if err != nil {
		return nil, err
	}

	eventMap := eventToMap(event)
	out, _, err := filter.program.Eval(map[string]interface{}{
		"event": eventMap,
	})
	return out, err
}
