// Copyright 2020 Dolthub, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package enginetest

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	gms "github.com/dolthub/go-mysql-server"
	"github.com/dolthub/go-mysql-server/enginetest"
	"github.com/dolthub/go-mysql-server/enginetest/queries"
	"github.com/dolthub/go-mysql-server/enginetest/scriptgen/setup"
	"github.com/dolthub/go-mysql-server/server"
	"github.com/dolthub/go-mysql-server/sql"
	"github.com/dolthub/go-mysql-server/sql/analyzer"
	"github.com/dolthub/go-mysql-server/sql/mysql_db"
	gmstypes "github.com/dolthub/go-mysql-server/sql/types"
	"github.com/dolthub/vitess/go/mysql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dolthub/dolt/go/libraries/doltcore/dtestutils"
	"github.com/dolthub/dolt/go/libraries/doltcore/env"
	"github.com/dolthub/dolt/go/libraries/doltcore/schema"
	"github.com/dolthub/dolt/go/libraries/doltcore/sqle"
	"github.com/dolthub/dolt/go/libraries/doltcore/sqle/dsess"
	"github.com/dolthub/dolt/go/libraries/utils/config"
	"github.com/dolthub/dolt/go/store/datas"
	"github.com/dolthub/dolt/go/store/types"
)

var skipPrepared bool

// SkipPreparedsCount is used by the "ci-check-repo CI workflow
// as a reminder to consider prepareds when adding a new
// enginetest suite.
const SkipPreparedsCount = 86

const skipPreparedFlag = "DOLT_SKIP_PREPARED_ENGINETESTS"

func init() {
	sqle.MinRowsPerPartition = 8
	sqle.MaxRowsPerPartition = 1024

	if v := os.Getenv(skipPreparedFlag); v != "" {
		skipPrepared = true
	}
}

func TestQueries(t *testing.T) {
	h := newDoltHarness(t)
	defer h.Close()
	enginetest.TestQueries(t, h)
}

func TestSingleQuery(t *testing.T) {
	t.Skip()

	harness := newDoltHarness(t)
	harness.Setup(setup.SimpleSetup...)
	engine, err := harness.NewEngine(t)
	if err != nil {
		panic(err)
	}

	setupQueries := []string{
		// "create table t1 (pk int primary key, c int);",
		// "insert into t1 values (1,2), (3,4)",
		// "call dolt_add('.')",
		// "set @Commit1 = dolt_commit('-am', 'initial table');",
		// "insert into t1 values (5,6), (7,8)",
		// "set @Commit2 = dolt_commit('-am', 'two more rows');",
	}

	for _, q := range setupQueries {
		enginetest.RunQuery(t, engine, harness, q)
	}

	engine.Analyzer.Debug = true
	engine.Analyzer.Verbose = true

	var test queries.QueryTest
	test = queries.QueryTest{
		Query: `show create table mytable`,
		Expected: []sql.Row{
			{"mytable",
				"CREATE TABLE `mytable` (\n" +
					"  `i` bigint NOT NULL,\n" +
					"  `s` varchar(20) NOT NULL COMMENT 'column s',\n" +
					"  PRIMARY KEY (`i`),\n" +
					"  KEY `idx_si` (`s`,`i`),\n" +
					"  KEY `mytable_i_s` (`i`,`s`),\n" +
					"  UNIQUE KEY `mytable_s` (`s`)\n" +
					") ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_bin"},
		},
	}

	enginetest.TestQueryWithEngine(t, harness, engine, test)
}

// Convenience test for debugging a single query. Unskip and set to the desired query.
func TestSingleScript(t *testing.T) {
	t.Skip()
	var scripts = []queries.ScriptTest{
		{
			Name: "parallel column updates (repro issue #4547)",
			SetUpScript: []string{
				"SET dolt_allow_commit_conflicts = on;",
				"create table t (rowId int not null, col1 varchar(255), col2 varchar(255), keyCol varchar(60), dataA varchar(255), dataB varchar(255), PRIMARY KEY (rowId), UNIQUE KEY uniqKey (col1, col2, keyCol));",
				"insert into t (rowId, col1, col2, keyCol, dataA, dataB) values (1, '1', '2', 'key-a', 'test1', 'test2')",
				"CALL DOLT_COMMIT('-Am', 'new table');",

				"CALL DOLT_CHECKOUT('-b', 'other');",
				"update t set dataA = 'other'",
				"CALL DOLT_COMMIT('-am', 'update data other');",

				"CALL DOLT_CHECKOUT('main');",
				"update t set dataB = 'main'",
				"CALL DOLT_COMMIT('-am', 'update on main');",
			},
			Assertions: []queries.ScriptTestAssertion{
				{
					Query:    "CALL DOLT_MERGE('other')",
					Expected: []sql.Row{{"child", uint64(1)}},
				},
				{
					Query:    "SELECT * from dolt_constraint_violations_t",
					Expected: []sql.Row{},
				},
				{
					Query: "SELECT * from t",
					Expected: []sql.Row{
						{1, "1", "2", "key-a", "other", "main"},
					},
				},
			},
		},
	}

	harness := newDoltHarness(t)
	for _, test := range scripts {
		enginetest.TestScript(t, harness, test)
	}
}

// Convenience test for debugging a single query. Unskip and set to the desired query.
func TestSingleMergeScript(t *testing.T) {
	t.Skip()
	var scripts = []MergeScriptTest{
		{
			Name: "adding a non-null column with a default value to one side",
			AncSetUpScript: []string{
				"set dolt_force_transaction_commit = on;",
				"create table t (pk int primary key, col1 int);",
				"insert into t values (1, 1);",
			},
			RightSetUpScript: []string{
				"alter table t add column col2 int not null default 0",
				"alter table t add column col3 int;",
				"insert into t values (2, 2, 2, null);",
			},
			LeftSetUpScript: []string{
				"insert into t values (3, 3);",
			},
			Assertions: []queries.ScriptTestAssertion{
				{
					Query:    "call dolt_merge('right');",
					Expected: []sql.Row{{0, 0}},
				},
				{
					Query:    "select * from t;",
					Expected: []sql.Row{{1, 1, 0, nil}, {2, 2, 2, nil}, {3, 3, 0, nil}},
				},
				{
					Query:    "select pk, violation_type from dolt_constraint_violations_t",
					Expected: []sql.Row{},
				},
			},
		},
	}
	for _, test := range scripts {
		t.Run("merge right into left", func(t *testing.T) {
			enginetest.TestScript(t, newDoltHarness(t), convertMergeScriptTest(test, false))
		})
		t.Run("merge left into right", func(t *testing.T) {
			enginetest.TestScript(t, newDoltHarness(t), convertMergeScriptTest(test, true))
		})
	}
}

func TestSingleQueryPrepared(t *testing.T) {
	t.Skip()

	harness := newDoltHarness(t)
	//engine := enginetest.NewEngine(t, harness)
	//enginetest.CreateIndexes(t, harness, engine)
	//engine := enginetest.NewSpatialEngine(t, harness)
	engine, err := harness.NewEngine(t)
	if err != nil {
		panic(err)
	}

	setupQueries := []string{
		"create table t1 (pk int primary key, c int);",
		"call dolt_add('.')",
		"insert into t1 values (1,2), (3,4)",
		"set @Commit1 = dolt_commit('-am', 'initial table');",
		"insert into t1 values (5,6), (7,8)",
		"set @Commit2 = dolt_commit('-am', 'two more rows');",
	}

	for _, q := range setupQueries {
		enginetest.RunQuery(t, engine, harness, q)
	}

	//engine.Analyzer.Debug = true
	//engine.Analyzer.Verbose = true

	var test queries.QueryTest
	test = queries.QueryTest{
		Query: "explain select pk, c from dolt_history_t1 where pk = 3 and committer = 'someguy'",
		Expected: []sql.Row{
			{"Exchange"},
			{" └─ Project(dolt_history_t1.pk, dolt_history_t1.c)"},
			{"     └─ Filter((dolt_history_t1.pk = 3) AND (dolt_history_t1.committer = 'someguy'))"},
			{"         └─ IndexedTableAccess(dolt_history_t1)"},
			{"             ├─ index: [dolt_history_t1.pk]"},
			{"             ├─ filters: [{[3, 3]}]"},
			{"             └─ columns: [pk c committer]"},
		},
	}

	enginetest.TestPreparedQuery(t, harness, test.Query, test.Expected, nil)
}

func TestSingleScriptPrepared(t *testing.T) {
	t.Skip()
	var script = queries.ScriptTest{
		Name: "table with commit column should maintain its data in diff",
		SetUpScript: []string{
			"CREATE TABLE t (pk int PRIMARY KEY, commit varchar(20));",
			"CALL DOLT_ADD('.');",
			"CALL dolt_commit('-am', 'creating table t');",
			"INSERT INTO t VALUES (1, '123456');",
			"CALL dolt_commit('-am', 'insert data');",
		},
		Assertions: []queries.ScriptTestAssertion{
			{
				Query:    "SELECT to_pk, char_length(to_commit), from_pk, char_length(from_commit), diff_type from dolt_diff_t;",
				Expected: []sql.Row{{1, 32, nil, 32, "added"}},
			},
		},
	}
	harness := newDoltHarness(t)
	enginetest.TestScriptPrepared(t, harness, script)
}

func TestVersionedQueries(t *testing.T) {
	h := newDoltHarness(t)
	defer h.Close()
	enginetest.TestVersionedQueries(t, h)
}

