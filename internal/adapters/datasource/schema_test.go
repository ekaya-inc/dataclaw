package datasource

import "testing"

func TestSchemaExploreRequestNormalizedDefaultsAndTrimsFilters(t *testing.T) {
	got := (SchemaExploreRequest{
		SchemaName: " public ",
		ObjectName: " accounts ",
	}).Normalized()

	if got.SchemaName != "public" || got.ObjectName != "accounts" {
		t.Fatalf("expected trimmed filters, got %#v", got)
	}
	if got.DetailMode != SchemaDetailModeCompact {
		t.Fatalf("expected compact detail mode default, got %q", got.DetailMode)
	}
}

func TestSchemaExploreRequestNormalizedPreservesFullDetailMode(t *testing.T) {
	got := (SchemaExploreRequest{DetailMode: SchemaDetailModeFull}).Normalized()
	if got.DetailMode != SchemaDetailModeFull {
		t.Fatalf("expected full detail mode, got %q", got.DetailMode)
	}
}

func TestSchemaExploreRequestNormalizedFallsBackForUnknownDetailMode(t *testing.T) {
	got := (SchemaExploreRequest{DetailMode: SchemaDetailMode("verbose")}).Normalized()
	if got.DetailMode != SchemaDetailModeCompact {
		t.Fatalf("expected compact fallback, got %q", got.DetailMode)
	}
}
