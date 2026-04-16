package core

import (
	"fmt"
	"math"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/ekaya-inc/dataclaw/pkg/models"
)

func prepareExecutionParameterValues(paramDefs []models.QueryParameter, suppliedParams map[string]any) (map[string]any, error) {
	if err := validateRequiredParameters(paramDefs, suppliedParams); err != nil {
		return nil, err
	}

	coerced, err := coerceParameterTypes(paramDefs, suppliedParams)
	if err != nil {
		return nil, err
	}

	effective, err := applyParameterDefaults(paramDefs, coerced)
	if err != nil {
		return nil, err
	}

	return effective, nil
}

func validateRequiredParameters(paramDefs []models.QueryParameter, suppliedParams map[string]any) error {
	for _, param := range paramDefs {
		if !param.Required {
			continue
		}

		value, exists := suppliedParams[param.Name]
		if exists && hasProvidedQueryParameterValue(value) {
			continue
		}

		if !queryParameterHasDefault(param) {
			return fmt.Errorf("required parameter '%s' is missing", param.Name)
		}
	}

	return nil
}

func coerceParameterTypes(paramDefs []models.QueryParameter, suppliedParams map[string]any) (map[string]any, error) {
	coerced := make(map[string]any, len(suppliedParams))
	defLookup := make(map[string]models.QueryParameter, len(paramDefs))
	for _, param := range paramDefs {
		defLookup[param.Name] = param
	}

	for name, value := range suppliedParams {
		def, exists := defLookup[name]
		if !exists {
			return nil, fmt.Errorf("unknown parameter '%s'", name)
		}

		if value == nil {
			continue
		}

		coercedValue, err := coerceValue(value, def.Type, name)
		if err != nil {
			return nil, err
		}
		coerced[name] = coercedValue
	}

	return coerced, nil
}

func applyParameterDefaults(paramDefs []models.QueryParameter, suppliedParams map[string]any) (map[string]any, error) {
	effective := make(map[string]any, len(suppliedParams))
	for name, value := range suppliedParams {
		effective[name] = value
	}

	for _, param := range paramDefs {
		if _, exists := effective[param.Name]; exists {
			continue
		}
		if param.Default == nil {
			continue
		}

		coercedDefault, err := coerceValue(param.Default, param.Type, param.Name)
		if err != nil {
			return nil, err
		}
		effective[param.Name] = coercedDefault
	}

	return effective, nil
}

func hasArrayParameters(paramDefs []models.QueryParameter, suppliedParams map[string]any) bool {
	for _, param := range paramDefs {
		if !isArrayParameterType(param.Type) {
			continue
		}
		if _, exists := suppliedParams[param.Name]; exists {
			return true
		}
	}
	return false
}

func isArrayParameterType(paramType string) bool {
	switch paramType {
	case "string[]", "integer[]":
		return true
	default:
		return false
	}
}

func queryParameterHasDefault(param models.QueryParameter) bool {
	return param.Default != nil
}

func hasProvidedQueryParameterValue(value any) bool {
	if value == nil {
		return false
	}

	if strValue, ok := value.(string); ok {
		return strings.TrimSpace(strValue) != ""
	}

	reflectValue := reflect.ValueOf(value)
	switch reflectValue.Kind() {
	case reflect.Array, reflect.Map, reflect.Slice:
		return reflectValue.Len() > 0
	}

	return true
}

func coerceValue(value any, targetType string, paramName string) (any, error) {
	switch targetType {
	case "string":
		return coerceToString(value), nil
	case "integer":
		return coerceToInteger(value, paramName)
	case "decimal":
		return coerceToDecimal(value, paramName)
	case "boolean":
		return coerceToBoolean(value, paramName)
	case "date":
		return coerceToDate(value, paramName)
	case "timestamp":
		return coerceToTimestamp(value, paramName)
	case "uuid":
		return coerceToUUID(value, paramName)
	case "string[]":
		return coerceToStringArray(value, paramName)
	case "integer[]":
		return coerceToIntegerArray(value, paramName)
	default:
		return nil, fmt.Errorf("unsupported parameter type '%s' for parameter '%s'", targetType, paramName)
	}
}

func coerceToString(value any) string {
	if str, ok := value.(string); ok {
		return str
	}
	return fmt.Sprintf("%v", value)
}

func coerceToInteger(value any, paramName string) (int64, error) {
	switch typed := value.(type) {
	case int:
		return int64(typed), nil
	case int8:
		return int64(typed), nil
	case int16:
		return int64(typed), nil
	case int32:
		return int64(typed), nil
	case int64:
		return typed, nil
	case float32:
		if math.Trunc(float64(typed)) != float64(typed) {
			return 0, fmt.Errorf("parameter '%s': cannot convert non-integer number %v to integer", paramName, typed)
		}
		return int64(typed), nil
	case float64:
		if math.Trunc(typed) != typed {
			return 0, fmt.Errorf("parameter '%s': cannot convert non-integer number %v to integer", paramName, typed)
		}
		return int64(typed), nil
	case string:
		parsed, err := strconv.ParseInt(strings.TrimSpace(typed), 10, 64)
		if err != nil {
			return 0, fmt.Errorf("parameter '%s': cannot convert %q to integer: %w", paramName, typed, err)
		}
		return parsed, nil
	default:
		return 0, fmt.Errorf("parameter '%s': cannot convert type %T to integer", paramName, value)
	}
}