// Tests of choosing the correct execution plan independent of result correctness. Mostly useful for confirming that
// the right indexes are being used for joining tables.
func TestQueryPlans(t *testing.T) {
	// Dolt supports partial keys, so the index matched is different for some plans
	// TODO: Fix these differences by implementing partial key matching in the memory tables, or the engine itself
	skipped := []string{
		"SELECT pk,pk1,pk2 FROM one_pk LEFT JOIN two_pk ON pk=pk1",
		"SELECT pk,pk1,pk2 FROM one_pk JOIN two_pk ON pk=pk1",
		"SELECT one_pk.c5,pk1,pk2 FROM one_pk JOIN two_pk ON pk=pk1 ORDER BY 1,2,3",
		"SELECT opk.c5,pk1,pk2 FROM one_pk opk JOIN two_pk tpk ON opk.pk=tpk.pk1 ORDER BY 1,2,3",
		"SELECT opk.c5,pk1,pk2 FROM one_pk opk JOIN two_pk tpk ON pk=pk1 ORDER BY 1,2,3",
		"SELECT pk,pk1,pk2 FROM one_pk LEFT JOIN two_pk ON pk=pk1 ORDER BY 1,2,3",
		"SELECT pk,pk1,pk2 FROM one_pk t1, two_pk t2 WHERE pk=1 AND pk2=1 AND pk1=1 ORDER BY 1,2",
	}
	// Parallelism introduces Exchange nodes into the query plans, so disable.
	// TODO: exchange nodes should really only be part of the explain plan under certain debug settings
	harness := newDoltHarness(t).WithParallelism(1).WithSkippedQueries(skipped)
	if !types.IsFormat_DOLT(types.Format_Default) {
		// only new format supports reverse IndexTableAccess
		reverseIndexSkip := []string{
			"SELECT * FROM one_pk ORDER BY pk",
			"SELECT * FROM two_pk ORDER BY pk1, pk2",
			"SELECT * FROM two_pk ORDER BY pk1",
			"SELECT pk1 AS one, pk2 AS two FROM two_pk ORDER BY pk1, pk2",
			"SELECT pk1 AS one, pk2 AS two FROM two_pk ORDER BY one, two",
			"SELECT i FROM (SELECT i FROM mytable ORDER BY i DESC LIMIT 1) sq WHERE i = 3",
			"SELECT i FROM (SELECT i FROM (SELECT i FROM mytable ORDER BY DES LIMIT 1) sql1)sql2 WHERE i = 3",
			"SELECT s,i FROM mytable order by i DESC",
			"SELECT s,i FROM mytable as a order by i DESC",
			"SELECT pk1, pk2 FROM two_pk order by pk1 asc, pk2 asc",
			"SELECT pk1, pk2 FROM two_pk order by pk1 desc, pk2 desc",
			"SELECT i FROM (SELECT i FROM (SELECT i FROM mytable ORDER BY i DESC  LIMIT 1) sq1) sq2 WHERE i = 3",
		}
		harness = harness.WithSkippedQueries(reverseIndexSkip)
	}

	defer harness.Close()
	enginetest.TestQueryPlans(t, harness, queries.PlanTests)
}

func TestIntegrationQueryPlans(t *testing.T) {
	harness := newDoltHarness(t).WithParallelism(1)

	defer harness.Close()
	enginetest.TestIntegrationPlans(t, harness)
}

func TestDoltDiffQueryPlans(t *testing.T) {
	if !types.IsFormat_DOLT(types.Format_Default) {
		t.Skip("only new format support system table indexing")
	}

	harness := newDoltHarness(t).WithParallelism(2) // want Exchange nodes
	defer harness.Close()
	harness.Setup(setup.SimpleSetup...)
	e, err := harness.NewEngine(t)
	require.NoError(t, err)
	defer e.Close()

	for _, tt := range DoltDiffPlanTests {
		enginetest.TestQueryPlan(t, harness, e, tt.Query, tt.ExpectedPlan, false)
	}
}

func TestQueryErrors(t *testing.T) {
	h := newDoltHarness(t)
	defer h.Close()
	enginetest.TestQueryErrors(t, h)
}

func TestInfoSchema(t *testing.T) {
	h := newDoltHarness(t)
	defer h.Close()
	enginetest.TestInfoSchema(t, h)
}

func TestColumnAliases(t *testing.T) {
	h := newDoltHarness(t)
	defer h.Close()
	enginetest.TestColumnAliases(t, h)
}

func TestOrderByGroupBy(t *testing.T) {
	h := newDoltHarness(t)
	defer h.Close()
	enginetest.TestOrderByGroupBy(t, h)
}

func TestAmbiguousColumnResolution(t *testing.T) {
	h := newDoltHarness(t)
	defer h.Close()
	enginetest.TestAmbiguousColumnResolution(t, h)
}

func TestInsertInto(t *testing.T) {
	h := newDoltHarness(t)
	defer h.Close()
	enginetest.TestInsertInto(t, h)
}

func TestInsertIgnoreInto(t *testing.T) {
	h := newDoltHarness(t)
	defer h.Close()
	enginetest.TestInsertIgnoreInto(t, h)
}

// TODO: merge this into the above test when we remove old format
func TestInsertDuplicateKeyKeyless(t *testing.T) {
	if !types.IsFormat_DOLT(types.Format_Default) {
		t.Skip()
	}
	enginetest.TestInsertDuplicateKeyKeyless(t, newDoltHarness(t))
}

// TODO: merge this into the above test when we remove old format
func TestInsertDuplicateKeyKeylessPrepared(t *testing.T) {
	if !types.IsFormat_DOLT(types.Format_Default) {
		t.Skip()
	}
	enginetest.TestInsertDuplicateKeyKeylessPrepared(t, newDoltHarness(t))
}

// TODO: merge this into the above test when we remove old format
func TestIgnoreIntoWithDuplicateUniqueKeyKeyless(t *testing.T) {
	if !types.IsFormat_DOLT(types.Format_Default) {
		t.Skip()
	}
	h := newDoltHarness(t)
	defer h.Close()
	enginetest.TestIgnoreIntoWithDuplicateUniqueKeyKeyless(t, h)
}

// TODO: merge this into the above test when we remove old format
func TestIgnoreIntoWithDuplicateUniqueKeyKeylessPrepared(t *testing.T) {
	if !types.IsFormat_DOLT(types.Format_Default) {
		t.Skip()
	}
	enginetest.TestIgnoreIntoWithDuplicateUniqueKeyKeylessPrepared(t, newDoltHarness(t))
}

func TestInsertIntoErrors(t *testing.T) {
	h := newDoltHarness(t)
	defer h.Close()
	enginetest.TestInsertIntoErrors(t, h)
}

func TestSpatialQueries(t *testing.T) {
	h := newDoltHarness(t)
	defer h.Close()
	enginetest.TestSpatialQueries(t, h)
}

func TestReplaceInto(t *testing.T) {
	h := newDoltHarness(t)
	defer h.Close()
	enginetest.TestReplaceInto(t, h)
}

func TestReplaceIntoErrors(t *testing.T) {
	h := newDoltHarness(t)
	defer h.Close()
	enginetest.TestReplaceIntoErrors(t, h)
}

func TestUpdate(t *testing.T) {
	h := newDoltHarness(t)
	defer h.Close()
	enginetest.TestUpdate(t, h)
}

func TestUpdateIgnore(t *testing.T) {
	h := newDoltHarness(t)
	defer h.Close()
	enginetest.TestUpdateIgnore(t, h)
}

func TestUpdateErrors(t *testing.T) {
	h := newDoltHarness(t)
	defer h.Close()
	enginetest.TestUpdateErrors(t, h)
}

func TestDeleteFrom(t *testing.T) {
	h := newDoltHarness(t)
	defer h.Close()
	enginetest.TestDelete(t, h)
}

func TestDeleteFromErrors(t *testing.T) {
	h := newDoltHarness(t)
	defer h.Close()
	enginetest.TestDeleteErrors(t, h)
}

func TestSpatialDelete(t *testing.T) {
	h := newDoltHarness(t)
	defer h.Close()
	enginetest.TestSpatialDelete(t, h)
}

func TestSpatialScripts(t *testing.T) {
	h := newDoltHarness(t)
	defer h.Close()
	enginetest.TestSpatialScripts(t, h)
}

func TestSpatialScriptsPrepared(t *testing.T) {
	enginetest.TestSpatialScriptsPrepared(t, newDoltHarness(t))
}

func TestSpatialIndexScripts(t *testing.T) {
	skipOldFormat(t)
	enginetest.TestSpatialIndexScripts(t, newDoltHarness(t))
}

func TestSpatialIndexScriptsPrepared(t *testing.T) {
	skipOldFormat(t)
	enginetest.TestSpatialIndexScriptsPrepared(t, newDoltHarness(t))
}

func TestSpatialIndexPlans(t *testing.T) {
	skipOldFormat(t)
	enginetest.TestSpatialIndexPlans(t, newDoltHarness(t))
}

func TestSpatialIndexPlansPrepared(t *testing.T) {
	skipOldFormat(t)
	enginetest.TestSpatialIndexPlansPrepared(t, newDoltHarness(t))
}

func TestTruncate(t *testing.T) {
	h := newDoltHarness(t)
	defer h.Close()
	enginetest.TestTruncate(t, h)
}

func TestConvert(t *testing.T) {
	if types.IsFormat_LD(types.Format_Default) {
		t.Skip("noms format has outdated type enforcement")
	}
	h := newDoltHarness(t)
	defer h.Close()
	enginetest.TestConvertPrepared(t, h)
}

func TestConvertPrepared(t *testing.T) {
	if types.IsFormat_LD(types.Format_Default) {
		t.Skip("noms format has outdated type enforcement")
	}
	h := newDoltHarness(t)
	defer h.Close()
	enginetest.TestConvertPrepared(t, h)
}

func TestScripts(t *testing.T) {
	var skipped []string
	if types.IsFormat_DOLT(types.Format_Default) {
		skipped = append(skipped, newFormatSkippedScripts...)
	}
	h := newDoltHarness(t).WithSkippedQueries(skipped).WithParallelism(1)
	defer h.Close()
	enginetest.TestScripts(t, h)
}

// TestDoltUserPrivileges tests Dolt-specific code that needs to handle user privilege checking
func TestDoltUserPrivileges(t *testing.T) {
	harness := newDoltHarness(t)
	defer harness.Close()
	for _, script := range DoltUserPrivTests {
		t.Run(script.Name, func(t *testing.T) {
			harness.Setup(setup.MydbData)
			engine, err := harness.NewEngine(t)
			require.NoError(t, err)
			defer engine.Close()

			ctx := enginetest.NewContextWithClient(harness, sql.Client{
				User:    "root",
				Address: "localhost",
			})

			engine.Analyzer.Catalog.MySQLDb.AddRootAccount()
			engine.Analyzer.Catalog.MySQLDb.SetPersister(&mysql_db.NoopPersister{})

			for _, statement := range script.SetUpScript {
				if sh, ok := interface{}(harness).(enginetest.SkippingHarness); ok {
					if sh.SkipQueryTest(statement) {
						t.Skip()
					}
				}
				enginetest.RunQueryWithContext(t, engine, harness, ctx, statement)
			}
			for _, assertion := range script.Assertions {
				if sh, ok := interface{}(harness).(enginetest.SkippingHarness); ok {
					if sh.SkipQueryTest(assertion.Query) {
						t.Skipf("Skipping query %s", assertion.Query)
					}
				}

				user := assertion.User
				host := assertion.Host
				if user == "" {
					user = "root"
				}
				if host == "" {
					host = "localhost"
				}
				ctx := enginetest.NewContextWithClient(harness, sql.Client{
					User:    user,
					Address: host,
				})

				if assertion.ExpectedErr != nil {
					t.Run(assertion.Query, func(t *testing.T) {
						enginetest.AssertErrWithCtx(t, engine, harness, ctx, assertion.Query, assertion.ExpectedErr)
					})
				} else if assertion.ExpectedErrStr != "" {
					t.Run(assertion.Query, func(t *testing.T) {
						enginetest.AssertErrWithCtx(t, engine, harness, ctx, assertion.Query, nil, assertion.ExpectedErrStr)
					})
				} else {
					t.Run(assertion.Query, func(t *testing.T) {
						enginetest.TestQueryWithContext(t, ctx, engine, harness, assertion.Query, assertion.Expected, nil, nil)
					})
				}
			}
		})
	}
}

