package core

import (
	"reflect"
	"testing"

	"github.com/ekaya-inc/dataclaw/pkg/models"
)

func TestPrepareReadOnlyParameterizedQueryUsesDefaults(t *testing.T) {
	sqlQuery := "SELECT * FROM orders WHERE total > {{min_total}}"
	params := []models.QueryParameter{
		{Name: "min_total", Type: "decimal", Required: false, Default: 0.0},
	}

	prepared, args, err := prepareReadOnlyParameterizedQuery("postgres", sqlQuery, params, nil)
	if err != nil {
		t.Fatalf("prepareReadOnlyParameterizedQuery: %v", err)
	}
	if prepared != "SELECT * FROM orders WHERE total > $1" {
		t.Fatalf("unexpected prepared SQL: %q", prepared)
	}
	if !reflect.DeepEqual(args, []any{0.0}) {
		t.Fatalf("unexpected args: %#v", args)
	}
}

func TestPrepareReadOnlyParameterizedQueryRejectsMissingRequiredParameter(t *testing.T) {
	sqlQuery := "SELECT * FROM orders WHERE customer_id = {{customer_id}}"
	params := []models.QueryParameter{
		{Name: "customer_id", Type: "uuid", Required: true},
	}

	_, _, err := prepareReadOnlyParameterizedQuery("postgres", sqlQuery, params, nil)
	if err == nil {
		t.Fatal("expected missing required parameter error")
	}
	if err.Error() != "missing required parameter: customer_id" {
		t.Fatalf("unexpected error: %v", err)
	}
}