func coerceToDecimal(value any, paramName string) (float64, error) {
	switch typed := value.(type) {
	case float32:
		return float64(typed), nil
	case float64:
		return typed, nil
	case int:
		return float64(typed), nil
	case int8:
		return float64(typed), nil
	case int16:
		return float64(typed), nil
	case int32:
		return float64(typed), nil
	case int64:
		return float64(typed), nil
	case string:
		parsed, err := strconv.ParseFloat(strings.TrimSpace(typed), 64)
		if err != nil {
			return 0, fmt.Errorf("parameter '%s': cannot convert %q to decimal: %w", paramName, typed, err)
		}
		return parsed, nil
	default:
		return 0, fmt.Errorf("parameter '%s': cannot convert type %T to decimal", paramName, value)
	}
}

func coerceToBoolean(value any, paramName string) (bool, error) {
	switch typed := value.(type) {
	case bool:
		return typed, nil
	case string:
		parsed, err := strconv.ParseBool(strings.TrimSpace(typed))
		if err != nil {
			return false, fmt.Errorf("parameter '%s': cannot convert %q to boolean: %w", paramName, typed, err)
		}
		return parsed, nil
	case int:
		return typed != 0, nil
	case int8:
		return typed != 0, nil
	case int16:
		return typed != 0, nil
	case int32:
		return typed != 0, nil
	case int64:
		return typed != 0, nil
	case float32:
		return typed != 0, nil
	case float64:
		return typed != 0, nil
	default:
		return false, fmt.Errorf("parameter '%s': cannot convert type %T to boolean", paramName, value)
	}
}

func coerceToDate(value any, paramName string) (string, error) {
	switch typed := value.(type) {
	case time.Time:
		return typed.UTC().Format("2006-01-02"), nil
	default:
		str := coerceToString(value)
		if _, err := time.Parse("2006-01-02", str); err != nil {
			return "", fmt.Errorf("parameter '%s': invalid date format %q, expected YYYY-MM-DD: %w", paramName, str, err)
		}
		return str, nil
	}
}

func coerceToTimestamp(value any, paramName string) (string, error) {
	switch typed := value.(type) {
	case time.Time:
		return typed.UTC().Format(time.RFC3339), nil
	default:
		str := coerceToString(value)
		if _, err := time.Parse(time.RFC3339, str); err != nil {
			return "", fmt.Errorf("parameter '%s': invalid timestamp format %q, expected RFC3339: %w", paramName, str, err)
		}
		return str, nil
	}
}

func coerceToUUID(value any, paramName string) (string, error) {
	str := coerceToString(value)
	if _, err := uuid.Parse(str); err != nil {
		return "", fmt.Errorf("parameter '%s': invalid UUID format %q: %w", paramName, str, err)
	}
	return str, nil
}

func coerceToStringArray(value any, paramName string) ([]string, error) {
	switch typed := value.(type) {
	case []string:
		return typed, nil
	case []any:
		result := make([]string, len(typed))
		for i, item := range typed {
			result[i] = coerceToString(item)
		}
		return result, nil
	case string:
		return splitCommaSeparated(typed), nil
	default:
		return nil, fmt.Errorf("parameter '%s': cannot convert type %T to string array", paramName, value)
	}
}

func coerceToIntegerArray(value any, paramName string) ([]int64, error) {
	switch typed := value.(type) {
	case []int64:
		return typed, nil
	case []int:
		result := make([]int64, len(typed))
		for i, item := range typed {
			result[i] = int64(item)
		}
		return result, nil
	case []any:
		result := make([]int64, len(typed))
		for i, item := range typed {
			coerced, err := coerceToInteger(item, fmt.Sprintf("%s[%d]", paramName, i))
			if err != nil {
				return nil, err
			}
			result[i] = coerced
		}
		return result, nil
	case string:
		parts := splitCommaSeparated(typed)
		result := make([]int64, len(parts))
		for i, item := range parts {
			coerced, err := coerceToInteger(item, fmt.Sprintf("%s[%d]", paramName, i))
			if err != nil {
				return nil, err
			}
			result[i] = coerced
		}
		return result, nil
	default:
		return nil, fmt.Errorf("parameter '%s': cannot convert type %T to integer array", paramName, value)
	}
}

func splitCommaSeparated(value string) []string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return []string{}
	}

	rawParts := strings.Split(trimmed, ",")
	parts := make([]string, 0, len(rawParts))
	for _, part := range rawParts {
		normalized := strings.TrimSpace(part)
		if normalized == "" {
			continue
		}
		parts = append(parts, normalized)
	}
	return parts
}