func TestJoinOps(t *testing.T) {
	if types.IsFormat_LD(types.Format_Default) {
		t.Skip("DOLT_LD keyless indexes are not sorted")
	}

	h := newDoltHarness(t)
	defer h.Close()
	enginetest.TestJoinOps(t, h)
}

func TestJoinPlanningPrepared(t *testing.T) {
	if types.IsFormat_LD(types.Format_Default) {
		t.Skip("DOLT_LD keyless indexes are not sorted")
	}
	h := newDoltHarness(t).WithParallelism(1)
	defer h.Close()
	enginetest.TestJoinPlanningPrepared(t, h)
}

func TestJoinPlanning(t *testing.T) {
	if types.IsFormat_LD(types.Format_Default) {
		t.Skip("DOLT_LD keyless indexes are not sorted")
	}
	h := newDoltHarness(t).WithParallelism(1)
	defer h.Close()
	enginetest.TestJoinPlanning(t, h)
}

func TestJoinOpsPrepared(t *testing.T) {
	if types.IsFormat_LD(types.Format_Default) {
		t.Skip("DOLT_LD keyless indexes are not sorted")
	}

	h := newDoltHarness(t)
	defer h.Close()
	enginetest.TestJoinOpsPrepared(t, h)
}

func TestJoinQueries(t *testing.T) {
	h := newDoltHarness(t)
	defer h.Close()
	enginetest.TestJoinQueries(t, h)
}

func TestJoinQueriesPrepared(t *testing.T) {
	h := newDoltHarness(t)
	defer h.Close()
	enginetest.TestJoinQueriesPrepared(t, h)
}

// TestJSONTableQueries runs the canonical test queries against a single threaded index enabled harness.
func TestJSONTableQueries(t *testing.T) {
	h := newDoltHarness(t)
	defer h.Close()
	enginetest.TestJSONTableQueries(t, h)
}

// TestJSONTableScripts runs the canonical test queries against a single threaded index enabled harness.
func TestJSONTableScripts(t *testing.T) {
	h := newDoltHarness(t)
	defer h.Close()
	enginetest.TestJSONTableScripts(t, h)
}

func TestUserPrivileges(t *testing.T) {
	h := newDoltHarness(t)
	defer h.Close()
	enginetest.TestUserPrivileges(t, h)
}

func TestUserAuthentication(t *testing.T) {
	t.Skip("Unexpected panic, need to fix")
	h := newDoltHarness(t)
	defer h.Close()
	enginetest.TestUserAuthentication(t, h)
}

func TestComplexIndexQueries(t *testing.T) {
	h := newDoltHarness(t)
	defer h.Close()
	enginetest.TestComplexIndexQueries(t, h)
}

func TestCreateTable(t *testing.T) {
	h := newDoltHarness(t)
	defer h.Close()
	enginetest.TestCreateTable(t, h)
}

func TestPkOrdinalsDDL(t *testing.T) {
	h := newDoltHarness(t)
	defer h.Close()
	enginetest.TestPkOrdinalsDDL(t, h)
}

func TestPkOrdinalsDML(t *testing.T) {
	h := newDoltHarness(t)
	defer h.Close()
	enginetest.TestPkOrdinalsDML(t, h)
}

func TestDropTable(t *testing.T) {
	h := newDoltHarness(t)
	defer h.Close()
	enginetest.TestDropTable(t, h)
}

func TestRenameTable(t *testing.T) {
	h := newDoltHarness(t)
	defer h.Close()
	enginetest.TestRenameTable(t, h)
}

func TestRenameColumn(t *testing.T) {
	h := newDoltHarness(t)
	defer h.Close()
	enginetest.TestRenameColumn(t, h)
}

func TestAddColumn(t *testing.T) {
	h := newDoltHarness(t)
	defer h.Close()
	enginetest.TestAddColumn(t, h)
}

func TestModifyColumn(t *testing.T) {
	h := newDoltHarness(t)
	defer h.Close()
	enginetest.TestModifyColumn(t, h)
}

func TestDropColumn(t *testing.T) {
	h := newDoltHarness(t)
	defer h.Close()
	enginetest.TestDropColumn(t, h)
}

func TestCreateDatabase(t *testing.T) {
	h := newDoltHarness(t)
	defer h.Close()
	enginetest.TestCreateDatabase(t, h)
}

func TestBlobs(t *testing.T) {
	skipOldFormat(t)
	h := newDoltHarness(t)
	defer h.Close()
	enginetest.TestBlobs(t, h)
}

func TestIndexes(t *testing.T) {
	harness := newDoltHarness(t)
	defer harness.Close()
	enginetest.TestIndexes(t, harness)
}

func TestIndexPrefix(t *testing.T) {
	skipOldFormat(t)
	harness := newDoltHarness(t)
	defer harness.Close()
	enginetest.TestIndexPrefix(t, harness)
	for _, script := range DoltIndexPrefixScripts {
		enginetest.TestScript(t, harness, script)
	}
}

func TestBigBlobs(t *testing.T) {
	skipOldFormat(t)

	h := newDoltHarness(t)
	defer h.Close()
	h.Setup(setup.MydbData, setup.BlobData)
	for _, tt := range BigBlobQueries {
		enginetest.RunWriteQueryTest(t, h, tt)
	}
}

func TestDropDatabase(t *testing.T) {
	func() {
		h := newDoltHarness(t)
		defer h.Close()
		enginetest.TestScript(t, h, queries.ScriptTest{
			Name: "Drop database engine tests for Dolt only",
			SetUpScript: []string{
				"CREATE DATABASE Test1db",
				"CREATE DATABASE TEST2db",
			},
			Assertions: []queries.ScriptTestAssertion{
				{
					Query:    "DROP DATABASE TeSt2DB",
					Expected: []sql.Row{{gmstypes.OkResult{RowsAffected: 1}}},
				},
				{
					Query:       "USE test2db",
					ExpectedErr: sql.ErrDatabaseNotFound,
				},
				{
					Query:    "USE TEST1DB",
					Expected: []sql.Row{},
				},
				{
					Query:    "DROP DATABASE IF EXISTS test1DB",
					Expected: []sql.Row{{gmstypes.OkResult{RowsAffected: 1}}},
				},
				{
					Query:       "USE Test1db",
					ExpectedErr: sql.ErrDatabaseNotFound,
				},
			},
		})
	}()

	t.Skip("Dolt doesn't yet support dropping the primary database, which these tests do")
	h := newDoltHarness(t)
	defer h.Close()
	enginetest.TestDropDatabase(t, h)
}

func TestCreateForeignKeys(t *testing.T) {
	h := newDoltHarness(t)
	defer h.Close()
	enginetest.TestCreateForeignKeys(t, h)
}

func TestDropForeignKeys(t *testing.T) {
	h := newDoltHarness(t)
	defer h.Close()
	enginetest.TestDropForeignKeys(t, h)
}

func TestForeignKeys(t *testing.T) {
	h := newDoltHarness(t)
	defer h.Close()
	enginetest.TestForeignKeys(t, h)
}

func TestCreateCheckConstraints(t *testing.T) {
	h := newDoltHarness(t)
	defer h.Close()
	enginetest.TestCreateCheckConstraints(t, h)
}

func TestChecksOnInsert(t *testing.T) {
	h := newDoltHarness(t)
	defer h.Close()
	enginetest.TestChecksOnInsert(t, h)
}

func TestChecksOnUpdate(t *testing.T) {
	h := newDoltHarness(t)
	defer h.Close()
	enginetest.TestChecksOnUpdate(t, h)
}

func TestDisallowedCheckConstraints(t *testing.T) {
	h := newDoltHarness(t)
	defer h.Close()
	enginetest.TestDisallowedCheckConstraints(t, h)
}

func TestDropCheckConstraints(t *testing.T) {
	h := newDoltHarness(t)
	defer h.Close()
	enginetest.TestDropCheckConstraints(t, h)
}

func TestReadOnly(t *testing.T) {
	h := newDoltHarness(t)
	defer h.Close()
	enginetest.TestReadOnly(t, h)
}

func TestViews(t *testing.T) {
	h := newDoltHarness(t)
	defer h.Close()
	enginetest.TestViews(t, h)
}

func TestVersionedViews(t *testing.T) {
	h := newDoltHarness(t)
	defer h.Close()
	enginetest.TestVersionedViews(t, h)
}

func TestWindowFunctions(t *testing.T) {
	h := newDoltHarness(t)
	defer h.Close()
	enginetest.TestWindowFunctions(t, h)
}

func TestWindowRowFrames(t *testing.T) {
	h := newDoltHarness(t)
	defer h.Close()
	enginetest.TestWindowRowFrames(t, h)
}

func TestWindowRangeFrames(t *testing.T) {
	h := newDoltHarness(t)
	defer h.Close()
	enginetest.TestWindowRangeFrames(t, h)
}

func TestNamedWindows(t *testing.T) {
	h := newDoltHarness(t)
	defer h.Close()
	enginetest.TestNamedWindows(t, h)
}

func TestNaturalJoin(t *testing.T) {
	h := newDoltHarness(t)
	defer h.Close()
	enginetest.TestNaturalJoin(t, h)
}

func TestNaturalJoinEqual(t *testing.T) {
	h := newDoltHarness(t)
	defer h.Close()
	enginetest.TestNaturalJoinEqual(t, h)
}

func TestNaturalJoinDisjoint(t *testing.T) {
	h := newDoltHarness(t)
	defer h.Close()
	enginetest.TestNaturalJoinEqual(t, h)
}

