package relational_test

import (
	"reflect"
	"testing"

	"opskeeper/backend/tool/mysql"
	"opskeeper/backend/tool/oceanbase"
	"opskeeper/backend/tool/oracle"
	"opskeeper/backend/tool/postgresql"
)

func TestDatabaseDiagnosticsShareWireContracts(t *testing.T) {
	sets := [][]reflect.Type{
		{reflect.TypeOf(mysql.SlowQueriesInput{}), reflect.TypeOf(mysql.SlowQueriesOutput{}), reflect.TypeOf(mysql.ActiveQueriesOutput{}), reflect.TypeOf(mysql.LockWaitsOutput{}), reflect.TypeOf(mysql.ConnectionStatsOutput{}), reflect.TypeOf(mysql.TableStructureOutput{})},
		{reflect.TypeOf(oracle.SlowQueriesInput{}), reflect.TypeOf(oracle.SlowQueriesOutput{}), reflect.TypeOf(oracle.ActiveQueriesOutput{}), reflect.TypeOf(oracle.LockWaitsOutput{}), reflect.TypeOf(oracle.ConnectionStatsOutput{}), reflect.TypeOf(oracle.TableStructureOutput{})},
		{reflect.TypeOf(oceanbase.SlowQueriesInput{}), reflect.TypeOf(oceanbase.SlowQueriesOutput{}), reflect.TypeOf(oceanbase.ActiveQueriesOutput{}), reflect.TypeOf(oceanbase.LockWaitsOutput{}), reflect.TypeOf(oceanbase.ConnectionStatsOutput{}), reflect.TypeOf(oceanbase.TableStructureOutput{})},
		{reflect.TypeOf(postgresql.SlowQueriesInput{}), reflect.TypeOf(postgresql.SlowQueriesOutput{}), reflect.TypeOf(postgresql.ActiveQueriesOutput{}), reflect.TypeOf(postgresql.LockWaitsOutput{}), reflect.TypeOf(postgresql.ConnectionStatsOutput{}), reflect.TypeOf(postgresql.TableStructureOutput{})},
	}
	for i := 1; i < len(sets); i++ {
		for j := range sets[0] {
			if sets[i][j] != sets[0][j] {
				t.Errorf("engine %d contract %d = %v, want %v", i, j, sets[i][j], sets[0][j])
			}
		}
	}
}

func TestSessionAndWaitFieldsDoNotClaimPIDSemantics(t *testing.T) {
	activeField, ok := reflect.TypeOf(mysql.ActiveQueriesOutput{}).FieldByName("Queries")
	if !ok {
		t.Fatal("active queries field missing")
	}
	active := activeField.Type.Elem().Field(0)
	if active.Name != "SessionID" || active.Tag.Get("json") != "session_id" {
		t.Fatalf("active session identifier = %s %q", active.Name, active.Tag.Get("json"))
	}
	waitsField, ok := reflect.TypeOf(mysql.LockWaitsOutput{}).FieldByName("Waits")
	if !ok {
		t.Fatal("lock waits field missing")
	}
	wait := waitsField.Type.Elem().Field(0)
	if wait.Name != "BlockedSessionID" || wait.Tag.Get("json") != "blocked_session_id" {
		t.Fatalf("blocked session identifier = %s %q", wait.Name, wait.Tag.Get("json"))
	}
}
