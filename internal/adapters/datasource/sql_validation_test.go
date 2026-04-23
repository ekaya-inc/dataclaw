package datasource

import "testing"

func TestValidateReadOnlySQLPolicy(t *testing.T) {
	tests := []struct {
		name     string
		sqlQuery string
		wantSQL  string
		wantErr  string
	}{
		{
			name: "allows recursive cte with comments and string literals",
			sqlQuery: `WITH RECURSIVE nums(n) AS (
  SELECT 1
  UNION ALL
  SELECT n + 1 FROM nums WHERE n < 3
)
SELECT 'DELETE' AS note, n FROM nums -- UPDATE
;`,
			wantSQL: `WITH RECURSIVE nums(n) AS (
  SELECT 1
  UNION ALL
  SELECT n + 1 FROM nums WHERE n < 3
)
SELECT 'DELETE' AS note, n FROM nums -- UPDATE`,
		},
		{
			name:     "rejects select into",
			sqlQuery: "SELECT * INTO audit_users FROM users",
			wantErr:  "SELECT INTO is not allowed in read-only queries",
		},
		{
			name:     "rejects mutating cte",
			sqlQuery: "WITH changed AS (DELETE FROM users RETURNING id) SELECT id FROM changed",
			wantErr:  "only read-only SELECT or WITH statements are allowed",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ValidateReadOnlySQL(tc.sqlQuery)
			if tc.wantErr != "" {
				if err == nil {
					t.Fatal("ValidateReadOnlySQL() returned nil error")
				}
				if err.Error() != tc.wantErr {
					t.Fatalf("ValidateReadOnlySQL() error = %q, want %q", err.Error(), tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("ValidateReadOnlySQL() error = %v", err)
			}
			if got != tc.wantSQL {
				t.Fatalf("ValidateReadOnlySQL() = %q, want %q", got, tc.wantSQL)
			}
		})
	}
}

func TestValidateDMLSQLPolicy(t *testing.T) {
	tests := []struct {
		name     string
		sqlQuery string
		wantSQL  string
		wantErr  string
	}{
		{
			name:     "allows dml with ddl keywords inside strings and comments",
			sqlQuery: "UPDATE users SET note = 'DROP TABLE', updated_at = CURRENT_TIMESTAMP /* CREATE TABLE audit */ WHERE id = 1;",
			wantSQL:  "UPDATE users SET note = 'DROP TABLE', updated_at = CURRENT_TIMESTAMP /* CREATE TABLE audit */ WHERE id = 1",
		},
		{
			name:     "rejects ddl keyword after valid dml start",
			sqlQuery: "DELETE FROM users TRUNCATE TABLE audit_log",
			wantErr:  "DDL statements (DROP, CREATE, ALTER, TRUNCATE, GRANT, REVOKE, RENAME, VACUUM, ATTACH, DETACH, PRAGMA) are not allowed",
		},
		{
			name:     "rejects select statements",
			sqlQuery: "SELECT * FROM users",
			wantErr:  "DML queries must start with INSERT, UPDATE, or DELETE",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ValidateDMLSQL(tc.sqlQuery)
			if tc.wantErr != "" {
				if err == nil {
					t.Fatal("ValidateDMLSQL() returned nil error")
				}
				if err.Error() != tc.wantErr {
					t.Fatalf("ValidateDMLSQL() error = %q, want %q", err.Error(), tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("ValidateDMLSQL() error = %v", err)
			}
			if got != tc.wantSQL {
				t.Fatalf("ValidateDMLSQL() = %q, want %q", got, tc.wantSQL)
			}
		})
	}
}