func TestInnerNestedInNaturalJoins(t *testing.T) {
	h := newDoltHarness(t)
	defer h.Close()
	enginetest.TestInnerNestedInNaturalJoins(t, h)
}

func TestColumnDefaults(t *testing.T) {
	h := newDoltHarness(t)
	defer h.Close()
	enginetest.TestColumnDefaults(t, h)
}

func TestAlterTable(t *testing.T) {
	h := newDoltHarness(t)
	defer h.Close()
	enginetest.TestAlterTable(t, h)
}

func TestVariables(t *testing.T) {
	h := newDoltHarness(t)
	defer h.Close()
	enginetest.TestVariables(t, h)
}

func TestVariableErrors(t *testing.T) {
	h := newDoltHarness(t)
	defer h.Close()
	enginetest.TestVariableErrors(t, h)
}

func TestLoadDataPrepared(t *testing.T) {
	t.Skip("feature not supported")
	skipPreparedTests(t)
	h := newDoltHarness(t)
	defer h.Close()
	enginetest.TestLoadDataPrepared(t, h)
}

func TestLoadData(t *testing.T) {
	t.Skip()
	h := newDoltHarness(t)
	defer h.Close()
	enginetest.TestLoadData(t, h)
}

func TestLoadDataErrors(t *testing.T) {
	h := newDoltHarness(t)
	defer h.Close()
	enginetest.TestLoadDataErrors(t, h)
}

func TestJsonScripts(t *testing.T) {
	h := newDoltHarness(t)
	defer h.Close()
	enginetest.TestJsonScripts(t, h)
}

func TestTriggers(t *testing.T) {
	h := newDoltHarness(t)
	defer h.Close()
	enginetest.TestTriggers(t, h)
}

func TestRollbackTriggers(t *testing.T) {
	h := newDoltHarness(t)
	defer h.Close()
	enginetest.TestRollbackTriggers(t, h)
}

func TestStoredProcedures(t *testing.T) {
	tests := make([]queries.ScriptTest, 0, len(queries.ProcedureLogicTests))
	for _, test := range queries.ProcedureLogicTests {
		//TODO: this passes locally but SOMETIMES fails tests on GitHub, no clue why
		if test.Name != "ITERATE and LEAVE loops" {
			tests = append(tests, test)
		}
	}
	queries.ProcedureLogicTests = tests

	h := newDoltHarness(t)
	defer h.Close()
	enginetest.TestStoredProcedures(t, h)
}

func TestEvents(t *testing.T) {
	doltHarness := newDoltHarness(t)
	defer doltHarness.Close()
	enginetest.TestEvents(t, doltHarness)
}

func TestCallAsOf(t *testing.T) {
	for _, script := range DoltCallAsOf {
		func() {
			h := newDoltHarness(t)
			defer h.Close()
			enginetest.TestScript(t, h, script)
		}()
	}
}

func TestLargeJsonObjects(t *testing.T) {
	SkipByDefaultInCI(t)
	harness := newDoltHarness(t)
	defer harness.Close()
	for _, script := range LargeJsonObjectScriptTests {
		enginetest.TestScript(t, harness, script)
	}
}

func SkipByDefaultInCI(t *testing.T) {
	if os.Getenv("CI") != "" && os.Getenv("DOLT_TEST_RUN_NON_RACE_TESTS") == "" {
		t.Skip()
	}
}

func TestTransactions(t *testing.T) {
	for _, script := range queries.TransactionTests {
		func() {
			h := newDoltHarness(t)
			defer h.Close()
			enginetest.TestTransactionScript(t, h, script)
		}()
	}
	for _, script := range DoltTransactionTests {
		func() {
			h := newDoltHarness(t)
			defer h.Close()
			enginetest.TestTransactionScript(t, h, script)
		}()
	}
	for _, script := range DoltStoredProcedureTransactionTests {
		func() {
			h := newDoltHarness(t)
			defer h.Close()
			enginetest.TestTransactionScript(t, h, script)
		}()
	}
	for _, script := range DoltConflictHandlingTests {
		func() {
			h := newDoltHarness(t)
			defer h.Close()
			enginetest.TestTransactionScript(t, h, script)
		}()
	}
	for _, script := range DoltConstraintViolationTransactionTests {
		func() {
			h := newDoltHarness(t)
			defer h.Close()
			enginetest.TestTransactionScript(t, h, script)
		}()
	}
}

func TestBranchTransactions(t *testing.T) {
	for _, script := range BranchIsolationTests {
		func() {
			h := newDoltHarness(t)
			defer h.Close()
			enginetest.TestTransactionScript(t, h, script)
		}()
	}
}

func TestMultiDbTransactions(t *testing.T) {
	t.Skip()
	for _, script := range MultiDbTransactionTests {
		func() {
			h := newDoltHarness(t)
			defer h.Close()
			enginetest.TestScript(t, h, script)
		}()
	}
}

func TestConcurrentTransactions(t *testing.T) {
	h := newDoltHarness(t)
	defer h.Close()
	enginetest.TestConcurrentTransactions(t, h)
}

func TestDoltScripts(t *testing.T) {
	harness := newDoltHarness(t)
	defer harness.Close()
	for _, script := range DoltScripts {
		enginetest.TestScript(t, harness, script)
	}
}

func TestDoltRevisionDbScripts(t *testing.T) {
	for _, script := range DoltRevisionDbScripts {
		func() {
			h := newDoltHarness(t)
			defer h.Close()
			enginetest.TestScript(t, h, script)
		}()
	}

	// Testing a commit-qualified database revision spec requires
	// a little extra work to get the generated commit hash
	harness := newDoltHarness(t)
	defer harness.Close()
	e, err := harness.NewEngine(t)
	require.NoError(t, err)
	defer e.Close()
	ctx := harness.NewContext()

	setupScripts := []setup.SetupScript{
		{"create table t01 (pk int primary key, c1 int)"},
		{"call dolt_add('.');"},
		{"call dolt_commit('-am', 'creating table t01 on main');"},
		{"insert into t01 values (1, 1), (2, 2);"},
		{"call dolt_commit('-am', 'adding rows to table t01 on main');"},
		{"insert into t01 values (3, 3);"},
		{"call dolt_commit('-am', 'adding another row to table t01 on main');"},
	}
	_, err = enginetest.RunSetupScripts(ctx, harness.engine, setupScripts, true)
	require.NoError(t, err)

	sch, iter, err := harness.engine.Query(ctx, "select hashof('HEAD~2');")
	require.NoError(t, err)
	rows, err := sql.RowIterToRows(ctx, sch, iter)
	require.NoError(t, err)
	assert.Equal(t, 1, len(rows))
	commithash := rows[0][0].(string)

	scriptTest := queries.ScriptTest{
		Name: "database revision specs: commit-qualified revision spec",
		Assertions: []queries.ScriptTestAssertion{
			{
				Query:    "show databases;",
				Expected: []sql.Row{{"mydb"}, {"information_schema"}, {"mysql"}},
			},
			{
				Query:    "use mydb/" + commithash,
				Expected: []sql.Row{},
			},
			{
				Query: "select active_branch();",
				Expected: []sql.Row{
					{nil},
				},
			},
			{
				Query:    "select database();",
				Expected: []sql.Row{{"mydb/" + commithash}},
			},
			{
				Query:    "show databases;",
				Expected: []sql.Row{{"mydb"}, {"information_schema"}, {"mydb/" + commithash}, {"mysql"}},
			},
			{
				Query:    "select * from t01",
				Expected: []sql.Row{},
			},
			{
				Query:          "call dolt_reset();",
				ExpectedErrStr: "unable to reset HEAD in read-only databases",
			},
			{
				Query:    "call dolt_checkout('main');",
				Expected: []sql.Row{{0}},
			},
			{
				Query:    "select database();",
				Expected: []sql.Row{{"mydb"}},
			},
			{
				Query:    "select active_branch();",
				Expected: []sql.Row{{"main"}},
			},
			{
				Query:    "use mydb;",
				Expected: []sql.Row{},
			},
			{
				Query:    "select database();",
				Expected: []sql.Row{{"mydb"}},
			},
			{
				Query:    "show databases;",
				Expected: []sql.Row{{"mydb"}, {"information_schema"}, {"mysql"}},
			},
		},
	}

	enginetest.TestScript(t, harness, scriptTest)
}

func TestDoltRevisionDbScriptsPrepared(t *testing.T) {
	for _, script := range DoltRevisionDbScripts {
		func() {
			h := newDoltHarness(t)
			defer h.Close()
			enginetest.TestScriptPrepared(t, h, script)
		}()
	}
}

func TestDoltDdlScripts(t *testing.T) {
	harness := newDoltHarness(t)
	defer harness.Close()
	harness.Setup()

	for _, script := range ModifyAndChangeColumnScripts {
		e, err := harness.NewEngine(t)
		require.NoError(t, err)
		enginetest.TestScriptWithEngine(t, e, harness, script)
	}

	for _, script := range ModifyColumnTypeScripts {
		e, err := harness.NewEngine(t)
		require.NoError(t, err)
		enginetest.TestScriptWithEngine(t, e, harness, script)
	}

	for _, script := range DropColumnScripts {
		e, err := harness.NewEngine(t)
		require.NoError(t, err)
		enginetest.TestScriptWithEngine(t, e, harness, script)
	}
	if !types.IsFormat_DOLT(types.Format_Default) {
		t.Skip("not fixing unique index on keyless tables for old format")
	}
	for _, script := range AddIndexScripts {
		e, err := harness.NewEngine(t)
		require.NoError(t, err)
		enginetest.TestScriptWithEngine(t, e, harness, script)
	}

	// TODO: these scripts should be general enough to go in GMS
	for _, script := range AddDropPrimaryKeysScripts {
		e, err := harness.NewEngine(t)
		require.NoError(t, err)
		enginetest.TestScriptWithEngine(t, e, harness, script)
	}
}

func TestBrokenDdlScripts(t *testing.T) {
	for _, script := range BrokenDDLScripts {
		t.Skip(script.Name)
	}
}

func TestDescribeTableAsOf(t *testing.T) {
	h := newDoltHarness(t)
	defer h.Close()
	enginetest.TestScript(t, h, DescribeTableAsOfScriptTest)
}

func TestShowCreateTable(t *testing.T) {
	for _, script := range ShowCreateTableScriptTests {
		func() {
			h := newDoltHarness(t)
			defer h.Close()
			enginetest.TestScript(t, h, script)
		}()
	}
}

func TestViewsWithAsOf(t *testing.T) {
	h := newDoltHarness(t)
	defer h.Close()
	enginetest.TestScript(t, h, ViewsWithAsOfScriptTest)
}

func TestViewsWithAsOfPrepared(t *testing.T) {
	skipPreparedTests(t)
	h := newDoltHarness(t)
	defer h.Close()
	enginetest.TestScriptPrepared(t, h, ViewsWithAsOfScriptTest)
}

func TestDoltMerge(t *testing.T) {
	for _, script := range MergeScripts {
		// dolt versioning conflicts with reset harness -- use new harness every time
		func() {
			h := newDoltHarness(t).WithParallelism(1)
			defer h.Close()
			enginetest.TestScript(t, h, script)
		}()
	}

	if types.IsFormat_DOLT(types.Format_Default) {
		for _, script := range Dolt1MergeScripts {
			func() {
				h := newDoltHarness(t).WithParallelism(1)
				defer h.Close()
				enginetest.TestScript(t, h, script)
			}()
		}
	}
}

func TestDoltAutoIncrement(t *testing.T) {
	for _, script := range DoltAutoIncrementTests {
		// doing commits on different branches is antagonistic to engine reuse, use a new engine on each script
		func() {
			h := newDoltHarness(t)
			defer h.Close()
			enginetest.TestScript(t, h, script)
		}()
	}

	for _, script := range BrokenAutoIncrementTests {
		t.Run(script.Name, func(t *testing.T) {
			t.Skip()
			h := newDoltHarness(t)
			defer h.Close()
			enginetest.TestScript(t, h, script)
		})
	}
}

func TestDoltAutoIncrementPrepared(t *testing.T) {
	for _, script := range DoltAutoIncrementTests {
		// doing commits on different branches is antagonistic to engine reuse, use a new engine on each script
		func() {
			h := newDoltHarness(t)
			defer h.Close()
			enginetest.TestScriptPrepared(t, h, script)
		}()
	}

	for _, script := range BrokenAutoIncrementTests {
		t.Run(script.Name, func(t *testing.T) {
			t.Skip()
			h := newDoltHarness(t)
			defer h.Close()
			enginetest.TestScriptPrepared(t, h, script)
		})
	}
}

func TestDoltConflictsTableNameTable(t *testing.T) {
	for _, script := range DoltConflictTableNameTableTests {
		func() {
			h := newDoltHarness(t)
			defer h.Close()
			enginetest.TestScript(t, h, script)
		}()
	}

	if types.IsFormat_DOLT(types.Format_Default) {
		for _, script := range Dolt1ConflictTableNameTableTests {
			func() {
				h := newDoltHarness(t)
				defer h.Close()
				enginetest.TestScript(t, h, script)
			}()
		}
	}
}

// tests new format behavior for keyless merges that create CVs and conflicts
func TestKeylessDoltMergeCVsAndConflicts(t *testing.T) {
	if !types.IsFormat_DOLT(types.Format_Default) {
		t.Skip()
	}
	for _, script := range KeylessMergeCVsAndConflictsScripts {
		func() {
			h := newDoltHarness(t)
			defer h.Close()
			enginetest.TestScript(t, h, script)
		}()
	}
}

// eventually this will be part of TestDoltMerge
func TestDoltMergeArtifacts(t *testing.T) {
	if !types.IsFormat_DOLT(types.Format_Default) {
		t.Skip()
	}
	for _, script := range MergeArtifactsScripts {
		func() {
			h := newDoltHarness(t)
			defer h.Close()
			enginetest.TestScript(t, h, script)
		}()
	}
	for _, script := range SchemaConflictScripts {
		h := newDoltHarness(t)
		enginetest.TestScript(t, h, script)
		h.Close()
	}
}

// these tests are temporary while there is a difference between the old format
// and new format merge behaviors.
func TestOldFormatMergeConflictsAndCVs(t *testing.T) {
	if types.IsFormat_DOLT(types.Format_Default) {
		t.Skip()
	}
	for _, script := range OldFormatMergeConflictsAndCVsScripts {
		func() {
			h := newDoltHarness(t)
			defer h.Close()
			enginetest.TestScript(t, h, script)
		}()
	}
}

func TestDoltReset(t *testing.T) {
	for _, script := range DoltReset {
		// dolt versioning conflicts with reset harness -- use new harness every time
		func() {
			h := newDoltHarness(t)
			defer h.Close()
			enginetest.TestScript(t, h, script)
		}()
	}
}

func TestDoltGC(t *testing.T) {
	t.SkipNow()
	for _, script := range DoltGC {
		func() {
			h := newDoltHarness(t)
			defer h.Close()
			enginetest.TestScript(t, h, script)
		}()
	}
}

func TestDoltBranch(t *testing.T) {
	for _, script := range DoltBranchScripts {
		func() {
			h := newDoltHarness(t)
			defer h.Close()
			enginetest.TestScript(t, h, script)
		}()
	}
}

func TestDoltTag(t *testing.T) {
	for _, script := range DoltTagTestScripts {
		func() {
			h := newDoltHarness(t)
			defer h.Close()
			enginetest.TestScript(t, h, script)
		}()
	}
}

func TestDoltRemote(t *testing.T) {
	for _, script := range DoltRemoteTestScripts {
		func() {
			h := newDoltHarness(t)
			defer h.Close()
			enginetest.TestScript(t, h, script)
		}()
	}
}

type testCommitClock struct {
	unixNano int64
}

func (tcc *testCommitClock) Now() time.Time {
	now := time.Unix(0, tcc.unixNano)
	tcc.unixNano += int64(time.Hour)
	return now
}

func installTestCommitClock(tcc *testCommitClock) func() {
	oldNowFunc := datas.CommitNowFunc
	oldCommitLoc := datas.CommitLoc
	datas.CommitNowFunc = tcc.Now
	datas.CommitLoc = time.UTC
	return func() {
		datas.CommitNowFunc = oldNowFunc
		datas.CommitLoc = oldCommitLoc
	}
}

// TestSingleTransactionScript is a convenience method for debugging a single transaction test. Unskip and set to the
// desired test.
func TestSingleTransactionScript(t *testing.T) {
	t.Skip()

	tcc := &testCommitClock{}
	cleanup := installTestCommitClock(tcc)
	defer cleanup()

	sql.RunWithNowFunc(tcc.Now, func() error {

		script := queries.TransactionTest{
			Name: "committed conflicts are seen by other sessions",
			SetUpScript: []string{
				"CREATE TABLE test (pk int primary key, val int)",
				"CALL DOLT_ADD('.')",
				"INSERT INTO test VALUES (0, 0)",
				"CALL DOLT_COMMIT('-a', '-m', 'Step 1');",
				"CALL DOLT_CHECKOUT('-b', 'feature-branch')",
				"INSERT INTO test VALUES (1, 1);",
				"UPDATE test SET val=1000 WHERE pk=0;",
				"CALL DOLT_COMMIT('-a', '-m', 'this is a normal commit');",
				"CALL DOLT_CHECKOUT('main');",
				"UPDATE test SET val=1001 WHERE pk=0;",
				"CALL DOLT_COMMIT('-a', '-m', 'update a value');",
			},
			Assertions: []queries.ScriptTestAssertion{
				{
					Query:    "/* client a */ start transaction",
					Expected: []sql.Row{},
				},
				{
					Query:    "/* client b */ start transaction",
					Expected: []sql.Row{},
				},
				{
					Query: "/* client a */ select * from dolt_log order by date",
					Expected:
					// existing transaction logic
					[]sql.Row{
						sql.Row{"j131v1r3cf6mrdjjjuqgkv4t33oa0l54", "billy bob", "bigbillieb@fake.horse", time.Date(1969, time.December, 31, 21, 0, 0, 0, time.Local), "Initialize data repository"},
						sql.Row{"kcg4345ir3tjfb13mr0on1bv1m56h9if", "billy bob", "bigbillieb@fake.horse", time.Date(1970, time.January, 1, 4, 0, 0, 0, time.Local), "checkpoint enginetest database mydb"},
						sql.Row{"9jtjpggd4t5nso3mefilbde3tkfosdna", "billy bob", "bigbillieb@fake.horse", time.Date(1970, time.January, 1, 12, 0, 0, 0, time.Local), "Step 1"},
						sql.Row{"559f6kdh0mm5i1o40hs3t8dr43bkerav", "billy bob", "bigbillieb@fake.horse", time.Date(1970, time.January, 2, 3, 0, 0, 0, time.Local), "update a value"},
					},

					// new tx logic
					// 	[]sql.Row{
					// 	sql.Row{"j131v1r3cf6mrdjjjuqgkv4t33oa0l54", "billy bob", "bigbillieb@fake.horse", time.Date(1969, time.December, 31, 21, 0, 0, 0, time.Local), "Initialize data repository"},
					// 	sql.Row{"kcg4345ir3tjfb13mr0on1bv1m56h9if", "billy bob", "bigbillieb@fake.horse", time.Date(1970, time.January, 1, 4, 0, 0, 0, time.Local), "checkpoint enginetest database mydb"},
					// 	sql.Row{"pifio95ccefa03qstm1g3s1sivj1sm1d", "billy bob", "bigbillieb@fake.horse", time.Date(1970, time.January, 1, 11, 0, 0, 0, time.Local), "Step 1"},
					// 	sql.Row{"rdrgqfcml1hfgj8clr0caabgu014v2g9", "billy bob", "bigbillieb@fake.horse", time.Date(1970, time.January, 1, 20, 0, 0, 0, time.Local), "this is a normal commit"},
					// 	sql.Row{"shhv61eiefo9c4m9lvo5bt23i3om1ft4", "billy bob", "bigbillieb@fake.horse", time.Date(1970, time.January, 2, 2, 0, 0, 0, time.Local), "update a value"},
					// },
				},
				{
					Query:    "/* client a */ CALL DOLT_MERGE('feature-branch')",
					Expected: []sql.Row{{0, 1}},
				},
				{
					Query:    "/* client a */ SELECT count(*) from dolt_conflicts_test",
					Expected: []sql.Row{{1}},
				},
				{
					Query:    "/* client b */ SELECT count(*) from dolt_conflicts_test",
					Expected: []sql.Row{{0}},
				},
				{
					Query:    "/* client a */ set dolt_allow_commit_conflicts = 1",
					Expected: []sql.Row{{}},
				},
				{
					Query:    "/* client a */ commit",
					Expected: []sql.Row{},
				},
				{
					Query:    "/* client b */ start transaction",
					Expected: []sql.Row{},
				},
				{
					Query:    "/* client b */ SELECT count(*) from dolt_conflicts_test",
					Expected: []sql.Row{{1}},
				},
				{
					Query:    "/* client a */ start transaction",
					Expected: []sql.Row{},
				},
				{
					Query:    "/* client a */ CALL DOLT_MERGE('--abort')",
					Expected: []sql.Row{{0, 0}},
				},
				{
					Query:    "/* client a */ commit",
					Expected: []sql.Row{},
				},
				{
					Query:    "/* client b */ start transaction",
					Expected: []sql.Row{},
				},
				{
					Query:    "/* client a */ SET @@dolt_allow_commit_conflicts = 0",
					Expected: []sql.Row{{}},
				},
				{
					Query:          "/* client a */ CALL DOLT_MERGE('feature-branch')",
					ExpectedErrStr: dsess.ErrUnresolvedConflictsCommit.Error(),
				},
				{ // client rolled back on merge with conflicts
					Query:    "/* client a */ SELECT count(*) from dolt_conflicts_test",
					Expected: []sql.Row{{0}},
				},
				{
					Query:    "/* client a */ commit",
					Expected: []sql.Row{},
				},
				{
					Query:    "/* client b */ SELECT count(*) from dolt_conflicts_test",
					Expected: []sql.Row{{0}},
				},
			},
		}

		h := newDoltHarness(t)
		defer h.Close()
		enginetest.TestTransactionScript(t, h, script)

		return nil
	})
}

func TestBrokenSystemTableQueries(t *testing.T) {
	t.Skip()

	h := newDoltHarness(t)
	defer h.Close()
	enginetest.RunQueryTests(t, h, BrokenSystemTableQueries)
}

func TestHistorySystemTable(t *testing.T) {
	harness := newDoltHarness(t).WithParallelism(2)
	defer harness.Close()
	harness.Setup(setup.MydbData)
	for _, test := range HistorySystemTableScriptTests {
		harness.engine = nil
		t.Run(test.Name, func(t *testing.T) {
			enginetest.TestScript(t, harness, test)
		})
	}
}

func TestHistorySystemTablePrepared(t *testing.T) {
	harness := newDoltHarness(t).WithParallelism(2)
	defer harness.Close()
	harness.Setup(setup.MydbData)
	for _, test := range HistorySystemTableScriptTests {
		harness.engine = nil
		t.Run(test.Name, func(t *testing.T) {
			enginetest.TestScriptPrepared(t, harness, test)
		})
	}
}

func TestBrokenHistorySystemTablePrepared(t *testing.T) {
	t.Skip()
	harness := newDoltHarness(t)
	defer harness.Close()
	harness.Setup(setup.MydbData)
	for _, test := range BrokenHistorySystemTableScriptTests {
		harness.engine = nil
		t.Run(test.Name, func(t *testing.T) {
			enginetest.TestScriptPrepared(t, harness, test)
		})
	}
}

func TestUnscopedDiffSystemTable(t *testing.T) {
	for _, test := range UnscopedDiffSystemTableScriptTests {
		t.Run(test.Name, func(t *testing.T) {
			h := newDoltHarness(t)
			defer h.Close()
			enginetest.TestScript(t, h, test)
		})
	}
}

func TestUnscopedDiffSystemTablePrepared(t *testing.T) {
	for _, test := range UnscopedDiffSystemTableScriptTests {
		t.Run(test.Name, func(t *testing.T) {
			h := newDoltHarness(t)
			defer h.Close()
			enginetest.TestScriptPrepared(t, h, test)
		})
	}
}

func TestColumnDiffSystemTable(t *testing.T) {
	if !types.IsFormat_DOLT(types.Format_Default) {
		t.Skip("correct behavior of dolt_column_diff only guaranteed on new format")
	}
	for _, test := range ColumnDiffSystemTableScriptTests {
		t.Run(test.Name, func(t *testing.T) {
			enginetest.TestScriptPrepared(t, newDoltHarness(t), test)
		})
	}
}

func TestColumnDiffSystemTablePrepared(t *testing.T) {
	if !types.IsFormat_DOLT(types.Format_Default) {
		t.Skip("correct behavior of dolt_column_diff only guaranteed on new format")
	}
	for _, test := range ColumnDiffSystemTableScriptTests {
		t.Run(test.Name, func(t *testing.T) {
			enginetest.TestScriptPrepared(t, newDoltHarness(t), test)
		})
	}
}

func TestDiffTableFunction(t *testing.T) {
	harness := newDoltHarness(t)
	defer harness.Close()
	harness.Setup(setup.MydbData)
	for _, test := range DiffTableFunctionScriptTests {
		harness.engine = nil
		t.Run(test.Name, func(t *testing.T) {
			enginetest.TestScript(t, harness, test)
		})
	}
}

func TestDiffTableFunctionPrepared(t *testing.T) {
	harness := newDoltHarness(t)
	defer harness.Close()
	harness.Setup(setup.MydbData)
	for _, test := range DiffTableFunctionScriptTests {
		harness.engine = nil
		t.Run(test.Name, func(t *testing.T) {
			enginetest.TestScriptPrepared(t, harness, test)
		})
	}
}

func TestDiffStatTableFunction(t *testing.T) {
	harness := newDoltHarness(t)
	harness.Setup(setup.MydbData)
	for _, test := range DiffStatTableFunctionScriptTests {
		harness.engine = nil
		t.Run(test.Name, func(t *testing.T) {
			enginetest.TestScript(t, harness, test)
		})
	}
}

func TestDiffStatTableFunctionPrepared(t *testing.T) {
	harness := newDoltHarness(t)
	harness.Setup(setup.MydbData)
	for _, test := range DiffStatTableFunctionScriptTests {
		harness.engine = nil
		t.Run(test.Name, func(t *testing.T) {
			enginetest.TestScriptPrepared(t, harness, test)
		})
	}
}

func TestDiffSummaryTableFunction(t *testing.T) {
	harness := newDoltHarness(t)
	defer harness.Close()
	harness.Setup(setup.MydbData)
	for _, test := range DiffSummaryTableFunctionScriptTests {
		harness.engine = nil
		t.Run(test.Name, func(t *testing.T) {
			enginetest.TestScript(t, harness, test)
		})
	}
}

func TestDiffSummaryTableFunctionPrepared(t *testing.T) {
	harness := newDoltHarness(t)
	defer harness.Close()
	harness.Setup(setup.MydbData)
	for _, test := range DiffSummaryTableFunctionScriptTests {
		harness.engine = nil
		t.Run(test.Name, func(t *testing.T) {
			enginetest.TestScriptPrepared(t, harness, test)
		})
	}
}

func TestPatchTableFunction(t *testing.T) {
	harness := newDoltHarness(t)
	harness.Setup(setup.MydbData)
	for _, test := range PatchTableFunctionScriptTests {
		harness.engine = nil
		t.Run(test.Name, func(t *testing.T) {
			enginetest.TestScript(t, harness, test)
		})
	}
}

func TestPatchTableFunctionPrepared(t *testing.T) {
	harness := newDoltHarness(t)
	harness.Setup(setup.MydbData)
	for _, test := range PatchTableFunctionScriptTests {
		harness.engine = nil
		t.Run(test.Name, func(t *testing.T) {
			enginetest.TestScriptPrepared(t, harness, test)
		})
	}
}

func TestLogTableFunction(t *testing.T) {
	harness := newDoltHarness(t)
	defer harness.Close()
	harness.Setup(setup.MydbData)
	for _, test := range LogTableFunctionScriptTests {
		harness.engine = nil
		t.Run(test.Name, func(t *testing.T) {
			enginetest.TestScript(t, harness, test)
		})
	}
}

func TestLogTableFunctionPrepared(t *testing.T) {
	harness := newDoltHarness(t)
	defer harness.Close()
	harness.Setup(setup.MydbData)
	for _, test := range LogTableFunctionScriptTests {
		harness.engine = nil
		t.Run(test.Name, func(t *testing.T) {
			enginetest.TestScriptPrepared(t, harness, test)
		})
	}
}

func TestCommitDiffSystemTable(t *testing.T) {
	harness := newDoltHarness(t)
	defer harness.Close()
	harness.Setup(setup.MydbData)
	for _, test := range CommitDiffSystemTableScriptTests {
		harness.engine = nil
		t.Run(test.Name, func(t *testing.T) {
			enginetest.TestScript(t, harness, test)
		})
	}
}

func TestCommitDiffSystemTablePrepared(t *testing.T) {
	harness := newDoltHarness(t)
	defer harness.Close()
	harness.Setup(setup.MydbData)
	for _, test := range CommitDiffSystemTableScriptTests {
		harness.engine = nil
		t.Run(test.Name, func(t *testing.T) {
			enginetest.TestScriptPrepared(t, harness, test)
		})
	}
}

func TestDiffSystemTable(t *testing.T) {
	if !types.IsFormat_DOLT(types.Format_Default) {
		t.Skip("only new format support system table indexing")
	}

	harness := newDoltHarness(t)
	defer harness.Close()
	harness.Setup(setup.MydbData)
	for _, test := range DiffSystemTableScriptTests {
		harness.engine = nil
		t.Run(test.Name, func(t *testing.T) {
			enginetest.TestScript(t, harness, test)
		})
	}

	if types.IsFormat_DOLT(types.Format_Default) {
		for _, test := range Dolt1DiffSystemTableScripts {
			func() {
				h := newDoltHarness(t)
				defer h.Close()
				enginetest.TestScript(t, h, test)
			}()
		}
	}
}

func TestDiffSystemTablePrepared(t *testing.T) {
	if !types.IsFormat_DOLT(types.Format_Default) {
		t.Skip("only new format support system table indexing")
	}

	harness := newDoltHarness(t)
	defer harness.Close()
	harness.Setup(setup.MydbData)
	for _, test := range DiffSystemTableScriptTests {
		harness.engine = nil
		t.Run(test.Name, func(t *testing.T) {
			enginetest.TestScriptPrepared(t, harness, test)
		})
	}

	if types.IsFormat_DOLT(types.Format_Default) {
		for _, test := range Dolt1DiffSystemTableScripts {
			func() {
				h := newDoltHarness(t)
				defer h.Close()
				enginetest.TestScriptPrepared(t, h, test)
			}()
		}
	}
}

func mustNewEngine(t *testing.T, h enginetest.Harness) *gms.Engine {
	e, err := h.NewEngine(t)
	if err != nil {
		require.NoError(t, err)
	}
	return e
}

var biasedCosters = []analyzer.Coster{
	analyzer.NewInnerBiasedCoster(),
	analyzer.NewLookupBiasedCoster(),
	analyzer.NewHashBiasedCoster(),
	analyzer.NewMergeBiasedCoster(),
}

func TestSystemTableIndexes(t *testing.T) {
	if !types.IsFormat_DOLT(types.Format_Default) {
		t.Skip("only new format support system table indexing")
	}

	for _, stt := range SystemTableIndexTests {
		harness := newDoltHarness(t).WithParallelism(2)
		defer harness.Close()
		harness.SkipSetupCommit()
		e := mustNewEngine(t, harness)
		defer e.Close()
		e.Analyzer.Coster = analyzer.NewMergeBiasedCoster()

		ctx := enginetest.NewContext(harness)
		for _, q := range stt.setup {
			enginetest.RunQuery(t, e, harness, q)
		}

		for i, c := range []string{"inner", "lookup", "hash", "merge"} {
			e.Analyzer.Coster = biasedCosters[i]
			for _, tt := range stt.queries {
				t.Run(fmt.Sprintf("%s(%s): %s", stt.name, c, tt.query), func(t *testing.T) {
					if tt.skip {
						t.Skip()
					}

					ctx = ctx.WithQuery(tt.query)
					if tt.exp != nil {
						enginetest.TestQueryWithContext(t, ctx, e, harness, tt.query, tt.exp, nil, nil)
					}
				})
			}
		}
	}
}

func TestSystemTableIndexesPrepared(t *testing.T) {
	if !types.IsFormat_DOLT(types.Format_Default) {
		t.Skip("only new format support system table indexing")
	}

	for _, stt := range SystemTableIndexTests {
		harness := newDoltHarness(t).WithParallelism(2)
		defer harness.Close()
		harness.SkipSetupCommit()
		e := mustNewEngine(t, harness)
		defer e.Close()

		ctx := enginetest.NewContext(harness)
		for _, q := range stt.setup {
			enginetest.RunQuery(t, e, harness, q)
		}

		for _, tt := range stt.queries {
			t.Run(fmt.Sprintf("%s: %s", stt.name, tt.query), func(t *testing.T) {
				if tt.skip {
					t.Skip()
				}

				ctx = ctx.WithQuery(tt.query)
				if tt.exp != nil {
					enginetest.TestPreparedQueryWithContext(t, ctx, e, harness, tt.query, tt.exp, nil)
				}
			})
		}
	}
}

func TestReadOnlyDatabases(t *testing.T) {
	h := newDoltHarness(t)
	defer h.Close()
	enginetest.TestReadOnlyDatabases(t, h)
}

func TestAddDropPks(t *testing.T) {
	h := newDoltHarness(t)
	defer h.Close()
	enginetest.TestAddDropPks(t, h)
}

func TestAddAutoIncrementColumn(t *testing.T) {
	h := newDoltHarness(t)
	defer h.Close()
	enginetest.TestAddAutoIncrementColumn(t, h)
}

func TestNullRanges(t *testing.T) {
	h := newDoltHarness(t)
	defer h.Close()
	enginetest.TestNullRanges(t, h)
}

func TestPersist(t *testing.T) {
	harness := newDoltHarness(t)
	defer harness.Close()
	dEnv := dtestutils.CreateTestEnv()
	defer dEnv.DoltDB.Close()
	localConf, ok := dEnv.Config.GetConfig(env.LocalConfig)
	require.True(t, ok)
	globals := config.NewPrefixConfig(localConf, env.SqlServerGlobalsPrefix)
	newPersistableSession := func(ctx *sql.Context) sql.PersistableSession {
		session := ctx.Session.(*dsess.DoltSession).WithGlobals(globals)
		err := session.RemoveAllPersistedGlobals()
		require.NoError(t, err)
		return session
	}

	enginetest.TestPersist(t, harness, newPersistableSession)
}

func TestTypesOverWire(t *testing.T) {
	harness := newDoltHarness(t)
	defer harness.Close()
	enginetest.TestTypesOverWire(t, harness, newSessionBuilder(harness))
}

func TestDoltCommit(t *testing.T) {
	harness := newDoltHarness(t)
	defer harness.Close()
	for _, script := range DoltCommitTests {
		enginetest.TestScript(t, harness, script)
	}
}

func TestDoltCommitPrepared(t *testing.T) {
	harness := newDoltHarness(t)
	defer harness.Close()
	for _, script := range DoltCommitTests {
		enginetest.TestScriptPrepared(t, harness, script)
	}
}

func TestQueriesPrepared(t *testing.T) {
	h := newDoltHarness(t)
	defer h.Close()
	enginetest.TestQueriesPrepared(t, h)
}

func TestPreparedStaticIndexQuery(t *testing.T) {
	h := newDoltHarness(t)
	defer h.Close()
	enginetest.TestPreparedStaticIndexQuery(t, h)
}

func TestStatistics(t *testing.T) {
	h := newDoltHarness(t)
	defer h.Close()
	enginetest.TestStatistics(t, h)
}

func TestSpatialQueriesPrepared(t *testing.T) {
	skipPreparedTests(t)

	h := newDoltHarness(t)
	defer h.Close()
	enginetest.TestSpatialQueriesPrepared(t, h)
}

func TestPreparedStatistics(t *testing.T) {
	h := newDoltHarness(t)
	defer h.Close()
	enginetest.TestStatisticsPrepared(t, h)
}

func TestVersionedQueriesPrepared(t *testing.T) {
	skipPreparedTests(t)
	h := newDoltHarness(t)
	defer h.Close()
	enginetest.TestVersionedQueriesPrepared(t, h)
}

func TestInfoSchemaPrepared(t *testing.T) {
	skipPreparedTests(t)
	h := newDoltHarness(t)
	defer h.Close()
	enginetest.TestInfoSchemaPrepared(t, h)
}

func TestUpdateQueriesPrepared(t *testing.T) {
	skipPreparedTests(t)
	h := newDoltHarness(t)
	defer h.Close()
	enginetest.TestUpdateQueriesPrepared(t, h)
}

func TestInsertQueriesPrepared(t *testing.T) {
	skipPreparedTests(t)
	h := newDoltHarness(t)
	defer h.Close()
	enginetest.TestInsertQueriesPrepared(t, h)
}

func TestReplaceQueriesPrepared(t *testing.T) {
	skipPreparedTests(t)
	h := newDoltHarness(t)
	defer h.Close()
	enginetest.TestReplaceQueriesPrepared(t, h)
}

func TestDeleteQueriesPrepared(t *testing.T) {
	skipPreparedTests(t)
	h := newDoltHarness(t)
	defer h.Close()
	enginetest.TestDeleteQueriesPrepared(t, h)
}

func TestScriptsPrepared(t *testing.T) {
	var skipped []string
	if types.IsFormat_DOLT(types.Format_Default) {
		skipped = append(skipped, newFormatSkippedScripts...)
	}
	skipPreparedTests(t)
	h := newDoltHarness(t).WithSkippedQueries(skipped).WithParallelism(1)
	defer h.Close()
	enginetest.TestScriptsPrepared(t, h)
}

func TestInsertScriptsPrepared(t *testing.T) {
	skipPreparedTests(t)
	h := newDoltHarness(t)
	defer h.Close()
	enginetest.TestInsertScriptsPrepared(t, h)
}

func TestComplexIndexQueriesPrepared(t *testing.T) {
	skipPreparedTests(t)
	h := newDoltHarness(t)
	defer h.Close()
	enginetest.TestComplexIndexQueriesPrepared(t, h)
}

func TestJsonScriptsPrepared(t *testing.T) {
	skipPreparedTests(t)
	h := newDoltHarness(t)
	defer h.Close()
	enginetest.TestJsonScriptsPrepared(t, h)
}

func TestCreateCheckConstraintsScriptsPrepared(t *testing.T) {
	skipPreparedTests(t)
	h := newDoltHarness(t)
	defer h.Close()
	enginetest.TestCreateCheckConstraintsScriptsPrepared(t, h)
}

func TestInsertIgnoreScriptsPrepared(t *testing.T) {
	skipPreparedTests(t)
	h := newDoltHarness(t)
	defer h.Close()
	enginetest.TestInsertIgnoreScriptsPrepared(t, h)
}

func TestInsertErrorScriptsPrepared(t *testing.T) {
	skipPreparedTests(t)
	h := newDoltHarness(t)
	defer h.Close()
	enginetest.TestInsertErrorScriptsPrepared(t, h)
}

func TestViewsPrepared(t *testing.T) {
	skipPreparedTests(t)
	h := newDoltHarness(t)
	defer h.Close()
	enginetest.TestViewsPrepared(t, h)
}

func TestVersionedViewsPrepared(t *testing.T) {
	t.Skip("not supported for prepareds")
	skipPreparedTests(t)
	h := newDoltHarness(t)
	defer h.Close()
	enginetest.TestVersionedViewsPrepared(t, h)
}

func TestShowTableStatusPrepared(t *testing.T) {
	skipPreparedTests(t)
	h := newDoltHarness(t)
	defer h.Close()
	enginetest.TestShowTableStatusPrepared(t, h)
}

func TestPrepared(t *testing.T) {
	skipPreparedTests(t)
	h := newDoltHarness(t)
	defer h.Close()
	enginetest.TestPrepared(t, h)
}

func TestPreparedInsert(t *testing.T) {
	skipPreparedTests(t)
	h := newDoltHarness(t)
	defer h.Close()
	enginetest.TestPreparedInsert(t, h)
}

func TestPreparedStatements(t *testing.T) {
	skipPreparedTests(t)
	h := newDoltHarness(t)
	defer h.Close()
	enginetest.TestPreparedStatements(t, h)
}

func TestCharsetCollationEngine(t *testing.T) {
	skipOldFormat(t)
	h := newDoltHarness(t)
	defer h.Close()
	enginetest.TestCharsetCollationEngine(t, h)
}

func TestCharsetCollationWire(t *testing.T) {
	skipOldFormat(t)
	harness := newDoltHarness(t)
	defer harness.Close()
	enginetest.TestCharsetCollationWire(t, harness, newSessionBuilder(harness))
}

func TestDatabaseCollationWire(t *testing.T) {
	skipOldFormat(t)
	harness := newDoltHarness(t)
	defer harness.Close()
	enginetest.TestDatabaseCollationWire(t, harness, newSessionBuilder(harness))
}

func TestAddDropPrimaryKeys(t *testing.T) {
	t.Run("adding and dropping primary keys does not result in duplicate NOT NULL constraints", func(t *testing.T) {
		harness := newDoltHarness(t)
		defer harness.Close()
		addPkScript := queries.ScriptTest{
			Name: "add primary keys",
			SetUpScript: []string{
				"create table test (id int not null, c1 int);",
				"create index c1_idx on test(c1)",
				"insert into test values (1,1),(2,2)",
				"ALTER TABLE test ADD PRIMARY KEY(id)",
				"ALTER TABLE test DROP PRIMARY KEY",
				"ALTER TABLE test ADD PRIMARY KEY(id)",
				"ALTER TABLE test DROP PRIMARY KEY",
				"ALTER TABLE test ADD PRIMARY KEY(id)",
				"ALTER TABLE test DROP PRIMARY KEY",
				"ALTER TABLE test ADD PRIMARY KEY(id)",
			},
			Assertions: []queries.ScriptTestAssertion{
				{
					Query: "show create table test",
					Expected: []sql.Row{
						{"test", "CREATE TABLE `test` (\n" +
							"  `id` int NOT NULL,\n" +
							"  `c1` int,\n" +
							"  PRIMARY KEY (`id`),\n" +
							"  KEY `c1_idx` (`c1`)\n" +
							") ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_bin"},
					},
				},
			},
		}

		enginetest.TestScript(t, harness, addPkScript)

		// make sure there is only one NOT NULL constraint after all those mutations
		ctx := sql.NewContext(context.Background(), sql.WithSession(harness.session))
		ws, err := harness.session.WorkingSet(ctx, "mydb")
		require.NoError(t, err)

		table, ok, err := ws.WorkingRoot().GetTable(ctx, "test")
		require.NoError(t, err)
		require.True(t, ok)

		sch, err := table.GetSchema(ctx)
		for _, col := range sch.GetAllCols().GetColumns() {
			count := 0
			for _, cc := range col.Constraints {
				if cc.GetConstraintType() == schema.NotNullConstraintType {
					count++
				}
			}
			require.Less(t, count, 2)
		}
	})

	t.Run("Add primary key to table with index", func(t *testing.T) {
		harness := newDoltHarness(t)
		defer harness.Close()
		script := queries.ScriptTest{
			Name: "add primary keys to table with index",
			SetUpScript: []string{
				"create table test (id int not null, c1 int);",
				"create index c1_idx on test(c1)",
				"insert into test values (1,1),(2,2)",
				"ALTER TABLE test ADD constraint test_check CHECK (c1 > 0)",
				"ALTER TABLE test ADD PRIMARY KEY(id)",
			},
			Assertions: []queries.ScriptTestAssertion{
				{
					Query: "show create table test",
					Expected: []sql.Row{
						{"test", "CREATE TABLE `test` (\n" +
							"  `id` int NOT NULL,\n" +
							"  `c1` int,\n" +
							"  PRIMARY KEY (`id`),\n" +
							"  KEY `c1_idx` (`c1`),\n" +
							"  CONSTRAINT `test_check` CHECK ((`c1` > 0))\n" +
							") ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_bin"},
					},
				},
				{
					Query: "select * from test order by id",
					Expected: []sql.Row{
						{1, 1},
						{2, 2},
					},
				},
			},
		}
		enginetest.TestScript(t, harness, script)

		ctx := sql.NewContext(context.Background(), sql.WithSession(harness.session))
		ws, err := harness.session.WorkingSet(ctx, "mydb")
		require.NoError(t, err)

		table, ok, err := ws.WorkingRoot().GetTable(ctx, "test")
		require.NoError(t, err)
		require.True(t, ok)

		// Assert the new index map is not empty
		newRows, err := table.GetIndexRowData(ctx, "c1_idx")
		require.NoError(t, err)
		empty, err := newRows.Empty()
		require.NoError(t, err)
		assert.False(t, empty)
		count, err := newRows.Count()
		require.NoError(t, err)
		assert.Equal(t, count, uint64(2))
	})

	t.Run("Add primary key when one more cells contain NULL", func(t *testing.T) {
		harness := newDoltHarness(t)
		defer harness.Close()
		script := queries.ScriptTest{
			Name: "Add primary key when one more cells contain NULL",
			SetUpScript: []string{
				"create table test (id int not null, c1 int);",
				"create index c1_idx on test(c1)",
				"insert into test values (1,1),(2,2)",
				"ALTER TABLE test ADD PRIMARY KEY (c1)",
				"ALTER TABLE test ADD COLUMN (c2 INT NULL)",
				"ALTER TABLE test DROP PRIMARY KEY",
			},
			Assertions: []queries.ScriptTestAssertion{
				{
					Query:       "ALTER TABLE test ADD PRIMARY KEY (id, c1, c2)",
					ExpectedErr: sql.ErrInsertIntoNonNullableProvidedNull,
				},
			},
		}
		enginetest.TestScript(t, harness, script)
	})

	t.Run("Drop primary key from table with index", func(t *testing.T) {
		harness := newDoltHarness(t)
		defer harness.Close()
		script := queries.ScriptTest{
			Name: "Drop primary key from table with index",
			SetUpScript: []string{
				"create table test (id int not null primary key, c1 int);",
				"create index c1_idx on test(c1)",
				"insert into test values (1,1),(2,2)",
				"ALTER TABLE test DROP PRIMARY KEY",
			},
			Assertions: []queries.ScriptTestAssertion{
				{
					Query: "show create table test",
					Expected: []sql.Row{
						{"test", "CREATE TABLE `test` (\n" +
							"  `id` int NOT NULL,\n" +
							"  `c1` int,\n" +
							"  KEY `c1_idx` (`c1`)\n" +
							") ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_bin"},
					},
				},
				{
					Query: "select * from test order by id",
					Expected: []sql.Row{
						{1, 1},
						{2, 2},
					},
				},
			},
		}

		enginetest.TestScript(t, harness, script)

		ctx := sql.NewContext(context.Background(), sql.WithSession(harness.session))
		ws, err := harness.session.WorkingSet(ctx, "mydb")
		require.NoError(t, err)

		table, ok, err := ws.WorkingRoot().GetTable(ctx, "test")
		require.NoError(t, err)
		require.True(t, ok)

		// Assert the index map is not empty
		newIdx, err := table.GetIndexRowData(ctx, "c1_idx")
		assert.NoError(t, err)
		empty, err := newIdx.Empty()
		require.NoError(t, err)
		assert.False(t, empty)
		count, err := newIdx.Count()
		require.NoError(t, err)
		assert.Equal(t, count, uint64(2))
	})
}

func TestDoltVerifyConstraints(t *testing.T) {
	for _, script := range DoltVerifyConstraintsTestScripts {
		func() {
			harness := newDoltHarness(t)
			defer harness.Close()
			enginetest.TestScript(t, harness, script)
		}()
	}
}

func TestDoltStorageFormat(t *testing.T) {
	var expectedFormatString string
	if types.IsFormat_DOLT(types.Format_Default) {
		expectedFormatString = "NEW ( __DOLT__ )"
	} else {
		expectedFormatString = fmt.Sprintf("OLD ( %s )", types.Format_Default.VersionString())
	}
	script := queries.ScriptTest{
		Name: "dolt storage format function works",
		Assertions: []queries.ScriptTestAssertion{
			{
				Query:    "select dolt_storage_format()",
				Expected: []sql.Row{{expectedFormatString}},
			},
		},
	}
	h := newDoltHarness(t)
	defer h.Close()
	enginetest.TestScript(t, h, script)
}

func TestDoltStorageFormatPrepared(t *testing.T) {
	var expectedFormatString string
	if types.IsFormat_DOLT(types.Format_Default) {
		expectedFormatString = "NEW ( __DOLT__ )"
	} else {
		expectedFormatString = fmt.Sprintf("OLD ( %s )", types.Format_Default.VersionString())
	}
	h := newDoltHarness(t)
	defer h.Close()
	enginetest.TestPreparedQuery(t, h, "SELECT dolt_storage_format()", []sql.Row{{expectedFormatString}}, nil)
}

func TestThreeWayMergeWithSchemaChangeScripts(t *testing.T) {
	skipOldFormat(t)
	t.Run("right to left merges", func(t *testing.T) {
		for _, script := range ThreeWayMergeWithSchemaChangeTestScripts {
			func() {
				h := newDoltHarness(t)
				defer h.Close()
				enginetest.TestScript(t, h, convertMergeScriptTest(script, false))
			}()
		}
	})
	t.Run("left to right merges", func(t *testing.T) {
		for _, script := range ThreeWayMergeWithSchemaChangeTestScripts {
			func() {
				h := newDoltHarness(t)
				defer h.Close()
				enginetest.TestScript(t, h, convertMergeScriptTest(script, true))
			}()
		}
	})
}

func TestThreeWayMergeWithSchemaChangeScriptsPrepared(t *testing.T) {
	skipOldFormat(t)
	t.Run("right to left merges", func(t *testing.T) {
		for _, script := range ThreeWayMergeWithSchemaChangeTestScripts {
			func() {
				h := newDoltHarness(t)
				defer h.Close()
				enginetest.TestScript(t, h, convertMergeScriptTest(script, false))
			}()
		}
	})
	t.Run("left to right merges", func(t *testing.T) {
		for _, script := range ThreeWayMergeWithSchemaChangeTestScripts {
			func() {
				h := newDoltHarness(t)
				defer h.Close()
				enginetest.TestScript(t, h, convertMergeScriptTest(script, true))
			}()
		}
	})
}

var newFormatSkippedScripts = []string{
	// Different query plans
	"Partial indexes are used and return the expected result",
	"Multiple indexes on the same columns in a different order",
}

func skipOldFormat(t *testing.T) {
	if !types.IsFormat_DOLT(types.Format_Default) {
		t.Skip()
	}
}

func skipPreparedTests(t *testing.T) {
	if skipPrepared {
		t.Skip("skip prepared")
	}
}

func newSessionBuilder(harness *DoltHarness) server.SessionBuilder {
	return func(ctx context.Context, conn *mysql.Conn, host string) (sql.Session, error) {
		newCtx := harness.NewSession()
		return newCtx.Session, nil
	}
}
