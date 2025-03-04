// Copyright 2021 Dolthub, Inc.
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
	"fmt"
	"strings"

	"github.com/dolthub/go-mysql-server/enginetest/queries"
	"github.com/dolthub/go-mysql-server/sql"
	"github.com/dolthub/go-mysql-server/sql/expression"
	"github.com/dolthub/go-mysql-server/sql/plan"
	"github.com/dolthub/go-mysql-server/sql/types"

	"github.com/dolthub/dolt/go/libraries/doltcore/sqle"
)

var ViewsWithAsOfScriptTest = queries.ScriptTest{
	SkipPrepared: true,
	Name:         "Querying a view with a union using an as of expression",
	SetUpScript: []string{
		"CALL dolt_commit('--allow-empty', '-m', 'cm0');",

		"CREATE TABLE t1 (pk int PRIMARY KEY AUTO_INCREMENT, c0 int);",
		"CALL dolt_add('.')",
		"CALL dolt_commit('-am', 'cm1');",
		"INSERT INTO t1 (c0) VALUES (1), (2);",
		"CALL dolt_commit('-am', 'cm2');",

		"CREATE TABLE t2 (pk int PRIMARY KEY AUTO_INCREMENT, vc varchar(100));",
		"CALL dolt_add('.')",
		"CALL dolt_commit('-am', 'cm3');",
		"INSERT INTO t2 (vc) VALUES ('one'), ('two');",
		"CALL dolt_commit('-am', 'cm4');",

		"CREATE VIEW v1 as select * from t1 union select * from t2",
		"call dolt_add('.');",
		"CALL dolt_commit('-am', 'cm5');",
	},
	Assertions: []queries.ScriptTestAssertion{
		{
			Query:    "select * from v1",
			Expected: []sql.Row{{1, "1"}, {2, "2"}, {1, "one"}, {2, "two"}},
		},
		{
			Query:    "select * from v1 as of 'HEAD'",
			Expected: []sql.Row{{1, "1"}, {2, "2"}, {1, "one"}, {2, "two"}},
		},
		{
			Query:    "select * from v1 as of 'HEAD~1'",
			Expected: []sql.Row{{1, "1"}, {2, "2"}, {1, "one"}, {2, "two"}},
		},
		{
			Query:    "select * from v1 as of 'HEAD~2'",
			Expected: []sql.Row{{1, "1"}, {2, "2"}},
		},
		{
			// At this point table t1 doesn't exist yet, so the view should return an error
			Query:          "select * from v1 as of 'HEAD~3'",
			ExpectedErrStr: "table not found: t2, maybe you mean t1?",
		},
		{
			Query:          "select * from v1 as of 'HEAD~4'",
			ExpectedErrStr: "table not found: t2, maybe you mean t1?",
		},
		{
			Query:          "select * from v1 as of 'HEAD~5'",
			ExpectedErrStr: "table not found: t1",
		},
		{
			Query:    "select * from v1 as of HEAD",
			Expected: []sql.Row{{1, "1"}, {2, "2"}, {1, "one"}, {2, "two"}},
		},
		{
			Query:          "select * from v1 as of HEAD.ASDF",
			ExpectedErrStr: "branch not found: HEAD.ASDF",
		},
	},
}

var ShowCreateTableScriptTests = []queries.ScriptTest{
	{
		Name: "Show create table as of",
		SetUpScript: []string{
			"set @Commit0 = '';",
			"set @Commit1 = '';",
			"set @Commit2 = '';",
			"set @Commit3 = '';",
			"set @Commit0 = hashof('main');",
			"create table a (pk int primary key, c1 int);",
			"call dolt_add('.');",
			"call dolt_commit_hash_out(@Commit1, '-am', 'creating table a');",
			"alter table a add column c2 varchar(20);",
			"call dolt_commit_hash_out(@Commit2, '-am', 'adding column c2');",
			"alter table a drop column c1;",
			"alter table a add constraint unique_c2 unique(c2);",
			"call dolt_commit_hash_out(@Commit3, '-am', 'dropping column c1');",
		},
		Assertions: []queries.ScriptTestAssertion{
			{
				Query:       "show create table a as of @Commit0;",
				ExpectedErr: sql.ErrTableNotFound,
			},
			{
				Query: "show create table a as of @Commit1;",
				Expected: []sql.Row{
					{"a", "CREATE TABLE `a` (\n" +
						"  `pk` int NOT NULL,\n" +
						"  `c1` int,\n" +
						"  PRIMARY KEY (`pk`)\n" +
						") ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_bin",
					},
				},
			},
			{
				Query: "show create table a as of @Commit2;",
				Expected: []sql.Row{
					{"a", "CREATE TABLE `a` (\n" +
						"  `pk` int NOT NULL,\n" +
						"  `c1` int,\n" +
						"  `c2` varchar(20),\n" +
						"  PRIMARY KEY (`pk`)\n" +
						") ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_bin",
					},
				},
			},
			{
				Query: "show create table a as of @Commit3;",
				Expected: []sql.Row{
					{"a", "CREATE TABLE `a` (\n" +
						"  `pk` int NOT NULL,\n" +
						"  `c2` varchar(20),\n" +
						"  PRIMARY KEY (`pk`),\n" +
						"  UNIQUE KEY `unique_c2` (`c2`)\n" +
						") ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_bin",
					},
				},
			},
			{
				Query: "show create table a as of HEAD;",
				Expected: []sql.Row{
					{"a", "CREATE TABLE `a` (\n" +
						"  `pk` int NOT NULL,\n" +
						"  `c2` varchar(20),\n" +
						"  PRIMARY KEY (`pk`),\n" +
						"  UNIQUE KEY `unique_c2` (`c2`)\n" +
						") ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_bin",
					},
				},
			},
		},
	},
	{
		// "https://github.com/dolthub/dolt/issues/5478"
		Name: "show table for default types with unique indexes",
		SetUpScript: []string{
			`create table tbl (a int primary key,
                                   b int not null default 42,
                                   c int not null default (24),
                                   d int not null default '-108',
                                   e int not null default ((((7+11)))),
                                   f int default (now()))`,
			`call dolt_commit('-Am', 'new table');`,
			`create index tbl_bc on tbl (b,c);`,
			`create unique index tbl_cbd on tbl (c,b,d);`,
			`create unique index tbl_c on tbl (c);`,
			`create unique index tbl_e on tbl (e);`,
		},
		Assertions: []queries.ScriptTestAssertion{
			{
				Query: "show create table tbl",
				Expected: []sql.Row{{"tbl", "CREATE TABLE `tbl` (\n" +
					"  `a` int NOT NULL,\n" +
					"  `b` int NOT NULL DEFAULT '42',\n" + //
					"  `c` int NOT NULL DEFAULT (24),\n" + // Ensure these match setup above.
					"  `d` int NOT NULL DEFAULT '-108',\n" + //
					"  `e` int NOT NULL DEFAULT ((7 + 11)),\n" + // Matches MySQL behavior.
					"  `f` int DEFAULT (NOW()),\n" + // MySql preserves now as lower case.
					"  PRIMARY KEY (`a`),\n" +
					"  KEY `tbl_bc` (`b`,`c`),\n" +
					"  UNIQUE KEY `tbl_c` (`c`),\n" +
					"  UNIQUE KEY `tbl_cbd` (`c`,`b`,`d`),\n" +
					"  UNIQUE KEY `tbl_e` (`e`)\n" +
					") ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_bin"}},
			},
		},
	},
	{
		// "https://github.com/dolthub/dolt/issues/5478"
		Name: "show table for default types with unique indexes no PK",
		SetUpScript: []string{
			`create table tbl (a int not null default (now()),
                                   b int not null default 42,
                                   c int not null default (24),
                                   d int not null default '-108',
                                   e int not null default ((((7+11)))));`,
			`call dolt_commit('-Am', 'new table');`,
			`create index tbl_bc on tbl (b,c);`,
			`create unique index tbl_cab on tbl (c,a,b);`,
			`create unique index tbl_c on tbl (c);`,
			`create unique index tbl_e on tbl (e);`,
		},
		Assertions: []queries.ScriptTestAssertion{
			{
				Query: "show create table tbl",
				Expected: []sql.Row{{"tbl", "CREATE TABLE `tbl` (\n" +
					"  `a` int NOT NULL DEFAULT (NOW()),\n" + // MySql preserves now as lower case.
					"  `b` int NOT NULL DEFAULT '42',\n" + //
					"  `c` int NOT NULL DEFAULT (24),\n" + // Ensure these match setup above.
					"  `d` int NOT NULL DEFAULT '-108',\n" + //
					"  `e` int NOT NULL DEFAULT ((7 + 11)),\n" + // Matches MySQL behavior.
					"  KEY `tbl_bc` (`b`,`c`),\n" +
					"  UNIQUE KEY `tbl_c` (`c`),\n" +
					"  UNIQUE KEY `tbl_cab` (`c`,`a`,`b`),\n" +
					"  UNIQUE KEY `tbl_e` (`e`)\n" +
					") ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_bin"}},
			},
		},
	},
}

var DescribeTableAsOfScriptTest = queries.ScriptTest{
	Name: "Describe table as of",
	SetUpScript: []string{
		"set @Commit0 = '';",
		"set @Commit1 = '';",
		"set @Commit2 = '';",
		"set @Commit3 = '';",
		"call dolt_commit_hash_out(@Commit0, '--allow-empty', '-m', 'before creating table a');",
		"create table a (pk int primary key, c1 int);",
		"call dolt_add('.');",
		"call dolt_commit_hash_out(@Commit1, '-am', 'creating table a');",
		"alter table a add column c2 varchar(20);",
		"call dolt_commit_hash_out(@Commit2, '-am', 'adding column c2');",
		"alter table a drop column c1;",
		"call dolt_commit_hash_out(@Commit3, '-am', 'dropping column c1');",
	},
	Assertions: []queries.ScriptTestAssertion{
		{
			Query:       "describe a as of @Commit0;",
			ExpectedErr: sql.ErrTableNotFound,
		},
		{
			Query: "describe a as of @Commit1;",
			Expected: []sql.Row{
				{"pk", "int", "NO", "PRI", "NULL", ""},
				{"c1", "int", "YES", "", "NULL", ""},
			},
		},
		{
			Query: "describe a as of @Commit2;",
			Expected: []sql.Row{
				{"pk", "int", "NO", "PRI", "NULL", ""},
				{"c1", "int", "YES", "", "NULL", ""},
				{"c2", "varchar(20)", "YES", "", "NULL", ""},
			},
		},
		{
			Query: "describe a as of @Commit3;",
			Expected: []sql.Row{
				{"pk", "int", "NO", "PRI", "NULL", ""},
				{"c2", "varchar(20)", "YES", "", "NULL", ""},
			},
		},
		{
			Query: "describe a as of HEAD;",
			Expected: []sql.Row{
				{"pk", "int", "NO", "PRI", "NULL", ""},
				{"c2", "varchar(20)", "YES", "", "NULL", ""},
			},
		},
	},
}

var DoltRevisionDbScripts = []queries.ScriptTest{
	{
		Name: "database revision specs: Ancestor references",
		SetUpScript: []string{
			"create table t01 (pk int primary key, c1 int)",
			"call dolt_add('t01');",
			"call dolt_commit('-am', 'creating table t01 on main');",
			"call dolt_branch('branch1');",
			"insert into t01 values (1, 1), (2, 2);",
			"call dolt_commit('-am', 'adding rows to table t01 on main');",
			"insert into t01 values (3, 3);",
			"call dolt_commit('-am', 'adding another row to table t01 on main');",
			"call dolt_tag('tag1');",
			"call dolt_checkout('branch1');",
			"insert into t01 values (100, 100), (200, 200);",
			"call dolt_commit('-am', 'inserting rows in t01 on branch1');",
			"insert into t01 values (1000, 1000);",
			"call dolt_commit('-am', 'inserting another row in t01 on branch1');",
		},
		Assertions: []queries.ScriptTestAssertion{
			{
				Query:    "show databases;",
				Expected: []sql.Row{{"mydb"}, {"information_schema"}, {"mysql"}},
			},
			{
				Query:    "use `mydb/tag1~`;",
				Expected: []sql.Row{},
			},
			{
				// The database name should be the revision spec we started with, not its resolved hash
				Query:    "select database() regexp '^mydb/[0-9a-v]{32}$', database() = 'mydb/tag1~';",
				Expected: []sql.Row{{false, true}},
			},
			{
				Query:    "select * from t01;",
				Expected: []sql.Row{{1, 1}, {2, 2}},
			},
			{
				Query:    "select * from `mydb/tag1^`.t01;",
				Expected: []sql.Row{{1, 1}, {2, 2}},
			},
			{
				// Only merge commits are valid for ^2 ancestor spec
				Query:          "select * from `mydb/tag1^2`.t01;",
				ExpectedErrStr: "invalid ancestor spec",
			},
			{
				Query:    "select * from `mydb/tag1~1`.t01;",
				Expected: []sql.Row{{1, 1}, {2, 2}},
			},
			{
				Query:    "select * from `mydb/tag1~2`.t01;",
				Expected: []sql.Row{},
			},
			{
				Query:       "select * from `mydb/tag1~3`.t01;",
				ExpectedErr: sql.ErrTableNotFound,
			},
			{
				Query:          "select * from `mydb/tag1~20`.t01;",
				ExpectedErrStr: "invalid ancestor spec",
			},
			{
				Query:    "select * from `mydb/branch1~`.t01;",
				Expected: []sql.Row{{100, 100}, {200, 200}},
			},
			{
				Query:    "select * from `mydb/branch1^`.t01;",
				Expected: []sql.Row{{100, 100}, {200, 200}},
			},
			{
				Query:    "select * from `mydb/branch1~2`.t01;",
				Expected: []sql.Row{},
			},
			{
				Query:       "select * from `mydb/branch1~3`.t01;",
				ExpectedErr: sql.ErrTableNotFound,
			},
		},
	},
	{
		Name: "database revision specs: tag-qualified revision spec",
		SetUpScript: []string{
			"create table t01 (pk int primary key, c1 int)",
			"call dolt_add('.')",
			"call dolt_commit('-am', 'creating table t01 on main');",
			"insert into t01 values (1, 1), (2, 2);",
			"call dolt_commit('-am', 'adding rows to table t01 on main');",
			"call dolt_tag('tag1');",
			"insert into t01 values (3, 3);",
			"call dolt_commit('-am', 'adding another row to table t01 on main');",
		},
		Assertions: []queries.ScriptTestAssertion{
			{
				Query:    "show databases;",
				Expected: []sql.Row{{"mydb"}, {"information_schema"}, {"mysql"}},
			},
			{
				Query:    "use mydb/tag1;",
				Expected: []sql.Row{},
			},
			{
				Query:    "select database();",
				Expected: []sql.Row{{"mydb/tag1"}},
			},
			{
				Query:    "show databases;",
				Expected: []sql.Row{{"mydb"}, {"information_schema"}, {"mydb/tag1"}, {"mysql"}},
			},
			{
				Query:    "select * from t01;",
				Expected: []sql.Row{{1, 1}, {2, 2}},
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
	},
	{
		Name: "database revision specs: branch-qualified revision spec",
		SetUpScript: []string{
			"create table t01 (pk int primary key, c1 int)",
			"call dolt_add('.')",
			"call dolt_commit('-am', 'creating table t01 on main');",
			"insert into t01 values (1, 1), (2, 2);",
			"call dolt_commit('-am', 'adding rows to table t01 on main');",
			"call dolt_branch('branch1');",
			"insert into t01 values (3, 3);",
			"call dolt_commit('-am', 'adding another row to table t01 on main');",
		},
		Assertions: []queries.ScriptTestAssertion{
			{
				Query:    "use mydb/branch1;",
				Expected: []sql.Row{},
			},
			{
				Query:    "show databases;",
				Expected: []sql.Row{{"mydb"}, {"information_schema"}, {"mydb/branch1"}, {"mysql"}},
			},
			{
				Query:    "select database();",
				Expected: []sql.Row{{"mydb/branch1"}},
			},
			{
				Query:    "select * from t01",
				Expected: []sql.Row{{1, 1}, {2, 2}},
			},
			{
				Query:    "call dolt_checkout('main');",
				Expected: []sql.Row{{0}},
			},
			{
				Query:    "show databases;",
				Expected: []sql.Row{{"mydb"}, {"information_schema"}, {"mysql"}},
			},
			{
				Query:    "select database();",
				Expected: []sql.Row{{"mydb"}},
			},
			{
				Query:    "use mydb/branch1;",
				Expected: []sql.Row{},
			},
			{
				Query:    "call dolt_reset();",
				Expected: []sql.Row{{0}},
			},
			{
				Query:    "select database();",
				Expected: []sql.Row{{"mydb/branch1"}},
			},
			{
				Query:    "show databases;",
				Expected: []sql.Row{{"mydb"}, {"information_schema"}, {"mydb/branch1"}, {"mysql"}},
			},
			{
				Query:    "create table working_set_table(pk int primary key);",
				Expected: []sql.Row{{types.NewOkResult(0)}},
			},
			{
				// Create a table in the working set to verify the main db
				Query:    "select table_name from dolt_diff where commit_hash='WORKING';",
				Expected: []sql.Row{{"working_set_table"}},
			},
			{
				Query:    "use mydb;",
				Expected: []sql.Row{},
			},
			{
				Query:    "select table_name from dolt_diff where commit_hash='WORKING';",
				Expected: []sql.Row{},
			},
			{
				Query:    "call dolt_checkout('branch1');",
				Expected: []sql.Row{{0}},
			},
			{
				Query:    "select table_name from dolt_diff where commit_hash='WORKING';",
				Expected: []sql.Row{{"working_set_table"}},
			},
		},
	},
	{
		Name: "database revision specs: dolt_checkout uses revision database name for DbData access",
		SetUpScript: []string{
			"create database newtest;",
			"use newtest;",
		},
		Assertions: []queries.ScriptTestAssertion{
			{
				Query:    "select active_branch();",
				Expected: []sql.Row{{"main"}},
			},
			{
				Query:    "call dolt_checkout('-b', 'branch-to-delete');",
				Expected: []sql.Row{{0}},
			},
			{
				Query:    "select active_branch();",
				Expected: []sql.Row{{"branch-to-delete"}},
			},
			{
				Query:    "use `newtest/main`;",
				Expected: []sql.Row{},
			},
			{
				Query:    "select active_branch();",
				Expected: []sql.Row{{"main"}},
			},
			{
				Query:    "call dolt_branch('-D', 'branch-to-delete');",
				Expected: []sql.Row{{0}},
			},
			{
				Query:    "select active_branch();",
				Expected: []sql.Row{{"main"}},
			},
			{
				Query:    "call dolt_checkout('-b', 'another-branch');",
				Expected: []sql.Row{{0}},
			},
			{
				Query:    "select active_branch();",
				Expected: []sql.Row{{"another-branch"}},
			},
		},
	},
	{
		Name: "database revision specs: can checkout a table",
		SetUpScript: []string{
			"call dolt_checkout('main')",
			"create table t01 (pk int primary key, c1 int)",
			"call dolt_add('t01');",
			"call dolt_commit('-am', 'creating table t01 on branch1');",
			"insert into t01 values (1, 1), (2, 2);",
			"call dolt_branch('new-branch')",
		},
		Assertions: []queries.ScriptTestAssertion{
			{
				Query:    "show databases;",
				Expected: []sql.Row{{"mydb"}, {"information_schema"}, {"mysql"}},
			},
			{
				Query:    "use `mydb/main`;",
				Expected: []sql.Row{},
			},
			{
				Query:    "select * from dolt_status",
				Expected: []sql.Row{{"t01", false, "modified"}},
			},
			{
				Query:    "call dolt_checkout('t01')",
				Expected: []sql.Row{{0}},
			},
			{
				Query: "select * from dolt_status",
				// Expected: []sql.Row{},
				SkipResultsCheck: true, // TODO: https://github.com/dolthub/dolt/issues/5816
			},
		},
	},
}

// DoltScripts are script tests specific to Dolt (not the engine in general), e.g. by involving Dolt functions. Break
// this slice into others with good names as it grows.
var DoltScripts = []queries.ScriptTest{
	{
		Name: "test null filtering in secondary indexes (https://github.com/dolthub/dolt/issues/4199)",
		SetUpScript: []string{
			"create table t (pk int primary key auto_increment, d datetime, index index1 (d));",
			"insert into t (d) values (NOW()), (NOW());",
			"insert into t (d) values (NULL), (NULL);",
		},
		Assertions: []queries.ScriptTestAssertion{
			{
				Query:    "select count(*) from t where d is not null",
				Expected: []sql.Row{{2}},
			},
			{
				Query:    "select count(*) from t where d is null",
				Expected: []sql.Row{{2}},
			},
			{
				// Test the null-safe equals operator
				Query:    "select count(*) from t where d <=> NULL",
				Expected: []sql.Row{{2}},
			},
			{
				// Test the null-safe equals operator
				Query:    "select count(*) from t where not(d <=> null)",
				Expected: []sql.Row{{2}},
			},
			{
				// Test an IndexedJoin
				Query:    "select count(ifnull(t.d, 1)) from t, t as t2 where t.d is not null and t.pk = t2.pk and t2.d is not null;",
				Expected: []sql.Row{{2}},
			},
			{
				// Test an IndexedJoin
				Query:    "select count(ifnull(t.d, 1)) from t, t as t2 where t.d is null and t.pk = t2.pk and t2.d is null;",
				Expected: []sql.Row{{2}},
			},
			{
				// Test an IndexedJoin
				Query:    "select count(ifnull(t.d, 1)) from t, t as t2 where t.d is null and t.pk = t2.pk and t2.d is not null;",
				Expected: []sql.Row{{0}},
			},
		},
	},
	{
		Name: "test backticks in index name (https://github.com/dolthub/dolt/issues/3776)",
		SetUpScript: []string{
			"create table t (pk int primary key, c1 int)",
		},
		Assertions: []queries.ScriptTestAssertion{
			{
				Query:    "alter table t add index ```i```(c1);",
				Expected: []sql.Row{{types.OkResult{}}},
			},
			{
				Query: "show create table t;",
				Expected: []sql.Row{{"t",
					"CREATE TABLE `t` (\n" +
						"  `pk` int NOT NULL,\n" +
						"  `c1` int,\n" +
						"  PRIMARY KEY (`pk`),\n" +
						"  KEY ```i``` (`c1`)\n" +
						") ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_bin"}},
			},
		},
	},
	{
		Name: "test as of indexed join (https://github.com/dolthub/dolt/issues/2189)",
		SetUpScript: []string{
			"create table a (pk int primary key, c1 int)",
			"call DOLT_ADD('.')",
			"insert into a values (1,1), (2,2), (3,3)",
			"CALL DOLT_COMMIT('-a', '-m', 'first commit')",
			"insert into a values (4,4), (5,5), (6,6)",
			"CALL DOLT_COMMIT('-a', '-m', 'second commit')",
			"set @second_commit = (select commit_hash from dolt_log order by date desc limit 1)",
			"set @first_commit = (select commit_hash from dolt_log order by date desc limit 1,1)",
		},
		Assertions: []queries.ScriptTestAssertion{
			{
				Query: "select a1.* from a as of @second_commit a1 " +
					"left join a as of @first_commit a2 on a1.pk = a2.pk where a2.pk is null order by 1",
				Expected: []sql.Row{
					{4, 4},
					{5, 5},
					{6, 6},
				},
			},
			{
				Query: "select a1.* from a as of @second_commit a1 " +
					"left join a as of @second_commit a2 on a1.pk = a2.pk where a2.pk is null order by 1",
				Expected: []sql.Row{},
			},
		},
	},
	{
		Name: "Show create table with various keys and constraints",
		SetUpScript: []string{
			"create table t1(a int primary key, b varchar(10) not null default 'abc')",
			"alter table t1 add constraint ck1 check (b like '%abc%')",
			"create index t1b on t1(b)",
			"create table t2(c int primary key, d varchar(10))",
			"alter table t2 add constraint fk1 foreign key (d) references t1 (b)",
			"alter table t2 add constraint t2du unique (d)",
		},
		Assertions: []queries.ScriptTestAssertion{
			{
				Query: "show create table t1",
				Expected: []sql.Row{
					{"t1", "CREATE TABLE `t1` (\n" +
						"  `a` int NOT NULL,\n" +
						"  `b` varchar(10) NOT NULL DEFAULT 'abc',\n" +
						"  PRIMARY KEY (`a`),\n" +
						"  KEY `t1b` (`b`),\n" +
						"  CONSTRAINT `ck1` CHECK (`b` LIKE '%abc%')\n" +
						") ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_bin"},
				},
			},
			{
				Query: "show create table t2",
				Expected: []sql.Row{
					{"t2", "CREATE TABLE `t2` (\n" +
						"  `c` int NOT NULL,\n" +
						"  `d` varchar(10),\n" +
						"  PRIMARY KEY (`c`),\n" +
						"  UNIQUE KEY `t2du` (`d`),\n" +
						"  CONSTRAINT `fk1` FOREIGN KEY (`d`) REFERENCES `t1` (`b`)\n" +
						") ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_bin"},
				},
			},
		},
	},
	{
		Name: "Query table with 10K rows ",
		SetUpScript: []string{
			"create table bigTable (pk int primary key, c0 int);",
			makeLargeInsert(10_000),
		},
		Assertions: []queries.ScriptTestAssertion{
			{
				Query: "select count(*) from bigTable;",
				Expected: []sql.Row{
					{int32(10_000)},
				},
			},
			{
				Query: "select * from bigTable order by pk limit 5 offset 9990;",
				Expected: []sql.Row{
					{int64(9990), int64(9990)},
					{int64(9991), int64(9991)},
					{int64(9992), int64(9992)},
					{int64(9993), int64(9993)},
					{int64(9994), int64(9994)},
				},
			},
		},
	},
	{
		Name: "SHOW CREATE PROCEDURE works with Dolt external procedures",
		Assertions: []queries.ScriptTestAssertion{
			{
				Query: "SHOW CREATE PROCEDURE dolt_checkout;",
				Expected: []sql.Row{
					{
						"dolt_checkout",
						"",
						"CREATE PROCEDURE dolt_checkout() SELECT 'External stored procedure';",
						"utf8mb4",
						"utf8mb4_0900_bin",
						"utf8mb4_0900_bin",
					},
				},
			},
		},
	},
	{
		Name: "Prepared ASOF",
		SetUpScript: []string{
			"set @Commit1 = '';",
			"set @Commit2 = '';",
			"create table test (pk int primary key, c1 int)",
			"call dolt_add('.')",
			"insert into test values (0,0), (1,1);",
			"call dolt_commit_hash_out(@Commit1, '-am', 'creating table');",
			"call dolt_branch('-c', 'main', 'newb')",
			"alter table test add column c2 int;",
			"call dolt_commit_hash_out(@Commit2, '-am', 'alter table');",
		},
		Assertions: []queries.ScriptTestAssertion{
			{
				Query:    "select * from test as of 'HEAD~' where pk=?;",
				Expected: []sql.Row{{0, 0}},
				Bindings: map[string]sql.Expression{
					"v1": expression.NewLiteral(0, types.Int8),
				},
			},
			{
				Query:    "select * from test as of hashof('HEAD') where pk=?;",
				Expected: []sql.Row{{1, 1, nil}},
				Bindings: map[string]sql.Expression{
					"v1": expression.NewLiteral(1, types.Int8),
				},
			},
			{
				Query:    "select * from test as of @Commit1 where pk=?;",
				Expected: []sql.Row{{0, 0}},
				Bindings: map[string]sql.Expression{
					"v1": expression.NewLiteral(0, types.Int8),
				},
			},
			{
				Query:    "select * from test as of @Commit2 where pk=?;",
				Expected: []sql.Row{{0, 0, nil}},
				Bindings: map[string]sql.Expression{
					"v1": expression.NewLiteral(0, types.Int8),
				},
			},
		},
	},
	{
		Name: "blame: composite pk ordered output with correct header (bats repro)",
		SetUpScript: []string{
			"CREATE TABLE t(pk varchar(20), val int)",
			"ALTER TABLE t ADD PRIMARY KEY (pk, val)",
			"INSERT INTO t VALUES ('zzz',4),('mult',1),('sub',2),('add',5)",
			"CALL dadd('.');",
			"CALL dcommit('-am', 'add rows');",
			"INSERT INTO t VALUES ('dolt',0),('alt',12),('del',8),('ctl',3)",
			"CALL dcommit('-am', 'add more rows');",
		},
		Assertions: []queries.ScriptTestAssertion{
			{
				Query: "SELECT pk, val, message FROM dolt_blame_t",
				Expected: []sql.Row{
					{"add", 5, "add rows"},
					{"alt", 12, "add more rows"},
					{"ctl", 3, "add more rows"},
					{"del", 8, "add more rows"},
					{"dolt", 0, "add more rows"},
					{"mult", 1, "add rows"},
					{"sub", 2, "add rows"},
					{"zzz", 4, "add rows"},
				},
			},
		},
	},
	{
		Name: "Nautobot FOREIGN KEY panic repro",
		SetUpScript: []string{
			"CREATE TABLE `auth_user` (" +
				"	`password` varchar(128) NOT NULL," +
				"	`last_login` datetime," +
				"	`is_superuser` tinyint NOT NULL," +
				"	`username` varchar(150) NOT NULL," +
				"	`first_name` varchar(150) NOT NULL," +
				"	`last_name` varchar(150) NOT NULL," +
				"	`email` varchar(254) NOT NULL," +
				"	`is_staff` tinyint NOT NULL," +
				"	`is_active` tinyint NOT NULL," +
				"	`date_joined` datetime NOT NULL," +
				"	`id` char(32) NOT NULL," +
				"	`config_data` json NOT NULL," +
				"	PRIMARY KEY (`id`)," +
				"	UNIQUE KEY `username` (`username`)" +
				") ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_bin",
			"CREATE TABLE `users_token` (" +
				"	`id` char(32) NOT NULL," +
				"	`created` datetime NOT NULL," +
				"	`expires` datetime," +
				"	`key` varchar(40) NOT NULL," +
				"	`write_enabled` tinyint NOT NULL," +
				"	`description` varchar(200) NOT NULL," +
				"	`user_id` char(32) NOT NULL," +
				"	PRIMARY KEY (`id`)," +
				"	UNIQUE KEY `key` (`key`)," +
				"	KEY `users_token_user_id_af964690` (`user_id`)," +
				"	CONSTRAINT `users_token_user_id_af964690_fk_auth_user_id` FOREIGN KEY (`user_id`) REFERENCES `auth_user` (`id`)" +
				") ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_bin;",
			"INSERT INTO `auth_user` (`password`,`last_login`,`is_superuser`,`username`,`first_name`,`last_name`,`email`,`is_staff`,`is_active`,`date_joined`,`id`,`config_data`)" +
				"VALUES ('pbkdf2_sha256$216000$KRpZeDPgwc5E$vl/2hwrmtnckaBT0A8pf63Ph+oYuCHYI7qozMTZihTo=',NULL,1,'admin','','','admin@example.com',1,1,'2022-08-30 18:27:21.810049','1056443cc03446c592fa4c06bb06a1a6','{}');",
		},
		Assertions: []queries.ScriptTestAssertion{
			{
				Query: "INSERT INTO `users_token` (`id`, `user_id`, `created`, `expires`, `key`, `write_enabled`, `description`) " +
					"VALUES ('acc2e157db2845a79221cc654b1dcecc', '1056443cc03446c592fa4c06bb06a1a6', '2022-08-30 18:27:21.948487', NULL, '0123456789abcdef0123456789abcdef01234567', 1, '');",
				Expected: []sql.Row{{types.OkResult{RowsAffected: 0x1, InsertID: 0x0}}},
			},
		},
	},
	{
		Name: "dolt_schemas schema",
		SetUpScript: []string{
			"CREATE TABLE viewtest(v1 int, v2 int)",
			"CREATE VIEW view1 AS SELECT v1 FROM viewtest",
			"CREATE VIEW view2 AS SELECT v2 FROM viewtest",
		},
		Assertions: []queries.ScriptTestAssertion{
			{
				Query: "SELECT type, name, fragment FROM dolt_schemas ORDER BY 1, 2",
				Expected: []sql.Row{
					{"view", "view1", "CREATE VIEW view1 AS SELECT v1 FROM viewtest"},
					{"view", "view2", "CREATE VIEW view2 AS SELECT v2 FROM viewtest"},
				},
			},
			{
				Query:       "CREATE VIEW VIEW1 AS SELECT v2 FROM viewtest",
				ExpectedErr: sql.ErrExistingView,
			},
			{
				Query:            "drop view view1",
				SkipResultsCheck: true,
			},
			{
				Query: "SELECT type, name, fragment FROM dolt_schemas ORDER BY 1, 2",
				Expected: []sql.Row{
					{"view", "view2", "CREATE VIEW view2 AS SELECT v2 FROM viewtest"},
				},
			},
			{
				Query:            "CREATE VIEW VIEW1 AS SELECT v1 FROM viewtest",
				SkipResultsCheck: true,
			},
			{
				Query: "SELECT type, name, fragment FROM dolt_schemas ORDER BY 1, 2",
				Expected: []sql.Row{
					{"view", "view1", "CREATE VIEW VIEW1 AS SELECT v1 FROM viewtest"},
					{"view", "view2", "CREATE VIEW view2 AS SELECT v2 FROM viewtest"},
				},
			},
		},
	},
	{
		Name: "test hashof",
		SetUpScript: []string{
			"CREATE TABLE hashof_test (pk int primary key, c1 int)",
			"INSERT INTO hashof_test values (1,1), (2,2), (3,3)",
			"CALL DOLT_ADD('hashof_test')",
			"CALL DOLT_COMMIT('-a', '-m', 'first commit')",
			"SET @Commit1 = (SELECT commit_hash FROM DOLT_LOG() LIMIT 1)",
			"INSERT INTO hashof_test values (4,4), (5,5), (6,6)",
			"CALL DOLT_COMMIT('-a', '-m', 'second commit')",
			"SET @Commit2 = (SELECT commit_hash from DOLT_LOG() LIMIT 1)",
		},
		Assertions: []queries.ScriptTestAssertion{
			{
				Query:    "SELECT (hashof(@Commit1) = hashof(@Commit2))",
				Expected: []sql.Row{{false}},
			},
			{
				Query: "SELECT (hashof(@Commit1) = hashof('HEAD~1'))",
				Expected: []sql.Row{
					{true},
				},
			},
			{
				Query: "SELECT (hashof(@Commit2) = hashof('HEAD'))",
				Expected: []sql.Row{
					{true},
				},
			},
			{
				Query: "SELECT (hashof(@Commit2) = hashof('main'))",
				Expected: []sql.Row{
					{true},
				},
			},
			{
				Query:          "SELECT hashof('non_branch')",
				ExpectedErrStr: "invalid ref spec",
			},
			{
				// Test that a short commit is invalid. This may change in the future.
				Query:          "SELECT hashof(left(@Commit2,30))",
				ExpectedErrStr: "invalid ref spec",
			},
		},
	},
}

func makeLargeInsert(sz int) string {
	var sb strings.Builder
	sb.WriteString("insert into bigTable values (0,0)")
	for i := 1; i < sz; i++ {
		sb.WriteString(fmt.Sprintf(",(%d,%d)", i, i))
	}
	sb.WriteString(";")
	return sb.String()
}

// DoltUserPrivTests are tests for Dolt-specific functionality that includes privilege checking logic.
var DoltUserPrivTests = []queries.UserPrivilegeTest{
	{
		Name: "table function privilege checking",
		SetUpScript: []string{
			"CREATE TABLE mydb.test (pk BIGINT PRIMARY KEY);",
			"CREATE TABLE mydb.test2 (pk BIGINT PRIMARY KEY);",
			"CALL DOLT_ADD('.')",
			"CALL DOLT_COMMIT('-am', 'creating tables test and test2');",
			"INSERT INTO mydb.test VALUES (1);",
			"CALL DOLT_COMMIT('-am', 'inserting into test');",
			"CREATE USER tester@localhost;",
		},
		Assertions: []queries.UserPrivilegeTestAssertion{
			{
				// Without access to the database, dolt_diff should fail with a database access error
				User:        "tester",
				Host:        "localhost",
				Query:       "SELECT * FROM dolt_diff('main~', 'main', 'test');",
				ExpectedErr: sql.ErrDatabaseAccessDeniedForUser,
			},
			{
				// Without access to the database, dolt_diff with dots should fail with a database access error
				User:        "tester",
				Host:        "localhost",
				Query:       "SELECT * FROM dolt_diff('main~..main', 'test');",
				ExpectedErr: sql.ErrDatabaseAccessDeniedForUser,
			},
			{
				// Without access to the database, dolt_diff_stat should fail with a database access error
				User:        "tester",
				Host:        "localhost",
				Query:       "SELECT * FROM dolt_diff_stat('main~', 'main', 'test');",
				ExpectedErr: sql.ErrDatabaseAccessDeniedForUser,
			},
			{
				// Without access to the database, dolt_diff_stat with dots should fail with a database access error
				User:        "tester",
				Host:        "localhost",
				Query:       "SELECT * FROM dolt_diff_stat('main~..main', 'test');",
				ExpectedErr: sql.ErrDatabaseAccessDeniedForUser,
			},
			{
				// Without access to the database, dolt_diff_summary should fail with a database access error
				User:        "tester",
				Host:        "localhost",
				Query:       "SELECT * FROM dolt_diff_summary('main~', 'main', 'test');",
				ExpectedErr: sql.ErrDatabaseAccessDeniedForUser,
			},
			{
				// Without access to the database, dolt_diff_summary with dots should fail with a database access error
				User:        "tester",
				Host:        "localhost",
				Query:       "SELECT * FROM dolt_diff_summary('main~..main', 'test');",
				ExpectedErr: sql.ErrDatabaseAccessDeniedForUser,
			},
			{
				// Without access to the database, dolt_patch should fail with a database access error
				User:        "tester",
				Host:        "localhost",
				Query:       "SELECT * FROM dolt_patch('main~', 'main', 'test');",
				ExpectedErr: sql.ErrDatabaseAccessDeniedForUser,
			},
			{
				// Without access to the database, dolt_patch with dots should fail with a database access error
				User:        "tester",
				Host:        "localhost",
				Query:       "SELECT * FROM dolt_patch('main~..main', 'test');",
				ExpectedErr: sql.ErrDatabaseAccessDeniedForUser,
			},
			{
				// Without access to the database, dolt_log should fail with a database access error
				User:        "tester",
				Host:        "localhost",
				Query:       "SELECT * FROM dolt_log('main');",
				ExpectedErr: sql.ErrDatabaseAccessDeniedForUser,
			},
			{
				// Grant single-table access to the underlying user table
				User:     "root",
				Host:     "localhost",
				Query:    "GRANT SELECT ON mydb.test TO tester@localhost;",
				Expected: []sql.Row{{types.NewOkResult(0)}},
			},
			{
				// After granting access to mydb.test, dolt_diff should work
				User:     "tester",
				Host:     "localhost",
				Query:    "SELECT COUNT(*) FROM dolt_diff('main~', 'main', 'test');",
				Expected: []sql.Row{{1}},
			},
			{
				// After granting access to mydb.test, dolt_diff with dots should work
				User:     "tester",
				Host:     "localhost",
				Query:    "SELECT COUNT(*) FROM dolt_diff('main~..main', 'test');",
				Expected: []sql.Row{{1}},
			},
			{
				// With access to the db, but not the table, dolt_diff should fail
				User:        "tester",
				Host:        "localhost",
				Query:       "SELECT * FROM dolt_diff('main~', 'main', 'test2');",
				ExpectedErr: sql.ErrPrivilegeCheckFailed,
			},
			{
				// With access to the db, but not the table, dolt_diff with dots should fail
				User:        "tester",
				Host:        "localhost",
				Query:       "SELECT * FROM dolt_diff('main~..main', 'test2');",
				ExpectedErr: sql.ErrPrivilegeCheckFailed,
			},
			{
				// With access to the db, but not the table, dolt_diff_stat should fail
				User:        "tester",
				Host:        "localhost",
				Query:       "SELECT * FROM dolt_diff_stat('main~', 'main', 'test2');",
				ExpectedErr: sql.ErrPrivilegeCheckFailed,
			},
			{
				// With access to the db, but not the table, dolt_diff_stat with dots should fail
				User:        "tester",
				Host:        "localhost",
				Query:       "SELECT * FROM dolt_diff_stat('main~...main', 'test2');",
				ExpectedErr: sql.ErrPrivilegeCheckFailed,
			},
			{
				// With access to the db, dolt_diff_stat should fail for all tables if no access any of tables
				User:        "tester",
				Host:        "localhost",
				Query:       "SELECT * FROM dolt_diff_stat('main~', 'main');",
				ExpectedErr: sql.ErrPrivilegeCheckFailed,
			},
			{
				// With access to the db, dolt_diff_stat with dots should fail for all tables if no access any of tables
				User:        "tester",
				Host:        "localhost",
				Query:       "SELECT * FROM dolt_diff_stat('main~...main');",
				ExpectedErr: sql.ErrPrivilegeCheckFailed,
			},
			{
				// With access to the db, but not the table, dolt_diff_summary should fail
				User:        "tester",
				Host:        "localhost",
				Query:       "SELECT * FROM dolt_diff_summary('main~', 'main', 'test2');",
				ExpectedErr: sql.ErrPrivilegeCheckFailed,
			},
			{
				// With access to the db, but not the table, dolt_diff_summary with dots should fail
				User:        "tester",
				Host:        "localhost",
				Query:       "SELECT * FROM dolt_diff_summary('main~...main', 'test2');",
				ExpectedErr: sql.ErrPrivilegeCheckFailed,
			},
			{
				// With access to the db, dolt_diff_summary should fail for all tables if no access any of tables
				User:        "tester",
				Host:        "localhost",
				Query:       "SELECT * FROM dolt_diff_summary('main~', 'main');",
				ExpectedErr: sql.ErrPrivilegeCheckFailed,
			},
			{
				// With access to the db, dolt_diff_summary with dots should fail for all tables if no access any of tables
				User:        "tester",
				Host:        "localhost",
				Query:       "SELECT * FROM dolt_diff_summary('main~...main');",
				ExpectedErr: sql.ErrPrivilegeCheckFailed,
			},
			{
				// With access to the db, but not the table, dolt_patch should fail
				User:        "tester",
				Host:        "localhost",
				Query:       "SELECT * FROM dolt_patch('main~', 'main', 'test2');",
				ExpectedErr: sql.ErrPrivilegeCheckFailed,
			},
			{
				// With access to the db, but not the table, dolt_patch with dots should fail
				User:        "tester",
				Host:        "localhost",
				Query:       "SELECT * FROM dolt_patch('main~...main', 'test2');",
				ExpectedErr: sql.ErrPrivilegeCheckFailed,
			},
			{
				// With access to the db, dolt_patch should fail for all tables if no access any of tables
				User:        "tester",
				Host:        "localhost",
				Query:       "SELECT * FROM dolt_patch('main~', 'main');",
				ExpectedErr: sql.ErrPrivilegeCheckFailed,
			},
			{
				// With access to the db, dolt_patch with dots should fail for all tables if no access any of tables
				User:        "tester",
				Host:        "localhost",
				Query:       "SELECT * FROM dolt_patch('main~...main');",
				ExpectedErr: sql.ErrPrivilegeCheckFailed,
			},
			{
				// Revoke select on mydb.test
				User:     "root",
				Host:     "localhost",
				Query:    "REVOKE SELECT ON mydb.test from tester@localhost;",
				Expected: []sql.Row{{types.NewOkResult(0)}},
			},
			{
				// After revoking access, dolt_diff should fail
				User:        "tester",
				Host:        "localhost",
				Query:       "SELECT * FROM dolt_diff('main~', 'main', 'test');",
				ExpectedErr: sql.ErrDatabaseAccessDeniedForUser,
			},
			{
				// After revoking access, dolt_diff with dots should fail
				User:        "tester",
				Host:        "localhost",
				Query:       "SELECT * FROM dolt_diff('main~..main', 'test');",
				ExpectedErr: sql.ErrDatabaseAccessDeniedForUser,
			},
			{
				// Grant multi-table access for all of mydb
				User:     "root",
				Host:     "localhost",
				Query:    "GRANT SELECT ON mydb.* to tester@localhost;",
				Expected: []sql.Row{{types.NewOkResult(0)}},
			},
			{
				// After granting access to the entire db, dolt_diff should work
				User:     "tester",
				Host:     "localhost",
				Query:    "SELECT COUNT(*) FROM dolt_diff('main~', 'main', 'test');",
				Expected: []sql.Row{{1}},
			},
			{
				// After granting access to the entire db, dolt_diff should work
				User:     "tester",
				Host:     "localhost",
				Query:    "SELECT COUNT(*) FROM dolt_diff('main~..main', 'test');",
				Expected: []sql.Row{{1}},
			},
			{
				// After granting access to the entire db, dolt_diff_stat should work
				User:     "tester",
				Host:     "localhost",
				Query:    "SELECT COUNT(*) FROM dolt_diff_stat('main~', 'main');",
				Expected: []sql.Row{{1}},
			},
			{
				// After granting access to the entire db, dolt_diff_stat with dots should work
				User:     "tester",
				Host:     "localhost",
				Query:    "SELECT COUNT(*) FROM dolt_diff_stat('main~...main');",
				Expected: []sql.Row{{1}},
			},
			{
				// After granting access to the entire db, dolt_diff_summary should work
				User:     "tester",
				Host:     "localhost",
				Query:    "SELECT COUNT(*) FROM dolt_diff_summary('main~', 'main');",
				Expected: []sql.Row{{1}},
			},
			{
				// After granting access to the entire db, dolt_diff_summary with dots should work
				User:     "tester",
				Host:     "localhost",
				Query:    "SELECT COUNT(*) FROM dolt_diff_summary('main~...main');",
				Expected: []sql.Row{{1}},
			},
			{
				// After granting access to the entire db, dolt_patch should work
				User:     "tester",
				Host:     "localhost",
				Query:    "SELECT COUNT(*) FROM dolt_patch('main~', 'main');",
				Expected: []sql.Row{{1}},
			},
			{
				// After granting access to the entire db, dolt_patch with dots should work
				User:     "tester",
				Host:     "localhost",
				Query:    "SELECT COUNT(*) FROM dolt_patch('main~...main');",
				Expected: []sql.Row{{1}},
			},
			{
				// After granting access to the entire db, dolt_log should work
				User:     "tester",
				Host:     "localhost",
				Query:    "SELECT COUNT(*) FROM dolt_log('main');",
				Expected: []sql.Row{{4}},
			},
			{
				// Revoke multi-table access
				User:     "root",
				Host:     "localhost",
				Query:    "REVOKE SELECT ON mydb.* from tester@localhost;",
				Expected: []sql.Row{{types.NewOkResult(0)}},
			},
			{
				// After revoking access, dolt_diff should fail
				User:        "tester",
				Host:        "localhost",
				Query:       "SELECT * FROM dolt_diff('main~', 'main', 'test');",
				ExpectedErr: sql.ErrDatabaseAccessDeniedForUser,
			},
			{
				// After revoking access, dolt_diff with dots should fail
				User:        "tester",
				Host:        "localhost",
				Query:       "SELECT * FROM dolt_diff('main~...main', 'test');",
				ExpectedErr: sql.ErrDatabaseAccessDeniedForUser,
			},
			{
				// After revoking access, dolt_diff_stat should fail
				User:        "tester",
				Host:        "localhost",
				Query:       "SELECT * FROM dolt_diff_stat('main~', 'main', 'test');",
				ExpectedErr: sql.ErrDatabaseAccessDeniedForUser,
			},
			{
				// After revoking access, dolt_diff_summary should fail
				User:        "tester",
				Host:        "localhost",
				Query:       "SELECT * FROM dolt_diff_summary('main~', 'main', 'test');",
				ExpectedErr: sql.ErrDatabaseAccessDeniedForUser,
			},
			{
				// After revoking access, dolt_patch should fail
				User:        "tester",
				Host:        "localhost",
				Query:       "SELECT * FROM dolt_patch('main~', 'main', 'test');",
				ExpectedErr: sql.ErrDatabaseAccessDeniedForUser,
			},
			{
				// After revoking access, dolt_log should fail
				User:        "tester",
				Host:        "localhost",
				Query:       "SELECT * FROM dolt_log('main');",
				ExpectedErr: sql.ErrDatabaseAccessDeniedForUser,
			},
			{
				// Grant global access to *.*
				User:     "root",
				Host:     "localhost",
				Query:    "GRANT SELECT ON *.* to tester@localhost;",
				Expected: []sql.Row{{types.NewOkResult(0)}},
			},
			{
				// After granting global access to *.*, dolt_diff should work
				User:     "tester",
				Host:     "localhost",
				Query:    "SELECT COUNT(*) FROM dolt_diff('main~', 'main', 'test');",
				Expected: []sql.Row{{1}},
			},
			{
				// After granting global access to *.*, dolt_diff should work
				User:     "tester",
				Host:     "localhost",
				Query:    "SELECT COUNT(*) FROM dolt_diff('main~...main', 'test');",
				Expected: []sql.Row{{1}},
			},
			{
				// Revoke global access
				User:     "root",
				Host:     "localhost",
				Query:    "REVOKE ALL ON *.* from tester@localhost;",
				Expected: []sql.Row{{types.NewOkResult(0)}},
			},
			{
				// After revoking global access, dolt_diff should fail
				User:        "tester",
				Host:        "localhost",
				Query:       "SELECT * FROM dolt_diff('main~', 'main', 'test');",
				ExpectedErr: sql.ErrDatabaseAccessDeniedForUser,
			},
			{
				// After revoking global access, dolt_diff with dots should fail
				User:        "tester",
				Host:        "localhost",
				Query:       "SELECT * FROM dolt_diff('main~..main', 'test');",
				ExpectedErr: sql.ErrDatabaseAccessDeniedForUser,
			},
		},
	},
}

// HistorySystemTableScriptTests contains working tests for both prepared and non-prepared
var HistorySystemTableScriptTests = []queries.ScriptTest{
	{
		Name: "empty table",
		SetUpScript: []string{
			"create table t (n int, c varchar(20));",
			"call dolt_add('.')",
			"set @Commit1 = '';",
			"call dolt_commit_hash_out(@Commit1, '-am', 'creating table t');",
		},
		Assertions: []queries.ScriptTestAssertion{
			{
				Query:    "select count(*) from DOLT_HISTORY_t;",
				Expected: []sql.Row{{0}},
			},
		},
	},
	{
		Name: "keyless table",
		SetUpScript: []string{
			"create table foo1 (n int, de varchar(20));",
			"insert into foo1 values (1, 'Ein'), (2, 'Zwei'), (3, 'Drei');",
			"call dolt_add('.')",
			"set @Commit1 = '';",
			"call dolt_commit_hash_out(@Commit1, '-am', 'inserting into foo1', '--date', '2022-08-06T12:00:00');",

			"update foo1 set de='Eins' where n=1;",
			"set @Commit2 = '';",
			"call dolt_commit_hash_out(@Commit2, '-am', 'updating data in foo1', '--date', '2022-08-06T12:00:01');",

			"insert into foo1 values (4, 'Vier');",
			"set @Commit3 = '';",
			"call dolt_commit_hash_out(@Commit3, '-am', 'inserting data in foo1', '--date', '2022-08-06T12:00:02');",
		},
		Assertions: []queries.ScriptTestAssertion{
			{
				Query:    "select count(*) from DOLT_HISTORY_foO1;",
				Expected: []sql.Row{{10}},
			},
			{
				Query:    "select n, de from dolt_history_foo1 where commit_hash=@Commit1;",
				Expected: []sql.Row{{1, "Ein"}, {2, "Zwei"}, {3, "Drei"}},
			},
			{
				Query:    "select n, de from dolt_history_Foo1 where commit_hash=@Commit2;",
				Expected: []sql.Row{{1, "Eins"}, {2, "Zwei"}, {3, "Drei"}},
			},
			{
				Query:    "select n, de from dolt_history_foo1 where commit_hash=@Commit3;",
				Expected: []sql.Row{{1, "Eins"}, {2, "Zwei"}, {3, "Drei"}, {4, "Vier"}},
			},
		},
	},
	{
		Name: "primary key table: basic cases",
		SetUpScript: []string{
			"create table t1 (n int primary key, de varchar(20));",
			"call dolt_add('.')",
			"insert into t1 values (1, 'Eins'), (2, 'Zwei'), (3, 'Drei');",
			"set @Commit1 = '';",
			"call dolt_commit_hash_out(@Commit1, '-am', 'inserting into t1', '--date', '2022-08-06T12:00:01');",

			"alter table t1 add column fr varchar(20);",
			"insert into t1 values (4, 'Vier', 'Quatre');",
			"set @Commit2 = '';",
			"call dolt_commit_hash_out(@Commit2, '-am', 'adding column and inserting data in t1', '--date', '2022-08-06T12:00:02');",

			"update t1 set fr='Un' where n=1;",
			"update t1 set fr='Deux' where n=2;",
			"set @Commit3 = '';",
			"call dolt_commit_hash_out(@Commit3, '-am', 'updating data in t1', '--date', '2022-08-06T12:00:03');",

			"update t1 set de=concat(de, ', meine herren') where n>1;",
			"set @Commit4 = '';",
			"call dolt_commit_hash_out(@Commit4, '-am', 'be polite when you address a gentleman', '--date', '2022-08-06T12:00:04');",

			"delete from t1 where n=2;",
			"set @Commit5 = '';",
			"call dolt_commit_hash_out(@Commit5, '-am', 'we don''t need the number 2', '--date', '2022-08-06T12:00:05');",
		},
		Assertions: []queries.ScriptTestAssertion{
			{
				Query:    "select count(*) from Dolt_History_t1;",
				Expected: []sql.Row{{18}},
			},
			{
				Query:    "select n, de, fr from dolt_history_T1 where commit_hash = @Commit1;",
				Expected: []sql.Row{{1, "Eins", nil}, {2, "Zwei", nil}, {3, "Drei", nil}},
			},
			{
				Query:    "select n, de, fr from dolt_history_T1 where commit_hash = @Commit2;",
				Expected: []sql.Row{{1, "Eins", nil}, {2, "Zwei", nil}, {3, "Drei", nil}, {4, "Vier", "Quatre"}},
			},
			{
				Query:    "select n, de, fr from dolt_history_T1 where commit_hash = @Commit3;",
				Expected: []sql.Row{{1, "Eins", "Un"}, {2, "Zwei", "Deux"}, {3, "Drei", nil}, {4, "Vier", "Quatre"}},
			},
			{
				Query: "select n, de, fr from dolt_history_T1 where commit_hash = @Commit4;",
				Expected: []sql.Row{
					{1, "Eins", "Un"},
					{2, "Zwei, meine herren", "Deux"},
					{3, "Drei, meine herren", nil},
					{4, "Vier, meine herren", "Quatre"},
				},
			},
			{
				Query: "select n, de, fr from dolt_history_T1 where commit_hash = @Commit5;",
				Expected: []sql.Row{
					{1, "Eins", "Un"},
					{3, "Drei, meine herren", nil},
					{4, "Vier, meine herren", "Quatre"},
				},
			},
			{
				Query: "select de, fr, commit_hash=@commit1, commit_hash=@commit2, commit_hash=@commit3, commit_hash=@commit4" +
					" from dolt_history_T1 where n=2 order by commit_date",
				Expected: []sql.Row{
					{"Zwei", nil, true, false, false, false},
					{"Zwei", nil, false, true, false, false},
					{"Zwei", "Deux", false, false, true, false},
					{"Zwei, meine herren", "Deux", false, false, false, true},
				},
			},
		},
	},
	{
		Name: "index by primary key",
		SetUpScript: []string{
			"create table t1 (pk int primary key, c int);",
			"call dolt_add('.')",
			"insert into t1 values (1,2), (3,4)",
			"set @Commit1 = '';",
			"call dolt_commit_hash_out(@Commit1, '-am', 'initial table');",
			"insert into t1 values (5,6), (7,8)",
			"set @Commit2 = '';",
			"call dolt_commit_hash_out(@Commit2, '-am', 'two more rows');",
		},
		Assertions: []queries.ScriptTestAssertion{
			{
				Query: "select pk, c, commit_hash = @Commit1, commit_hash = @Commit2 from dolt_history_t1",
				Expected: []sql.Row{
					{1, 2, false, true},
					{3, 4, false, true},
					{5, 6, false, true},
					{7, 8, false, true},
					{1, 2, true, false},
					{3, 4, true, false},
				},
			},
			{
				Query: "select pk, c from dolt_history_t1 order by pk",
				Expected: []sql.Row{
					{1, 2},
					{1, 2},
					{3, 4},
					{3, 4},
					{5, 6},
					{7, 8},
				},
			},
			{
				Query: "select pk, c from dolt_history_t1 order by pk, c",
				Expected: []sql.Row{
					{1, 2},
					{1, 2},
					{3, 4},
					{3, 4},
					{5, 6},
					{7, 8},
				},
			},
			{
				Query: "select pk, c from dolt_history_t1 where pk = 3",
				Expected: []sql.Row{
					{3, 4},
					{3, 4},
				},
			},
			{
				Query: "select pk, c from dolt_history_t1 where pk = 3 and commit_hash = @Commit2",
				Expected: []sql.Row{
					{3, 4},
				},
			},
			{
				Query: "explain select pk, c from dolt_history_t1 where pk = 3",
				Expected: []sql.Row{
					{"Filter"},
					{" ├─ (dolt_history_t1.pk = 3)"},
					{" └─ IndexedTableAccess(dolt_history_t1)"},
					{"     ├─ index: [dolt_history_t1.pk]"},
					{"     ├─ filters: [{[3, 3]}]"},
					{"     └─ columns: [pk c]"},
				},
			},
			{
				Query: "explain select pk, c from dolt_history_t1 where pk = 3 and committer = 'someguy'",
				Expected: []sql.Row{
					{"Project"},
					{" ├─ columns: [dolt_history_t1.pk, dolt_history_t1.c]"},
					{" └─ Filter"},
					{"     ├─ ((dolt_history_t1.pk = 3) AND (dolt_history_t1.committer = 'someguy'))"},
					{"     └─ IndexedTableAccess(dolt_history_t1)"},
					{"         ├─ index: [dolt_history_t1.pk]"},
					{"         ├─ filters: [{[3, 3]}]"},
					{"         └─ columns: [pk c committer]"},
				},
			},
		},
	},
	{
		Name: "adding an index",
		SetUpScript: []string{
			"create table t1 (pk int primary key, c int);",
			"call dolt_add('.')",
			"insert into t1 values (1,2), (3,4)",
			"set @Commit1 = '';",
			"call dolt_commit_hash_out(@Commit1, '-am', 'initial table');",
			"insert into t1 values (5,6), (7,8)",
			"set @Commit2 = '';",
			"call dolt_commit_hash_out(@Commit2, '-am', 'two more rows');",
			"insert into t1 values (9,10), (11,12)",
			"create index t1_c on t1(c)",
			"set @Commit2 = '';",
			"call dolt_commit_hash_out(@Commit2, '-am', 'two more rows and an index');",
		},
		Assertions: []queries.ScriptTestAssertion{
			{
				Query: "select pk, c from dolt_history_t1 order by pk",
				Expected: []sql.Row{
					{1, 2},
					{1, 2},
					{1, 2},
					{3, 4},
					{3, 4},
					{3, 4},
					{5, 6},
					{5, 6},
					{7, 8},
					{7, 8},
					{9, 10},
					{11, 12},
				},
			},
			{
				Query: "select pk, c from dolt_history_t1 where c = 4 order by pk",
				Expected: []sql.Row{
					{3, 4},
					{3, 4},
					{3, 4},
				},
			},
			{
				Query: "select pk, c from dolt_history_t1 where c = 10 order by pk",
				Expected: []sql.Row{
					{9, 10},
				},
			},
			{
				Query: "explain select pk, c from dolt_history_t1 where c = 4",
				Expected: []sql.Row{
					{"Filter"},
					{" ├─ (dolt_history_t1.c = 4)"},
					{" └─ IndexedTableAccess(dolt_history_t1)"},
					{"     ├─ index: [dolt_history_t1.c]"},
					{"     ├─ filters: [{[4, 4]}]"},
					{"     └─ columns: [pk c]"},
				},
			},
			{
				Query: "explain select pk, c from dolt_history_t1 where c = 10 and committer = 'someguy'",
				Expected: []sql.Row{
					{"Project"},
					{" ├─ columns: [dolt_history_t1.pk, dolt_history_t1.c]"},
					{" └─ Filter"},
					{"     ├─ ((dolt_history_t1.c = 10) AND (dolt_history_t1.committer = 'someguy'))"},
					{"     └─ IndexedTableAccess(dolt_history_t1)"},
					{"         ├─ index: [dolt_history_t1.c]"},
					{"         ├─ filters: [{[10, 10]}]"},
					{"         └─ columns: [pk c committer]"},
				},
			},
		},
	},
	{
		Name: "primary key table: non-pk column drops and adds",
		SetUpScript: []string{
			"create table t (pk int primary key, c1 int, c2 varchar(20));",
			"call dolt_add('.')",
			"insert into t values (1, 2, '3'), (4, 5, '6');",
			"set @Commit1 = '';",
			"CALL DOLT_COMMIT_HASH_OUT(@Commit1, '-am', 'creating table t');",

			"alter table t drop column c2;",
			"set @Commit2 = '';",
			"CALL DOLT_COMMIT_HASH_OUT(@Commit2, '-am', 'dropping column c2');",

			"alter table t rename column c1 to c2;",
			"set @Commit3 = '';",
			"CALL DOLT_COMMIT_HASH_OUT(@Commit3, '-am', 'renaming c1 to c2');",
		},
		Assertions: []queries.ScriptTestAssertion{
			{
				Query:    "select count(*) from dolt_history_t;",
				Expected: []sql.Row{{6}},
			},
			{
				// TODO: Instead of just spot checking the non-existence of c1, it would be useful to be able to
				//       assert the full schema of the result set. ScriptTestAssertion doesn't support that currently,
				//       but the code from QueryTest could be ported over to ScriptTestAssertion.
				Query:       "select c1 from dolt_history_t;",
				ExpectedErr: sql.ErrColumnNotFound,
			},
			{
				Query:    "select pk, c2 from dolt_history_t where commit_hash=@Commit1 order by pk;",
				Expected: []sql.Row{{1, nil}, {4, nil}},
			},
			{
				Query:    "select pk, c2 from dolt_history_t where commit_hash=@Commit2 order by pk;",
				Expected: []sql.Row{{1, nil}, {4, nil}},
			},
			{
				Query:    "select pk, c2 from dolt_history_t where commit_hash=@Commit3 order by pk;",
				Expected: []sql.Row{{1, 2}, {4, 5}},
			},
		},
	},
	{
		Name: "primary key table: non-pk column type changes",
		SetUpScript: []string{
			"create table t (pk int primary key, c1 int, c2 varchar(20));",
			"call dolt_add('.')",
			"insert into t values (1, 2, '3'), (4, 5, '6');",
			"set @Commit1 = '';",
			"CALL DOLT_COMMIT_HASH_OUT(@Commit1, '-am', 'creating table t');",
			"alter table t modify column c2 int;",
			"set @Commit2 = '';",
			"CALL DOLT_COMMIT_HASH_OUT(@Commit2, '-am', 'changed type of c2');",
		},
		Assertions: []queries.ScriptTestAssertion{
			{
				Query:    "select count(*) from dolt_history_t;",
				Expected: []sql.Row{{4}},
			},
			// Can't represent the old schema in the current one, so it gets nil valued
			{
				Query:    "select pk, c2 from dolt_history_t where commit_hash=@Commit1 order by pk;",
				Expected: []sql.Row{{1, nil}, {4, nil}},
			},
			{
				Query:    "select pk, c2 from dolt_history_t where commit_hash=@Commit2 order by pk;",
				Expected: []sql.Row{{1, 3}, {4, 6}},
			},
		},
	},
	{
		Name: "primary key table: rename table",
		SetUpScript: []string{
			"create table t (pk int primary key, c1 int, c2 varchar(20));",
			"call dolt_add('.')",
			"insert into t values (1, 2, '3'), (4, 5, '6');",
			"set @Commit1 = '';",
			"CALL DOLT_COMMIT_HASH_OUT(@Commit1, '-am', 'creating table t');",

			"alter table t rename to t2;",
			"call dolt_add('.')",
			"set @Commit2 = '';",
			"CALL DOLT_COMMIT_HASH_OUT(@Commit2, '-am', 'renaming table to t2');",
		},
		Assertions: []queries.ScriptTestAssertion{
			{
				Query:       "select count(*) from dolt_history_t;",
				ExpectedErr: sql.ErrTableNotFound,
			},
			{
				Query:    "select count(*) from dolt_history_T2;",
				Expected: []sql.Row{{2}},
			},
			{
				Query:    "select pk, c1, c2 from dolt_history_t2 where commit_hash != @Commit1;",
				Expected: []sql.Row{{1, 2, "3"}, {4, 5, "6"}},
			},
		},
	},
	{
		Name: "primary key table: delete and recreate table",
		SetUpScript: []string{
			"create table t (pk int primary key, c1 int, c2 varchar(20));",
			"call dolt_add('.')",
			"insert into t values (1, 2, '3'), (4, 5, '6');",
			"set @Commit1 = '';",
			"CALL DOLT_COMMIT_HASH_OUT(@Commit1, '-am', 'creating table t');",

			"drop table t;",
			"set @Commit2 = '';",
			"CALL DOLT_COMMIT_HASH_OUT(@Commit2, '-am', 'dropping table t');",

			"create table t (pk int primary key, c1 int);",
			"call dolt_add('.')",
			"set @Commit3 = '';",
			"CALL DOLT_COMMIT_HASH_OUT(@Commit3, '-am', 'recreating table t');",
		},
		Assertions: []queries.ScriptTestAssertion{
			{
				// TODO: The history system table processes history in parallel and pulls the rows for the
				//       user table at all commits. This means we can't currently detect when a table was dropped
				//       and if a different table with the same name exists at earlier commits, those results will
				//       be included in the history table. It may make more sense to have history scoped only
				//       to the current instance of the table, which would require changing the history system table
				//       to use something like an iterator approach where it goes back sequentially until it detects
				//       the current table doesn't exist any more and then stop.
				Query:    "select count(*) from dolt_history_t;",
				Expected: []sql.Row{{2}},
			},
		},
	},
	{
		Name: "dolt_history table with AS OF",
		SetUpScript: []string{
			"create table t (pk int primary key, c1 int, c2 varchar(20));",
			"call dolt_add('-A');",
			"call dolt_commit('-m', 'creating table t');",
			"insert into t values (1, 2, '3'), (4, 5, '6');",
			"call dolt_commit('-am', 'added values');",
			"insert into t values (11, 22, '3'), (44, 55, '6');",
			"call dolt_commit('-am', 'added values again');",
		},
		Assertions: []queries.ScriptTestAssertion{
			{
				Query:    "select count(*) from dolt_history_t;",
				Expected: []sql.Row{{6}}, // 2 + 4
			},
			{
				Query:    "select count(*) from dolt_history_t AS OF 'head^';",
				Expected: []sql.Row{{2}}, // 2
			},
			{
				Query: "select message from dolt_log;",
				Expected: []sql.Row{
					{"added values again"},
					{"added values"},
					{"creating table t"},
					{"checkpoint enginetest database mydb"},
					{"Initialize data repository"},
				},
			},
		},
	},
	{
		SkipPrepared: true,
		Name:         "dolt_history table with AS OF",
		SetUpScript: []string{
			"create table t (pk int primary key, c1 int, c2 varchar(20));",
			"call dolt_add('-A');",
			"call dolt_commit('-m', 'creating table t');",
			"insert into t values (1, 2, '3'), (4, 5, '6');",
			"call dolt_commit('-am', 'added values');",
			"insert into t values (11, 22, '3'), (44, 55, '6');",
			"call dolt_commit('-am', 'added values again');",
		},
		Assertions: []queries.ScriptTestAssertion{
			{
				Query: "select message from dolt_log AS OF 'head^';",
				Expected: []sql.Row{
					{"added values"},
					{"creating table t"},
					{"checkpoint enginetest database mydb"},
					{"Initialize data repository"},
				},
			},
		},
	},
	{
		Name: "dolt_history table with enums",
		SetUpScript: []string{
			"create table t (pk int primary key, c1 enum('foo','bar'));",
			"call dolt_add('-A');",
			"call dolt_commit('-m', 'creating table t');",
			"insert into t values (1, 'foo');",
			"call dolt_commit('-am', 'added values');",
		},
		Assertions: []queries.ScriptTestAssertion{
			{
				Query: "select c1 from dolt_history_t;",
				Expected: []sql.Row{
					{uint64(1)},
				},
			},
		},
	},
	{
		Name: "dolt_history table index lookup",
		SetUpScript: []string{
			"create table yx (y int, x int primary key);",
			"call dolt_add('.');",
			"call dolt_commit('-m', 'creating table');",
			"insert into yx values (0, 1);",
			"call dolt_commit('-am', 'add data');",
			"insert into yx values (2, 3);",
			"call dolt_commit('-am', 'add data');",
			"insert into yx values (4, 5);",
			"call dolt_commit('-am', 'add data');",
		},
		Assertions: []queries.ScriptTestAssertion{
			{
				Query: "select count(x) from dolt_history_yx where x = 1;",
				Expected: []sql.Row{
					{3},
				},
			},
		},
	},
	{
		Name: "dolt_history table filter correctness",
		SetUpScript: []string{
			"create table xy (x int primary key, y int);",
			"call dolt_add('.');",
			"call dolt_commit('-m', 'creating table');",
			"insert into xy values (0, 1);",
			"call dolt_commit('-am', 'add data');",
			"insert into xy values (2, 3);",
			"call dolt_commit('-am', 'add data');",
			"insert into xy values (4, 5);",
			"call dolt_commit('-am', 'add data');",
		},
		Assertions: []queries.ScriptTestAssertion{
			{
				Query: "select count(*) from dolt_history_xy where commit_hash = (select dolt_log.commit_hash from dolt_log limit 1 offset 1)",
				Expected: []sql.Row{
					{2},
				},
			},
		},
	},
}

// BrokenHistorySystemTableScriptTests contains tests that work for non-prepared, but don't work
// for prepared queries.
var BrokenHistorySystemTableScriptTests = []queries.ScriptTest{
	{
		Name: "dolt_history table with AS OF",
		SetUpScript: []string{
			"create table t (pk int primary key, c1 int, c2 varchar(20));",
			"call dolt_add('-A');",
			"call dolt_commit('-m', 'creating table t');",
			"insert into t values (1, 2, '3'), (4, 5, '6');",
			"call dolt_commit('-am', 'added values');",
			"insert into t values (11, 22, '3'), (44, 55, '6');",
			"call dolt_commit('-am', 'added values again');",
		},
		Assertions: []queries.ScriptTestAssertion{
			{
				Query: "select message from dolt_log AS OF 'head^';",
				Expected: []sql.Row{
					{"added values"},
					{"creating table t"},
					{"checkpoint enginetest database mydb"},
					{"Initialize data repository"},
				},
			},
		},
	},
}

var DoltBranchScripts = []queries.ScriptTest{
	{
		Name: "Create branches from HEAD with dolt_branch procedure",
		Assertions: []queries.ScriptTestAssertion{
			{
				Query:    "CALL DOLT_BRANCH('myNewBranch1')",
				Expected: []sql.Row{{0}},
			},
			{
				Query:    "SELECT COUNT(*) FROM DOLT_BRANCHES WHERE NAME='myNewBranch1';",
				Expected: []sql.Row{{1}},
			},
			{
				// Trying to recreate that branch fails without the force flag
				Query:          "CALL DOLT_BRANCH('myNewBranch1')",
				ExpectedErrStr: "fatal: A branch named 'myNewBranch1' already exists.",
			},
			{
				Query:    "CALL DOLT_BRANCH('-f', 'myNewBranch1')",
				Expected: []sql.Row{{0}},
			},
		},
	},
	{
		Name: "Rename branches with dolt_branch procedure",
		Assertions: []queries.ScriptTestAssertion{
			{
				Query:    "CALL DOLT_BRANCH('myNewBranch1')",
				Expected: []sql.Row{{0}},
			},
			{
				Query:    "CALL DOLT_BRANCH('myNewBranch2')",
				Expected: []sql.Row{{0}},
			},
			{
				// Renaming to an existing name fails without the force flag
				Query:          "CALL DOLT_BRANCH('-m', 'myNewBranch1', 'myNewBranch2')",
				ExpectedErrStr: "already exists",
			},
			{
				Query:    "CALL DOLT_BRANCH('-mf', 'myNewBranch1', 'myNewBranch2')",
				Expected: []sql.Row{{0}},
			},
			{
				Query:    "CALL DOLT_BRANCH('-m', 'myNewBranch2', 'myNewBranch3')",
				Expected: []sql.Row{{0}},
			},
		},
	},
	{
		Name: "Copy branches from other branches using dolt_branch procedure",
		SetUpScript: []string{
			"CALL DOLT_BRANCH('myNewBranch1')",
		},
		Assertions: []queries.ScriptTestAssertion{
			{
				Query:          "CALL DOLT_BRANCH('-c')",
				ExpectedErrStr: "error: invalid usage",
			},
			{
				Query:          "CALL DOLT_BRANCH('-c', 'myNewBranch1')",
				ExpectedErrStr: "error: invalid usage",
			},
			{
				Query:          "CALL DOLT_BRANCH('-c', 'myNewBranch2')",
				ExpectedErrStr: "error: invalid usage",
			},
			{
				Query:          "CALL DOLT_BRANCH('-c', '', '')",
				ExpectedErrStr: "error: cannot branch empty string",
			},
			{
				Query:    "CALL DOLT_BRANCH('-c', 'myNewBranch1', 'myNewBranch2')",
				Expected: []sql.Row{{0}},
			},
			{
				Query:    "SELECT COUNT(*) FROM DOLT_BRANCHES WHERE NAME='myNewBranch2';",
				Expected: []sql.Row{{1}},
			},
			{
				Query:          "CALL DOLT_BRANCH('-c', 'myNewBranch1', 'myNewBranch2')",
				ExpectedErrStr: "fatal: A branch named 'myNewBranch2' already exists.",
			},
			{
				Query:    "CALL DOLT_BRANCH('-cf', 'myNewBranch1', 'myNewBranch2')",
				Expected: []sql.Row{{0}},
			},
		},
	},
	{
		Name: "Delete branches with dolt_branch procedure",
		SetUpScript: []string{
			"CALL DOLT_BRANCH('myNewBranch1')",
			"CALL DOLT_BRANCH('myNewBranch2')",
			"CALL DOLT_BRANCH('myNewBranch3')",
			"CALL DOLT_BRANCH('myNewBranchWithCommit')",
			"CALL DOLT_CHECKOUT('myNewBranchWithCommit')",
			"CALL DOLT_COMMIT('--allow-empty', '-am', 'empty commit')",
			"CALL DOLT_CHECKOUT('main')",
		},
		Assertions: []queries.ScriptTestAssertion{
			{
				Query:          "CALL DOLT_BRANCH('-d')",
				ExpectedErrStr: "error: invalid usage",
			},
			{
				Query:          "CALL DOLT_BRANCH('-d', '')",
				ExpectedErrStr: "error: cannot branch empty string",
			},
			{
				Query:          "CALL DOLT_BRANCH('-d', 'branchDoesNotExist')",
				ExpectedErrStr: "branch not found",
			},
			{
				Query:    "CALL DOLT_BRANCH('-d', 'myNewBranch1')",
				Expected: []sql.Row{{0}},
			},
			{
				Query:    "SELECT COUNT(*) FROM DOLT_BRANCHES WHERE NAME='myNewBranch1'",
				Expected: []sql.Row{{0}},
			},
			{
				Query:    "CALL DOLT_BRANCH('-d', 'myNewBranch2', 'myNewBranch3')",
				Expected: []sql.Row{{0}},
			},
			{
				// Trying to delete a branch with unpushed changes fails without force option
				Query:          "CALL DOLT_BRANCH('-d', 'myNewBranchWithCommit')",
				ExpectedErrStr: "branch is not fully merged",
			},
			{
				Query:    "CALL DOLT_BRANCH('-df', 'myNewBranchWithCommit')",
				Expected: []sql.Row{{0}},
			},
		},
	},
	{
		Name: "Create branch from startpoint",
		SetUpScript: []string{
			"create table a (x int)",
			"call dolt_add('.')",
			"SET @commit1 = '';",
			"CALL DOLT_COMMIT_HASH_OUT(@commit1, '-am', 'add table a');",
		},
		Assertions: []queries.ScriptTestAssertion{
			{
				Query:    "show tables",
				Expected: []sql.Row{{"a"}, {"myview"}},
			},
			{
				Query:    "CALL DOLT_CHECKOUT('-b', 'newBranch', 'head~1')",
				Expected: []sql.Row{{0}},
			},
			{
				Query:    "show tables",
				Expected: []sql.Row{{"myview"}},
			},
			{
				Query:    "CALL DOLT_CHECKOUT('-b', 'newBranch2', @commit1)",
				Expected: []sql.Row{{0}},
			},
			{
				Query:    "show tables",
				Expected: []sql.Row{{"a"}, {"myview"}},
			},
			{
				Query:          "CALL DOLT_CHECKOUT('-b', 'otherBranch', 'unknownCommit')",
				ExpectedErrStr: "fatal: 'unknownCommit' is not a commit and a branch 'otherBranch' cannot be created from it",
			},
		},
	},
}

var DoltReset = []queries.ScriptTest{
	{
		Name: "CALL DOLT_RESET('--hard') should reset the merge state after uncommitted merge",
		SetUpScript: []string{
			"CREATE TABLE test1 (pk int NOT NULL, c1 int, c2 int, PRIMARY KEY (pk));",
			"CALL DOLT_ADD('.')",
			"INSERT INTO test1 values (0,1,1);",
			"CALL DOLT_COMMIT('-am', 'added table')",

			"CALL DOLT_CHECKOUT('-b', 'merge_branch');",
			"UPDATE test1 set c1 = 2;",
			"CALL DOLT_COMMIT('-am', 'update pk 0 = 2,1 to test1');",

			"CALL DOLT_CHECKOUT('main');",
			"UPDATE test1 set c2 = 2;",
			"CALL DOLT_COMMIT('-am', 'update pk 0 = 1,2 to test1');",

			"CALL DOLT_MERGE('merge_branch');",

			"CALL DOLT_RESET('--hard');",
		},
		Assertions: []queries.ScriptTestAssertion{
			{
				Query:          "CALL DOLT_MERGE('--abort')",
				ExpectedErrStr: "fatal: There is no merge to abort",
			},
		},
	},
	{
		Name: "CALL DOLT_RESET('--hard') should reset the merge state after conflicting merge",
		SetUpScript: []string{
			"SET dolt_allow_commit_conflicts = on",
			"CREATE TABLE test1 (pk int NOT NULL, c1 int, c2 int, PRIMARY KEY (pk));",
			"CALL DOLT_ADD('.')",
			"INSERT INTO test1 values (0,1,1);",
			"CALL DOLT_COMMIT('-am', 'added table')",

			"CALL DOLT_CHECKOUT('-b', 'merge_branch');",
			"UPDATE test1 set c1 = 2, c2 = 2;",
			"CALL DOLT_COMMIT('-am', 'update pk 0 = 2,2 to test1');",

			"CALL DOLT_CHECKOUT('main');",
			"UPDATE test1 set c1 = 3, c2 = 3;",
			"CALL DOLT_COMMIT('-am', 'update pk 0 = 3,3 to test1');",

			"CALL DOLT_MERGE('merge_branch');",
			"CALL DOLT_RESET('--hard');",
		},
		Assertions: []queries.ScriptTestAssertion{
			{
				Query:          "CALL DOLT_MERGE('--abort')",
				ExpectedErrStr: "fatal: There is no merge to abort",
			},
		},
	},
}

func gcSetup() []string {
	queries := []string{
		"create table t (pk int primary key);",
		"call dolt_commit('-Am', 'create table');",
	}
	for i := 0; i < 250; i++ {
		queries = append(
			queries,
			fmt.Sprintf("INSERT INTO t VALUES (%d);", i),
			fmt.Sprintf("CALL DOLT_COMMIT('-am', 'added pk %d')", i),
		)
	}
	return queries
}

var DoltGC = []queries.ScriptTest{
	{
		Name:        "base case: gc",
		SetUpScript: gcSetup(),
		Assertions: []queries.ScriptTestAssertion{
			{
				Query:          "CALL DOLT_GC(null);",
				ExpectedErrStr: "error: invalid usage",
			},
			{
				Query:          "CALL DOLT_GC('bad', '--shallow');",
				ExpectedErrStr: "error: invalid usage",
			},
			{
				Query:    "CALL DOLT_GC('--shallow');",
				Expected: []sql.Row{{1}},
			},
			{
				Query:    "CALL DOLT_GC();",
				Expected: []sql.Row{{1}},
			},
			{
				Query:          "CALL DOLT_GC();",
				ExpectedErrStr: "no changes since last gc",
			},
		},
	},
}

var LogTableFunctionScriptTests = []queries.ScriptTest{
	{
		Name: "invalid arguments",
		SetUpScript: []string{
			"create table t (pk int primary key, c1 varchar(20), c2 varchar(20));",
			"call dolt_add('.')",
			"set @Commit1 = '';",
			"call dolt_commit_hash_out(@Commit1, '-am', 'creating table t');",

			"insert into t values(1, 'one', 'two'), (2, 'two', 'three');",
			"set @Commit2 = '';",
			"call dolt_commit_hash_out(@Commit2, '-am', 'inserting into t');",
		},
		Assertions: []queries.ScriptTestAssertion{
			{
				Query:       "SELECT * from dolt_log(@Commit1, @Commit2, 't');",
				ExpectedErr: sql.ErrInvalidArgumentNumber,
			},
			{
				Query:       "SELECT * from dolt_log(null);",
				ExpectedErr: sql.ErrInvalidArgumentDetails,
			},
			{
				Query:       "SELECT * from dolt_log(null, null);",
				ExpectedErr: sql.ErrInvalidArgumentDetails,
			},
			{
				Query:       "SELECT * from dolt_log(null, '--not', null);",
				ExpectedErr: sql.ErrInvalidArgumentDetails,
			},
			{
				Query:       "SELECT * from dolt_log(@Commit1, '--not', null);",
				ExpectedErr: sql.ErrInvalidArgumentDetails,
			},
			{
				Query:       "SELECT * from dolt_log(@Commit1, '--min-parents', null);",
				ExpectedErr: sql.ErrInvalidArgumentDetails,
			},
			{
				Query:       "SELECT * from dolt_log(@Commit1, '--min-parents', 123);",
				ExpectedErr: sql.ErrInvalidArgumentDetails,
			},
			{
				Query:       "SELECT * from dolt_log(123, @Commit1);",
				ExpectedErr: sql.ErrInvalidArgumentDetails,
			},
			{
				Query:       "SELECT * from dolt_log(@Commit1, 123);",
				ExpectedErr: sql.ErrInvalidArgumentDetails,
			},
			{
				Query:       "SELECT * from dolt_log(@Commit1, '--not', 123);",
				ExpectedErr: sql.ErrInvalidArgumentDetails,
			},
			{
				Query:       "SELECT * from dolt_log('main..branch1', @Commit1);",
				ExpectedErr: sql.ErrInvalidArgumentDetails,
			},
			{
				Query:       "SELECT * from dolt_log('^main..branch1');",
				ExpectedErr: sql.ErrInvalidArgumentDetails,
			},
			{
				Query:       "SELECT * from dolt_log('^main...branch1');",
				ExpectedErr: sql.ErrInvalidArgumentDetails,
			},
			{
				Query:       "SELECT * from dolt_log(@Commit1, 'main..branch1');",
				ExpectedErr: sql.ErrInvalidArgumentDetails,
			},
			{
				Query:       "SELECT * from dolt_log(@Commit1, 'main...branch1');",
				ExpectedErr: sql.ErrInvalidArgumentDetails,
			},
			{
				Query:       "SELECT * from dolt_log('main..branch1', '--not', @Commit1);",
				ExpectedErr: sql.ErrInvalidArgumentDetails,
			},
			{
				Query:       "SELECT * from dolt_log('main...branch1', '--not', @Commit1);",
				ExpectedErr: sql.ErrInvalidArgumentDetails,
			},
			{
				Query:       "SELECT * from dolt_log('^main', '--not', @Commit1);",
				ExpectedErr: sql.ErrInvalidArgumentDetails,
			},
			{
				Query:       "SELECT * from dolt_log('main', '--not', '^branch1');",
				ExpectedErr: sql.ErrInvalidArgumentDetails,
			},
			{
				Query:       "SELECT * from dolt_log('main', '--not', 'main..branch1');",
				ExpectedErr: sql.ErrInvalidArgumentDetails,
			},
			{
				Query:       "SELECT * from dolt_log('^main', @Commit2, '--not', @Commit1);",
				ExpectedErr: sql.ErrInvalidArgumentDetails,
			},
			{
				Query:       "SELECT * from dolt_log(@Commit1, @Commit2);",
				ExpectedErr: sql.ErrInvalidArgumentDetails,
			},
			{
				Query:       "SELECT * from dolt_log('^main', '^branch1');",
				ExpectedErr: sql.ErrInvalidArgumentDetails,
			},
			{
				Query:       "SELECT * from dolt_log('^main');",
				ExpectedErr: sql.ErrInvalidArgumentDetails,
			},
			{
				Query:          "SELECT * from dolt_log('fake-branch');",
				ExpectedErrStr: "branch not found: fake-branch",
			},
			{
				Query:          "SELECT * from dolt_log('^fake-branch', 'main');",
				ExpectedErrStr: "branch not found: fake-branch",
			},
			{
				Query:          "SELECT * from dolt_log('fake-branch', '^main');",
				ExpectedErrStr: "branch not found: fake-branch",
			},
			{
				Query:          "SELECT * from dolt_log('main..fake-branch');",
				ExpectedErrStr: "branch not found: fake-branch",
			},
			{
				Query:          "SELECT * from dolt_log('main', '--not', 'fake-branch');",
				ExpectedErrStr: "branch not found: fake-branch",
			},
			{
				Query:       "SELECT * from dolt_log(concat('fake', '-', 'branch'));",
				ExpectedErr: sqle.ErrInvalidNonLiteralArgument,
			},
			{
				Query:       "SELECT * from dolt_log(hashof('main'));",
				ExpectedErr: sqle.ErrInvalidNonLiteralArgument,
			},
			{
				Query:       "SELECT * from dolt_log(@Commit3, '--not', hashof('main'));",
				ExpectedErr: sqle.ErrInvalidNonLiteralArgument,
			},
			{
				Query:       "SELECT * from dolt_log(@Commit1, LOWER(@Commit2));",
				ExpectedErr: sqle.ErrInvalidNonLiteralArgument,
			},
			{
				Query:          "SELECT parents from dolt_log();",
				ExpectedErrStr: `column "parents" could not be found in any table in scope`,
			},
			{
				Query:       "SELECT * from dolt_log('--decorate', 'invalid');",
				ExpectedErr: sql.ErrInvalidArgumentDetails,
			},
			{
				Query:       "SELECT * from dolt_log('--decorate', 123);",
				ExpectedErr: sql.ErrInvalidArgumentDetails,
			},
			{
				Query:       "SELECT * from dolt_log('--decorate', null);",
				ExpectedErr: sql.ErrInvalidArgumentDetails,
			},
			{
				Query:          "SELECT refs from dolt_log();",
				ExpectedErrStr: `column "refs" could not be found in any table in scope`,
			},
			{
				Query:          "SELECT refs from dolt_log('--decorate', 'auto');",
				ExpectedErrStr: `column "refs" could not be found in any table in scope`,
			},
			{
				Query:          "SELECT refs from dolt_log('--decorate', 'no');",
				ExpectedErrStr: `column "refs" could not be found in any table in scope`,
			},
		},
	},
	{
		Name: "basic case with one revision",
		SetUpScript: []string{
			"create table t (pk int primary key, c1 varchar(20), c2 varchar(20));",
			"call dolt_add('.')",
			"set @Commit1 = '';",
			"call dolt_commit_hash_out(@Commit1, '-am', 'creating table t');",

			"insert into t values(1, 'one', 'two'), (2, 'two', 'three');",
			"set @Commit2 = '';",
			"call dolt_commit_hash_out(@Commit2, '-am', 'inserting into t');",

			"call dolt_checkout('-b', 'new-branch')",
			"insert into t values (3, 'three', 'four');",
			"set @Commit3 = '';",
			"call dolt_commit_hash_out(@Commit3, '-am', 'inserting into t again');",
			"call dolt_checkout('main')",
		},
		Assertions: []queries.ScriptTestAssertion{
			{
				Query:    "SELECT count(*) from dolt_log();",
				Expected: []sql.Row{{4}},
			},
			{
				Query:    "SELECT count(*) from dolt_log('main');",
				Expected: []sql.Row{{4}},
			},
			{
				Query:    "SELECT count(*) from dolt_log(@Commit1);",
				Expected: []sql.Row{{3}},
			},
			{
				Query:    "SELECT count(*) from dolt_log(@Commit2);",
				Expected: []sql.Row{{4}},
			},
			{
				Query:    "SELECT count(*) from dolt_log(@Commit3);",
				Expected: []sql.Row{{5}},
			},
			{
				Query:    "SELECT count(*) from dolt_log('new-branch');",
				Expected: []sql.Row{{5}},
			},
			{
				Query:    "SELECT count(*) from dolt_log('main^');",
				Expected: []sql.Row{{3}},
			},
			{
				Query:    "SELECT count(*)	 from dolt_log('main') join dolt_diff(@Commit1, @Commit2, 't') where commit_hash = to_commit;",
				Expected: []sql.Row{{2}},
			},
		},
	},
	{
		Name: "basic case with more than one revision or revision range",
		SetUpScript: []string{
			"create table t (pk int primary key, c1 varchar(20), c2 varchar(20));",
			"call dolt_add('.');",
			"set @Commit1 = '';",
			"call dolt_commit_hash_out(@Commit1, '-am', 'creating table t');",

			"insert into t values(1, 'one', 'two'), (2, 'two', 'three');",
			"set @Commit2 = '';",
			"call dolt_commit_hash_out(@Commit2, '-am', 'inserting into t 2');",

			"call dolt_checkout('-b', 'new-branch');",
			"insert into t values (3, 'three', 'four');",
			"set @Commit3 = '';",
			"call dolt_commit_hash_out(@Commit3, '-am', 'inserting into t 3');",
			"insert into t values (4, 'four', 'five');",
			"set @Commit4 = '';",
			"call dolt_commit_hash_out(@Commit4, '-am', 'inserting into t 4');",

			"call dolt_checkout('main');",
			"insert into t values (5, 'five', 'six');",
			"set @Commit5 = '';",
			"call dolt_commit_hash_out(@Commit5, '-am', 'inserting into t 5');",
		},
		/* Commit graph:
		          3 - 4 (new-branch)
		         /
		0 - 1 - 2 - 5 (main)
		*/
		Assertions: []queries.ScriptTestAssertion{
			{
				Query:    "SELECT count(*) from dolt_log('^main', 'new-branch');",
				Expected: []sql.Row{{2}}, // 4, 3
			},
			{
				Query:    "SELECT count(*) from dolt_log('main..new-branch');",
				Expected: []sql.Row{{2}}, // 4, 3
			},
			{
				Query:    "SELECT count(*) from dolt_log('main...new-branch');",
				Expected: []sql.Row{{3}}, // 5, 4, 3
			},
			{
				Query:    "SELECT count(*) from dolt_log('new-branch', '--not', 'main');",
				Expected: []sql.Row{{2}}, // 4, 3
			},
			{
				Query:    "SELECT count(*) from dolt_log('new-branch', '^main');",
				Expected: []sql.Row{{2}}, // 4, 3
			},
			{
				Query:    "SELECT count(*) from dolt_log('^new-branch', 'main');",
				Expected: []sql.Row{{1}}, // 5
			},
			{
				Query:    "SELECT count(*) from dolt_log('main', '--not', 'new-branch');",
				Expected: []sql.Row{{1}}, // 5
			},
			{
				Query:    "SELECT count(*) from dolt_log('^main', 'main');",
				Expected: []sql.Row{{0}},
			},
			{
				Query:    "SELECT count(*) from dolt_log('main..main');",
				Expected: []sql.Row{{0}},
			},
			{
				Query:    "SELECT count(*) from dolt_log('main...main');",
				Expected: []sql.Row{{0}},
			},
			{
				Query:    "SELECT count(*) from dolt_log('main', '--not', 'main');",
				Expected: []sql.Row{{0}},
			},
			{
				Query:    "SELECT count(*) from dolt_log('^main~', 'main');",
				Expected: []sql.Row{{1}},
			},
			{
				Query:    "SELECT count(*) from dolt_log('^main^', 'main');",
				Expected: []sql.Row{{1}}, // 5
			},
			{
				Query:    "SELECT count(*) from dolt_log('^main', 'main^');",
				Expected: []sql.Row{{0}},
			},
			{
				Query:    "SELECT count(*) from dolt_log('^main', @Commit3);",
				Expected: []sql.Row{{1}},
			},
			{
				Query:    "SELECT count(*) from dolt_log('^new-branch', @Commit5);",
				Expected: []sql.Row{{1}}, // 5
			},
			{
				Query:    "SELECT count(*) from dolt_log(@Commit3, '--not', @Commit2);",
				Expected: []sql.Row{{1}}, // 3
			},
			{
				Query:    "SELECT count(*) from dolt_log(@Commit4, '--not', @Commit2);",
				Expected: []sql.Row{{2}}, // 4, 3
			},
		},
	},
	{
		Name: "basic case with one revision, row content",
		SetUpScript: []string{
			"create table t (pk int primary key, c1 varchar(20), c2 varchar(20));",
			"call dolt_add('.')",
			"set @Commit1 = '';",
			"call dolt_commit_hash_out(@Commit1, '-am', 'creating table t');",

			"insert into t values(1, 'one', 'two'), (2, 'two', 'three');",
			"set @Commit2 = '';",
			"call dolt_commit_hash_out(@Commit2, '-am', 'inserting into t');",

			"call dolt_checkout('-b', 'new-branch')",
			"insert into t values (3, 'three', 'four');",
			"set @Commit3 = '';",
			"call dolt_commit_hash_out(@Commit3, '-am', 'inserting into t again', '--author', 'John Doe <johndoe@example.com>');",
			"call dolt_checkout('main')",
		},
		Assertions: []queries.ScriptTestAssertion{
			{
				Query: "SELECT commit_hash = @Commit2, commit_hash = @Commit1, committer, email, message from dolt_log();",
				Expected: []sql.Row{
					{true, false, "billy bob", "bigbillieb@fake.horse", "inserting into t"},
					{false, true, "billy bob", "bigbillieb@fake.horse", "creating table t"},
					{false, false, "billy bob", "bigbillieb@fake.horse", "checkpoint enginetest database mydb"},
					{false, false, "billy bob", "bigbillieb@fake.horse", "Initialize data repository"},
				},
			},
			{
				Query:    "SELECT commit_hash = @Commit2, committer, email, message from dolt_log('main') limit 1;",
				Expected: []sql.Row{{true, "billy bob", "bigbillieb@fake.horse", "inserting into t"}},
			},
			{
				Query:    "SELECT commit_hash = @Commit3, committer, email, message from dolt_log('new-branch') limit 1;",
				Expected: []sql.Row{{true, "John Doe", "johndoe@example.com", "inserting into t again"}},
			},
			{
				Query:    "SELECT commit_hash = @Commit1, committer, email, message from dolt_log(@Commit1) limit 1;",
				Expected: []sql.Row{{true, "billy bob", "bigbillieb@fake.horse", "creating table t"}},
			},
		},
	},
	{
		Name: "basic case with more than one revision or revision range, row content",
		SetUpScript: []string{
			"create table t (pk int primary key, c1 varchar(20), c2 varchar(20));",
			"call dolt_add('.');",
			"set @Commit1 = '';",
			"call dolt_commit_hash_out(@Commit1, '-am', 'creating table t');",

			"insert into t values(1, 'one', 'two'), (2, 'two', 'three');",
			"set @Commit2 = '';",
			"call dolt_commit_hash_out(@Commit2, '-am', 'inserting into t 2');",

			"call dolt_checkout('-b', 'new-branch');",
			"insert into t values (3, 'three', 'four');",
			"set @Commit3 = '';",
			"call dolt_commit_hash_out(@Commit3, '-am', 'inserting into t 3', '--author', 'John Doe <johndoe@example.com>');",
			"insert into t values (4, 'four', 'five');",
			"set @Commit4 = '';",
			"call dolt_commit_hash_out(@Commit4, '-am', 'inserting into t 4', '--author', 'John Doe <johndoe@example.com>');",

			"call dolt_checkout('main');",
			"insert into t values (5, 'five', 'six');",
			"set @Commit5 = '';",
			"call dolt_commit_hash_out(@Commit5, '-am', 'inserting into t 5');",
		},
		/* Commit graph:
		          3 - 4 (new-branch)
		         /
		0 - 1 - 2 - 5 (main)
		*/
		Assertions: []queries.ScriptTestAssertion{
			{
				Query: "SELECT commit_hash = @Commit4, commit_hash = @Commit3, committer, email, message from dolt_log('^main', 'new-branch');",
				Expected: []sql.Row{
					{true, false, "John Doe", "johndoe@example.com", "inserting into t 4"},
					{false, true, "John Doe", "johndoe@example.com", "inserting into t 3"},
				},
			},
			{
				Query: "SELECT commit_hash = @Commit4, commit_hash = @Commit3, committer, email, message from dolt_log('main..new-branch');",
				Expected: []sql.Row{
					{true, false, "John Doe", "johndoe@example.com", "inserting into t 4"},
					{false, true, "John Doe", "johndoe@example.com", "inserting into t 3"},
				},
			},
			{
				Query: "SELECT commit_hash = @Commit5, commit_hash = @Commit4, commit_hash = @Commit3, committer, email, message from dolt_log('main...new-branch');",
				Expected: []sql.Row{
					{true, false, false, "billy bob", "bigbillieb@fake.horse", "inserting into t 5"},
					{false, true, false, "John Doe", "johndoe@example.com", "inserting into t 4"},
					{false, false, true, "John Doe", "johndoe@example.com", "inserting into t 3"},
				},
			},
			{
				Query: "SELECT commit_hash = @Commit4, commit_hash = @Commit3, committer, email, message from dolt_log('new-branch', '--not', 'main');",
				Expected: []sql.Row{
					{true, false, "John Doe", "johndoe@example.com", "inserting into t 4"},
					{false, true, "John Doe", "johndoe@example.com", "inserting into t 3"},
				},
			},
			{
				Query: "SELECT commit_hash = @Commit4, commit_hash = @Commit3, committer, email, message from dolt_log('new-branch', '^main');",
				Expected: []sql.Row{
					{true, false, "John Doe", "johndoe@example.com", "inserting into t 4"},
					{false, true, "John Doe", "johndoe@example.com", "inserting into t 3"},
				},
			},
			{
				Query:    "SELECT commit_hash = @Commit5, committer, email, message from dolt_log('^new-branch', 'main');",
				Expected: []sql.Row{{true, "billy bob", "bigbillieb@fake.horse", "inserting into t 5"}},
			},
			{
				Query:    "SELECT * from dolt_log('^main', 'main');",
				Expected: []sql.Row{},
			},
			{
				Query:    "SELECT commit_hash = @Commit5, committer, email, message from dolt_log('^main~', 'main');",
				Expected: []sql.Row{{true, "billy bob", "bigbillieb@fake.horse", "inserting into t 5"}},
			},
			{
				Query:    "SELECT commit_hash = @Commit5, committer, email, message from dolt_log( 'main', '--not', 'main~');",
				Expected: []sql.Row{{true, "billy bob", "bigbillieb@fake.horse", "inserting into t 5"}},
			},
			{
				Query:    "SELECT commit_hash = @Commit3, committer, email, message from dolt_log('^main', @Commit3);",
				Expected: []sql.Row{{true, "John Doe", "johndoe@example.com", "inserting into t 3"}},
			},
			{
				Query:    "SELECT commit_hash = @Commit3, committer, email, message from dolt_log(@Commit3, '--not', @Commit2);",
				Expected: []sql.Row{{true, "John Doe", "johndoe@example.com", "inserting into t 3"}},
			},
			{
				Query:    "SELECT commit_hash = @Commit5, committer, email, message from dolt_log('^new-branch', @Commit5);",
				Expected: []sql.Row{{true, "billy bob", "bigbillieb@fake.horse", "inserting into t 5"}},
			},
			{
				Query:    "SELECT commit_hash = @Commit5, committer, email, message from dolt_log(@Commit5, '--not', @Commit4);",
				Expected: []sql.Row{{true, "billy bob", "bigbillieb@fake.horse", "inserting into t 5"}},
			},
		},
	},
	//TODO: figure out how we were returning a commit from the function
	/*{
		Name: "min parents, merges, show parents, decorate",
		SetUpScript: []string{
			"create table t (pk int primary key, c1 int);",
			"call dolt_add('.')",
			"set @Commit1 = '';",
			"call dolt_commit_hash_out(@Commit1, '-am', 'creating table t');",

			"call dolt_checkout('-b', 'branch1')",
			"insert into t values(0,0);",
			"set @Commit2 = '';",
		"call dolt_commit_hash_out(@Commit2, '-am', 'inserting 0,0');",

			"call dolt_checkout('main')",
			"call dolt_checkout('-b', 'branch2')",
			"insert into t values(1,1);",
			"set @Commit3 = '';",
		"call dolt_commit_hash_out(@Commit3, '-am', 'inserting 1,1');",

			"call dolt_checkout('main')",
			"call dolt_merge('branch1')",               // fast-forward merge
			"set @MergeCommit = dolt_merge('branch2')", // actual merge with commit
			"call dolt_tag('v1')",
		},
		Assertions: []queries.ScriptTestAssertion{
			{
				Query:    "SELECT commit_hash = @MergeCommit, committer, email, message from dolt_log('--merges');",
				Expected: []sql.Row{{true, "billy bob", "bigbillieb@fake.horse", "Merge branch 'branch2' into main"}},
			},
			{
				Query:    "SELECT commit_hash = @MergeCommit, committer, email, message from dolt_log('--min-parents', '2');",
				Expected: []sql.Row{{true, "billy bob", "bigbillieb@fake.horse", "Merge branch 'branch2' into main"}},
			},
			{
				Query:    "SELECT commit_hash = @MergeCommit, committer, email, message from dolt_log('main', '--min-parents', '2');",
				Expected: []sql.Row{{true, "billy bob", "bigbillieb@fake.horse", "Merge branch 'branch2' into main"}},
			},
			{
				Query:    "SELECT count(*) from dolt_log('main');",
				Expected: []sql.Row{{6}},
			},
			{
				Query:    "SELECT count(*) from dolt_log('main', '--min-parents', '1');", // Should show everything except first commit
				Expected: []sql.Row{{5}},
			},
			{
				Query:    "SELECT count(*) from dolt_log('main', '--min-parents', '1', '--merges');", // --merges overrides --min-parents
				Expected: []sql.Row{{1}},
			},
			{
				Query:    "SELECT commit_hash = @MergeCommit, committer, email, message from dolt_log('branch1..main', '--min-parents', '2');",
				Expected: []sql.Row{{true, "billy bob", "bigbillieb@fake.horse", "Merge branch 'branch2' into main"}},
			},
			{
				Query:    "SELECT count(*) from dolt_log('--min-parents', '5');",
				Expected: []sql.Row{{0}},
			},
			{
				Query:    "SELECT commit_hash = @MergeCommit, SUBSTRING_INDEX(parents, ', ', 1) = @Commit2, SUBSTRING_INDEX(parents, ', ', -1) = @Commit3 from dolt_log('main', '--parents', '--merges');",
				Expected: []sql.Row{{true, true, true}}, // shows two parents for merge commit
			},
			{
				Query:    "SELECT commit_hash = @Commit3, parents = @Commit1 from dolt_log('branch2', '--parents') LIMIT 1;", // shows one parent for non-merge commit
				Expected: []sql.Row{{true, true}},
			},
			{
				Query:    "SELECT commit_hash = @MergeCommit, SUBSTRING_INDEX(parents, ', ', 1) = @Commit2, SUBSTRING_INDEX(parents, ', ', -1) = @Commit3 from dolt_log('branch1..main', '--parents', '--merges') LIMIT 1;",
				Expected: []sql.Row{{true, true, true}},
			},
			{
				Query:    "SELECT commit_hash = @Commit2, parents = @Commit1 from dolt_log('branch2..branch1', '--parents') LIMIT 1;",
				Expected: []sql.Row{{true, true}},
			},
			{
				Query:    "SELECT refs from dolt_log('--decorate', 'short') LIMIT 1;",
				Expected: []sql.Row{{"HEAD -> main, tag: v1"}},
			},
			{
				Query:    "SELECT refs from dolt_log('--decorate', 'full') LIMIT 1;",
				Expected: []sql.Row{{"HEAD -> refs/heads/main, tag: refs/tags/v1"}},
			},
			{
				Query:    "SELECT commit_hash = @Commit2, parents = @Commit1, refs from dolt_log('branch2..branch1', '--parents', '--decorate', 'short') LIMIT 1;",
				Expected: []sql.Row{{true, true, "HEAD -> branch1"}},
			},
		},
	},*/
}

var LargeJsonObjectScriptTests = []queries.ScriptTest{
	{
		Name: "JSON under max length limit",
		SetUpScript: []string{
			"create table t (j JSON)",
		},
		Assertions: []queries.ScriptTestAssertion{
			{
				Query:    `insert into t set j= concat('[', repeat('"word",', 10000000), '"word"]')`,
				Expected: []sql.Row{{types.OkResult{RowsAffected: 1}}},
			},
		},
	},
	{
		Name: "JSON over max length limit",
		SetUpScript: []string{
			"create table t (j JSON)",
		},
		Assertions: []queries.ScriptTestAssertion{
			{
				Query:       `insert into t set j= concat('[', repeat('"word",', 50000000), '"word"]')`,
				ExpectedErr: types.ErrLengthTooLarge,
			},
		},
	},
}

var DoltTagTestScripts = []queries.ScriptTest{
	{
		Name: "dolt-tag: SQL create tags",
		SetUpScript: []string{
			"CREATE TABLE test(pk int primary key);",
			"CALL DOLT_ADD('.')",
			"INSERT INTO test VALUES (0),(1),(2);",
			"CALL DOLT_COMMIT('-am','created table test')",
		},
		Assertions: []queries.ScriptTestAssertion{
			{
				Query:    "CALL DOLT_TAG('v1', 'HEAD')",
				Expected: []sql.Row{{0}},
			},
			{
				Query:    "SELECT tag_name, IF(CHAR_LENGTH(tag_hash) < 0, NULL, 'not null'), tagger, email, IF(date IS NULL, NULL, 'not null'), message from dolt_tags",
				Expected: []sql.Row{{"v1", "not null", "billy bob", "bigbillieb@fake.horse", "not null", ""}},
			},
			{
				Query:    "CALL DOLT_TAG('v2', '-m', 'create tag v2')",
				Expected: []sql.Row{{0}},
			},
			{
				Query:    "SELECT tag_name, message from dolt_tags",
				Expected: []sql.Row{{"v1", ""}, {"v2", "create tag v2"}},
			},
		},
	},
	{
		Name: "dolt-tag: SQL delete tags",
		SetUpScript: []string{
			"CREATE TABLE test(pk int primary key);",
			"CALL DOLT_ADD('.')",
			"INSERT INTO test VALUES (0),(1),(2);",
			"CALL DOLT_COMMIT('-am','created table test')",
			"CALL DOLT_TAG('v1', '-m', 'create tag v1')",
			"CALL DOLT_TAG('v2', '-m', 'create tag v2')",
			"CALL DOLT_TAG('v3', '-m', 'create tag v3')",
		},
		Assertions: []queries.ScriptTestAssertion{
			{
				Query:    "SELECT tag_name, message from dolt_tags",
				Expected: []sql.Row{{"v1", "create tag v1"}, {"v2", "create tag v2"}, {"v3", "create tag v3"}},
			},
			{
				Query:    "CALL DOLT_TAG('-d','v1')",
				Expected: []sql.Row{{0}},
			},
			{
				Query:    "SELECT tag_name, message from dolt_tags",
				Expected: []sql.Row{{"v2", "create tag v2"}, {"v3", "create tag v3"}},
			},
			{
				Query:    "CALL DOLT_TAG('-d','v2','v3')",
				Expected: []sql.Row{{0}},
			},
			{
				Query:    "SELECT tag_name, message from dolt_tags",
				Expected: []sql.Row{},
			},
		},
	},
	{
		Name: "dolt-tag: SQL use a tag as a ref for merge",
		SetUpScript: []string{
			"CREATE TABLE test(pk int primary key);",
			"CALL DOLT_ADD('.')",
			"INSERT INTO test VALUES (0),(1),(2);",
			"CALL DOLT_COMMIT('-am','created table test')",
			"DELETE FROM test WHERE pk = 0",
			"INSERT INTO test VALUES (3)",
			"CALL DOLT_COMMIT('-am','made changes')",
		},
		Assertions: []queries.ScriptTestAssertion{
			{
				Query:    "CALL DOLT_TAG('v1','HEAD')",
				Expected: []sql.Row{{0}},
			},
			{
				Query:    "CALL DOLT_CHECKOUT('-b','other','HEAD^')",
				Expected: []sql.Row{{0}},
			},
			{
				Query:    "INSERT INTO test VALUES (8), (9)",
				Expected: []sql.Row{{types.OkResult{RowsAffected: 2}}},
			},
			{
				Query:            "CALL DOLT_COMMIT('-am','made changes in other')",
				SkipResultsCheck: true,
			},
			{
				Query:    "CALL DOLT_MERGE('v1')",
				Expected: []sql.Row{{0, 0}},
			},
			{
				Query:    "SELECT * FROM test",
				Expected: []sql.Row{{1}, {2}, {3}, {8}, {9}},
			},
		},
	},
}

var DoltRemoteTestScripts = []queries.ScriptTest{
	{
		Name: "dolt-remote: SQL add remotes",
		Assertions: []queries.ScriptTestAssertion{
			{
				Query:    "CALL DOLT_REMOTE('add','origin','file://../test')",
				Expected: []sql.Row{{0}},
			},
			{
				Query:    "SELECT name, IF(CHAR_LENGTH(url) < 0, NULL, 'not null'), fetch_specs, params FROM DOLT_REMOTES",
				Expected: []sql.Row{{"origin", "not null", types.MustJSON(`["refs/heads/*:refs/remotes/origin/*"]`), types.MustJSON(`{}`)}},
			},
			{
				Query:          "CALL DOLT_REMOTE()",
				ExpectedErrStr: "error: invalid argument, use 'dolt_remotes' system table to list remotes",
			},
			{
				Query:          "CALL DOLT_REMOTE('origin')",
				ExpectedErrStr: "error: invalid argument",
			},
			{
				Query:          "INSERT INTO dolt_remotes (name, url) VALUES ('origin', 'file://../test')",
				ExpectedErrStr: "the dolt_remotes table is read-only; use the dolt_remote stored procedure to edit remotes",
			},
		},
	},
	{
		Name: "dolt-remote: SQL remove remotes",
		SetUpScript: []string{
			"CALL DOLT_REMOTE('add','origin1','file://.')",
			"CALL DOLT_REMOTE('add','origin2','aws://[dynamo_db_table:s3_bucket]/repo_name')",
		},
		Assertions: []queries.ScriptTestAssertion{
			{
				Query: "SELECT name, IF(CHAR_LENGTH(url) < 0, NULL, 'not null'), fetch_specs, params FROM DOLT_REMOTES",
				Expected: []sql.Row{
					{"origin1", "not null", types.MustJSON(`["refs/heads/*:refs/remotes/origin1/*"]`), types.MustJSON(`{}`)},
					{"origin2", "not null", types.MustJSON(`["refs/heads/*:refs/remotes/origin2/*"]`), types.MustJSON(`{}`)}},
			},
			{
				Query:    "CALL DOLT_REMOTE('remove','origin2')",
				Expected: []sql.Row{{0}},
			},
			{
				Query:    "SELECT name, IF(CHAR_LENGTH(url) < 0, NULL, 'not null'), fetch_specs, params FROM DOLT_REMOTES",
				Expected: []sql.Row{{"origin1", "not null", types.MustJSON(`["refs/heads/*:refs/remotes/origin1/*"]`), types.MustJSON(`{}`)}},
			},
			// 'origin1' remote must exist in order this error to be returned; otherwise, no error from EOF
			{
				Query:          "DELETE FROM dolt_remotes WHERE name = 'origin1'",
				ExpectedErrStr: "the dolt_remotes table is read-only; use the dolt_remote stored procedure to edit remotes",
			},
		},
	},
	{
		Name: "dolt-remote: multi-repo test",
		SetUpScript: []string{
			"create database one",
		},
		Assertions: []queries.ScriptTestAssertion{
			{
				Query:    "use one;",
				Expected: []sql.Row{},
			},
			{
				Query:    "CALL DOLT_REMOTE('add','test01','file:///foo');",
				Expected: []sql.Row{{0}},
			},
			{
				Query:    "select count(*) from dolt_remotes where name='test01';",
				Expected: []sql.Row{{1}},
			},
			{
				Query:    "use mydb;",
				Expected: []sql.Row{},
			},
			{
				Query:    "select count(*) from dolt_remotes where name='test01';",
				Expected: []sql.Row{{0}},
			},
		},
	},
}

// DoltAutoIncrementTests is tests of dolt's global auto increment logic
var DoltAutoIncrementTests = []queries.ScriptTest{
	{
		Name: "insert on different branches",
		SetUpScript: []string{
			"create table t (a int primary key auto_increment, b int)",
			"call dolt_add('.')",
			"call dolt_commit('-am', 'empty table')",
			"call dolt_branch('branch1')",
			"call dolt_branch('branch2')",
		},
		Assertions: []queries.ScriptTestAssertion{
			{
				Query:    "insert into t (b) values (1), (2)",
				Expected: []sql.Row{{types.OkResult{RowsAffected: 2, InsertID: 1}}},
			},
			{
				Query:            "call dolt_commit('-am', 'two values on main')",
				SkipResultsCheck: true,
			},
			{
				Query:            "call dolt_checkout('branch1')",
				SkipResultsCheck: true,
			},
			{
				Query:    "insert into t (b) values (3), (4)",
				Expected: []sql.Row{{types.OkResult{RowsAffected: 2, InsertID: 3}}},
			},
			{
				Query: "select * from t order by a",
				Expected: []sql.Row{
					{3, 3},
					{4, 4},
				},
			},
			{
				Query:            "call dolt_commit('-am', 'two values on branch1')",
				SkipResultsCheck: true,
			},
			{
				Query:            "call dolt_checkout('branch2')",
				SkipResultsCheck: true,
			},
			{
				Query:    "insert into t (b) values (5), (6)",
				Expected: []sql.Row{{types.OkResult{RowsAffected: 2, InsertID: 5}}},
			},
			{
				Query: "select * from t order by a",
				Expected: []sql.Row{
					{5, 5},
					{6, 6},
				},
			},
		},
	},
	{
		Name: "drop table",
		SetUpScript: []string{
			"create table t (a int primary key auto_increment, b int)",
			"call dolt_add('.')",
			"call dolt_commit('-am', 'empty table')",
			"call dolt_branch('branch1')",
			"call dolt_branch('branch2')",
			"insert into t (b) values (1), (2)",
			"call dolt_commit('-am', 'two values on main')",
			"call dolt_checkout('branch1')",
			"insert into t (b) values (3), (4)",
			"call dolt_commit('-am', 'two values on branch1')",
			"call dolt_checkout('branch2')",
			"insert into t (b) values (5), (6)",
			"call dolt_checkout('branch1')",
		},
		Assertions: []queries.ScriptTestAssertion{
			{
				Query:    "drop table t",
				Expected: []sql.Row{{types.NewOkResult(0)}},
			},
			{
				Query:            "call dolt_checkout('main')",
				SkipResultsCheck: true,
			},
			{
				// highest value in any branch is 6
				Query:    "insert into t (b) values (7), (8)",
				Expected: []sql.Row{{types.OkResult{RowsAffected: 2, InsertID: 7}}},
			},
			{
				Query: "select * from t order by a",
				Expected: []sql.Row{
					{1, 1},
					{2, 2},
					{7, 7},
					{8, 8},
				},
			},
			{
				Query:    "drop table t",
				Expected: []sql.Row{{types.NewOkResult(0)}},
			},
			{
				Query:            "call dolt_checkout('branch2')",
				SkipResultsCheck: true,
			},
			{
				// highest value in any branch is still 6 (dropped table above)
				Query:    "insert into t (b) values (7), (8)",
				Expected: []sql.Row{{types.OkResult{RowsAffected: 2, InsertID: 7}}},
			},
			{
				Query: "select * from t order by a",
				Expected: []sql.Row{
					{5, 5},
					{6, 6},
					{7, 7},
					{8, 8},
				},
			},
			{
				Query:    "drop table t",
				Expected: []sql.Row{{types.NewOkResult(0)}},
			},
			{
				Query:            "create table t (a int primary key auto_increment, b int)",
				SkipResultsCheck: true,
			},
			{
				// no value on any branch
				Query:    "insert into t (b) values (1), (2)",
				Expected: []sql.Row{{types.OkResult{RowsAffected: 2, InsertID: 1}}},
			},
			{
				Query: "select * from t order by a",
				Expected: []sql.Row{
					{1, 1},
					{2, 2},
				},
			},
		},
	},
}

var BrokenAutoIncrementTests = []queries.ScriptTest{
	{
		// truncate table doesn't reset the persisted auto increment counter of tables on other branches, which leads to
		// the value not resetting to 1 after a truncate if the table exists on other branches, even if truncated on every
		// branch
		Name: "truncate table",
		SetUpScript: []string{
			"create table t (a int primary key auto_increment, b int)",
			"call dolt_add('.')",
			"call dolt_commit('-am', 'empty table')",
			"call dolt_branch('branch1')",
			"call dolt_branch('branch2')",
			"insert into t (b) values (1), (2)",
			"call dolt_commit('-am', 'two values on main')",
			"call dolt_checkout('branch1')",
			"insert into t (b) values (3), (4)",
			"call dolt_commit('-am', 'two values on branch1')",
			"call dolt_checkout('branch2')",
			"insert into t (b) values (5), (6)",
			"call dolt_checkout('branch1')",
		},
		Assertions: []queries.ScriptTestAssertion{
			{
				Query:    "truncate table t",
				Expected: []sql.Row{{types.NewOkResult(2)}},
			},
			{
				Query:            "call dolt_checkout('main')",
				SkipResultsCheck: true,
			},
			{
				// highest value in any branch is 6
				Query:    "insert into t (b) values (7), (8)",
				Expected: []sql.Row{{types.OkResult{RowsAffected: 2, InsertID: 7}}},
			},
			{
				Query: "select * from t order by a",
				Expected: []sql.Row{
					{1, 1},
					{2, 2},
					{7, 7},
					{8, 8},
				},
			},
			{
				Query:    "truncate table t",
				Expected: []sql.Row{{types.NewOkResult(4)}},
			},
			{
				Query:            "call dolt_checkout('branch2')",
				SkipResultsCheck: true,
			},
			{
				// highest value in any branch is still 6 (truncated table above)
				Query:    "insert into t (b) values (7), (8)",
				Expected: []sql.Row{{types.OkResult{RowsAffected: 2, InsertID: 7}}},
			},
			{
				Query: "select * from t order by a",
				Expected: []sql.Row{
					{5, 5},
					{6, 6},
					{7, 7},
					{8, 8},
				},
			},
			{
				Query:    "truncate table t",
				Expected: []sql.Row{{types.NewOkResult(4)}},
			},
			{
				// no value on any branch
				Query:    "insert into t (b) values (1), (2)",
				Expected: []sql.Row{{types.OkResult{RowsAffected: 2, InsertID: 1}}},
			},
			{
				Query: "select * from t order by a",
				Expected: []sql.Row{
					{1, 1},
					{2, 2},
				},
			},
		},
	},
}

var DoltCommitTests = []queries.ScriptTest{
	{
		Name: "CALL DOLT_COMMIT('-ALL') adds all tables (including new ones) to the commit.",
		SetUpScript: []string{
			"CREATE table t (pk int primary key);",
			"INSERT INTO t VALUES (1);",
			"CALL DOLT_ADD('t');",
			"CALL DOLT_COMMIT('-m', 'add table t');",
			"CALL DOLT_RESET('--hard');",
		},
		Assertions: []queries.ScriptTestAssertion{
			{
				Query:    "SELECT * from t;",
				Expected: []sql.Row{{1}},
			},
			// update a table
			{
				Query:    "DELETE from t where pk = 1;",
				Expected: []sql.Row{{types.NewOkResult(1)}},
			},
			{
				Query:            "CALL DOLT_COMMIT('-ALL', '-m', 'update table terminator');",
				SkipResultsCheck: true,
			},
			// check last commit
			{
				Query:    "select message from dolt_log limit 1",
				Expected: []sql.Row{{"update table terminator"}},
			},
			// amend last commit
			{
				Query:            "CALL DOLT_COMMIT('-amend', '-m', 'update table t');",
				SkipResultsCheck: true,
			},
			// check amended commit
			{
				Query:    "SELECT * from t;",
				Expected: []sql.Row{},
			},
			{
				Query:    "select message from dolt_log limit 1",
				Expected: []sql.Row{{"update table t"}},
			},
			{
				Query:    "CALL DOLT_RESET('--hard');",
				Expected: []sql.Row{{0}},
			},
			{
				Query:    "SELECT * from t;",
				Expected: []sql.Row{},
			},
			// delete a table
			{
				Query:    "DROP TABLE t;",
				Expected: []sql.Row{{types.NewOkResult(0)}},
			},
			{
				Query:            "CALL DOLT_COMMIT('-Am', 'drop table t');",
				SkipResultsCheck: true,
			},
			{
				Query:    "CALL DOLT_RESET('--hard');",
				Expected: []sql.Row{{0}},
			},
			{
				Query:       "SELECT * from t;",
				ExpectedErr: sql.ErrTableNotFound,
			},
			// create a table
			{
				Query:    "CREATE table t2 (pk int primary key);",
				Expected: []sql.Row{{types.NewOkResult(0)}},
			},
			{
				Query:            "CALL DOLT_COMMIT('-Am', 'add table 21');",
				SkipResultsCheck: true,
			},
			// amend last commit
			{
				Query:            "CALL DOLT_COMMIT('-amend', '-m', 'add table 2');",
				SkipResultsCheck: true,
			},
			// check amended commit
			{
				Query:    "select message from dolt_log limit 1",
				Expected: []sql.Row{{"add table 2"}},
			},
			{
				Query:    "CALL DOLT_RESET('--hard');",
				Expected: []sql.Row{{0}},
			},
			{
				Query:    "SELECT * from t2;",
				Expected: []sql.Row{},
			},
		},
	},
	{
		Name: "dolt commit works with arguments",
		SetUpScript: []string{
			"CREATE table t (pk int primary key);",
			"INSERT INTO t VALUES (1);",
			"CALL DOLT_ADD('t');",
			"CALL DOLT_COMMIT('-m', concat('author: ','somebody'));",
		},
		Assertions: []queries.ScriptTestAssertion{
			{
				Query: "SELECT message from dolt_log where message = 'author: somebody'",
				Expected: []sql.Row{
					{"author: somebody"},
				},
			},
		},
	},
	{
		Name: "CALL DOLT_COMMIT('-amend') works to update commit message",
		SetUpScript: []string{
			"SET @@AUTOCOMMIT=0;",
			"CREATE TABLE test (id INT PRIMARY KEY );",
			"INSERT INTO test (id) VALUES (2)",
			"CALL DOLT_ADD('.');",
			"CALL DOLT_COMMIT('-m', 'original commit message');",
		},
		Assertions: []queries.ScriptTestAssertion{
			{
				Query: "SELECT  message FROM dolt_log;",
				Expected: []sql.Row{
					{"original commit message"},
					{"author: somebody"},
					{"add table 2"},
					{"drop table t"},
					{"update table t"},
					{"add table t"},
					{"checkpoint enginetest database mydb"},
					{"Initialize data repository"},
				},
			},
			{
				Query:    "SELECT to_id, from_id, diff_type FROM dolt_diff_test;",
				Expected: []sql.Row{{2, nil, "added"}},
			},
			{
				Query:            "CALL DOLT_COMMIT('--amend', '-m', 'amended commit message');",
				SkipResultsCheck: true, // commit hash is being returned, skip check
			},
			{
				Query: "SELECT  message FROM dolt_log;",
				Expected: []sql.Row{
					{"amended commit message"},
					{"author: somebody"},
					{"add table 2"},
					{"drop table t"},
					{"update table t"},
					{"add table t"},
					{"checkpoint enginetest database mydb"},
					{"Initialize data repository"},
				},
			},
			{
				Query:    "SELECT to_id, from_id, diff_type FROM dolt_diff_test;",
				Expected: []sql.Row{{2, nil, "added"}},
			},
		},
	},
	{
		Name: "CALL DOLT_COMMIT('-amend') works to add changes to a commit",
		SetUpScript: []string{
			"SET @@AUTOCOMMIT=0;",
			"INSERT INTO test (id) VALUES (3)",
			"CALL DOLT_ADD('.');",
			"CALL DOLT_COMMIT('-m', 'original commit message for adding changes to a commit');",
		},
		Assertions: []queries.ScriptTestAssertion{
			{
				Query: "SELECT to_id, from_id, diff_type FROM dolt_diff_test;",
				Expected: []sql.Row{
					{3, nil, "added"},
					{2, nil, "added"},
				},
			},
			{
				Query:    "SELECT COUNT(*) FROM dolt_status;",
				Expected: []sql.Row{{0}},
			},
			{
				Query: "SELECT  message FROM dolt_log;",
				Expected: []sql.Row{
					{"original commit message for adding changes to a commit"},
					{"amended commit message"},
					{"author: somebody"},
					{"add table 2"},
					{"drop table t"},
					{"update table t"},
					{"add table t"},
					{"checkpoint enginetest database mydb"},
					{"Initialize data repository"},
				},
			},
			{
				Query:    "INSERT INTO test (id) VALUES (4)",
				Expected: []sql.Row{{types.NewOkResult(1)}},
			},
			{
				Query:    "SELECT COUNT(*) FROM dolt_status;",
				Expected: []sql.Row{{1}},
			},
			{
				Query:    "CALL DOLT_ADD('.');",
				Expected: []sql.Row{{0}},
			},
			{
				Query:            "CALL DOLT_COMMIT('--amend');",
				SkipResultsCheck: true, // commit hash is being returned, skip check
			},
			{
				Query: "SELECT message FROM dolt_log;",
				Expected: []sql.Row{
					{"original commit message for adding changes to a commit"},
					{"amended commit message"},
					{"author: somebody"},
					{"add table 2"},
					{"drop table t"},
					{"update table t"},
					{"add table t"},
					{"checkpoint enginetest database mydb"},
					{"Initialize data repository"},
				},
			},
			{
				Query: "SELECT to_id, from_id, diff_type FROM dolt_diff_test;",
				Expected: []sql.Row{
					{4, nil, "added"},
					{3, nil, "added"},
					{2, nil, "added"},
				},
			},
			{
				Query:    "INSERT INTO test (id) VALUES (5)",
				Expected: []sql.Row{{types.NewOkResult(1)}},
			},
			{
				Query:    "SELECT COUNT(*) FROM dolt_status;",
				Expected: []sql.Row{{1}},
			},
			{
				Query:    "CALL DOLT_ADD('.');",
				Expected: []sql.Row{{0}},
			},
			{
				Query:            "CALL DOLT_COMMIT('--amend', '-m', 'amended commit with added changes');",
				SkipResultsCheck: true, // commit hash is being returned, skip check
			},
			{
				Query:    "SELECT COUNT(*) FROM dolt_status;",
				Expected: []sql.Row{{0}},
			},
			{
				Query: "SELECT message FROM dolt_log;",
				Expected: []sql.Row{
					{"amended commit with added changes"},
					{"amended commit message"},
					{"author: somebody"},
					{"add table 2"},
					{"drop table t"},
					{"update table t"},
					{"add table t"},
					{"checkpoint enginetest database mydb"},
					{"Initialize data repository"},
				},
			},
			{
				Query: "SELECT to_id, from_id, diff_type FROM dolt_diff_test;",
				Expected: []sql.Row{
					{5, nil, "added"},
					{4, nil, "added"},
					{3, nil, "added"},
					{2, nil, "added"},
				},
			},
		},
	},
	{
		Name: "CALL DOLT_COMMIT('-amend') works to remove changes from a commit",
		SetUpScript: []string{
			"SET @@AUTOCOMMIT=0;",
			"INSERT INTO test (id) VALUES (6)",
			"INSERT INTO test (id) VALUES (7)",
			"CALL DOLT_ADD('.');",
			"CALL DOLT_COMMIT('-m', 'original commit message');",
		},
		Assertions: []queries.ScriptTestAssertion{
			{
				Query:    "SELECT * FROM test;",
				Expected: []sql.Row{{2}, {3}, {4}, {5}, {6}, {7}},
			},
			{
				Query: "SELECT to_id, from_id, diff_type FROM dolt_diff_test;",
				Expected: []sql.Row{
					{7, nil, "added"},
					{6, nil, "added"},
					{5, nil, "added"},
					{4, nil, "added"},
					{3, nil, "added"},
					{2, nil, "added"},
				},
			},
			{
				Query:    "DELETE FROM test WHERE id = 6",
				Expected: []sql.Row{{types.NewOkResult(1)}},
			},
			{
				Query:    "CALL DOLT_ADD('.');",
				Expected: []sql.Row{{0}},
			},
			{
				Query:            "CALL DOLT_COMMIT('--amend', '-m', 'amended commit with removed changes');",
				SkipResultsCheck: true, // commit hash is being returned, skip check
			},
			{
				Query:    "SELECT * FROM test;",
				Expected: []sql.Row{{2}, {3}, {4}, {5}, {7}},
			},
			{
				Query: "SELECT message FROM dolt_log;",
				Expected: []sql.Row{
					{"amended commit with removed changes"},
					{"amended commit with added changes"},
					{"amended commit message"},
					{"author: somebody"},
					{"add table 2"},
					{"drop table t"},
					{"update table t"},
					{"add table t"},
					{"checkpoint enginetest database mydb"},
					{"Initialize data repository"},
				},
			},
			{
				Query: "SELECT to_id, from_id, diff_type FROM dolt_diff_test;",
				Expected: []sql.Row{
					{7, nil, "added"},
					{5, nil, "added"},
					{4, nil, "added"},
					{3, nil, "added"},
					{2, nil, "added"},
				},
			},
		},
	},
	{
		Name: "CALL DOLT_COMMIT('-amend') works to update a merge commit",
		SetUpScript: []string{
			"SET @@AUTOCOMMIT=0;",

			"CREATE TABLE test2 (id INT PRIMARY KEY, id2 INT);",
			"CALL DOLT_ADD('.');",
			"CALL DOLT_COMMIT('-m', 'original table');",

			"CALL DOLT_CHECKOUT('-b','test-branch');",
			"INSERT INTO test2 (id, id2) VALUES (0, 2)",
			"CALL DOLT_ADD('.');",
			"CALL DOLT_COMMIT('-m', 'conflicting commit message');",

			"CALL DOLT_CHECKOUT('main');",
			"INSERT INTO test2 (id, id2) VALUES (0, 1)",
			"CALL DOLT_ADD('.');",
			"CALL DOLT_COMMIT('-m', 'original commit message');",

			"CALL DOLT_MERGE('test-branch');",
			"CALL DOLT_CONFLICTS_RESOLVE('--theirs', '.');",
			"CALL DOLT_COMMIT('-m', 'final merge');",
		},
		Assertions: []queries.ScriptTestAssertion{
			{
				Query:            "CALL DOLT_COMMIT('--amend', '-m', 'new merge');",
				SkipResultsCheck: true, // commit hash is being returned, skip check
			},
			{
				Query: "SELECT message FROM dolt_log;",
				Expected: []sql.Row{
					{"new merge"},
					{"original commit message"},
					{"conflicting commit message"},
					{"original table"},
					{"amended commit with removed changes"},
					{"amended commit with added changes"},
					{"amended commit message"},
					{"author: somebody"},
					{"add table 2"},
					{"drop table t"},
					{"update table t"},
					{"add table t"},
					{"checkpoint enginetest database mydb"},
					{"Initialize data repository"},
				},
			},
			{
				Query:    "SET @hash=(SELECT commit_hash FROM dolt_log LIMIT 1);",
				Expected: []sql.Row{{}},
			},
			{
				Query:    "SELECT COUNT(parent_hash) FROM dolt_commit_ancestors WHERE commit_hash= @hash;",
				Expected: []sql.Row{{2}},
			},
		},
	},
}

var DoltIndexPrefixScripts = []queries.ScriptTest{
	{
		Name: "inline secondary indexes with collation",
		SetUpScript: []string{
			"create table t (i int primary key, v1 varchar(10), v2 varchar(10), unique index (v1(3),v2(5))) collate utf8mb4_0900_ai_ci",
		},
		Assertions: []queries.ScriptTestAssertion{
			{
				Query:    "show create table t",
				Expected: []sql.Row{{"t", "CREATE TABLE `t` (\n  `i` int NOT NULL,\n  `v1` varchar(10) COLLATE utf8mb4_0900_ai_ci,\n  `v2` varchar(10) COLLATE utf8mb4_0900_ai_ci,\n  PRIMARY KEY (`i`),\n  UNIQUE KEY `v1v2` (`v1`(3),`v2`(5))\n) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci"}},
			},
			{
				Query:    "insert into t values (0, 'a', 'a'), (1, 'ab','ab'), (2, 'abc', 'abc'), (3, 'abcde', 'abcde')",
				Expected: []sql.Row{{types.NewOkResult(4)}},
			},
			{
				Query:       "insert into t values (99, 'ABC', 'ABCDE')",
				ExpectedErr: sql.ErrUniqueKeyViolation,
			},
			{
				Query:          "insert into t values (99, 'ABC', 'ABCDE')",
				ExpectedErrStr: "duplicate unique key given: [ABC,ABCDE]",
			},
			{
				Query:       "insert into t values (99, 'ABC123', 'ABCDE123')",
				ExpectedErr: sql.ErrUniqueKeyViolation,
			},
			{
				Query:          "insert into t values (99, 'ABC123', 'ABCDE123')",
				ExpectedErrStr: "duplicate unique key given: [ABC,ABCDE]",
			},
			{
				Query: "select * from t where v1 = 'A'",
				Expected: []sql.Row{
					{0, "a", "a"},
				},
			},
			{
				Query: "select * from t where v1 = 'ABC'",
				Expected: []sql.Row{
					{2, "abc", "abc"},
				},
			},
			{
				Query:    "select * from t where v1 = 'ABCD'",
				Expected: []sql.Row{},
			},
			{
				Query: "select * from t where v1 > 'A' and v1 < 'ABCDE'",
				Expected: []sql.Row{
					{1, "ab", "ab"},
					{2, "abc", "abc"},
				},
			},
			{
				Query: "select * from t where v1 > 'A' and v2 < 'ABCDE'",
				Expected: []sql.Row{
					{1, "ab", "ab"},
					{2, "abc", "abc"},
				},
			},
			{
				Query: "update t set v1 = concat(v1, 'Z') where v1 >= 'A'",
				Expected: []sql.Row{
					{types.OkResult{RowsAffected: 4, InsertID: 0, Info: plan.UpdateInfo{Matched: 4, Updated: 4}}},
				},
			},
			{
				Query: "select * from t",
				Expected: []sql.Row{
					{0, "aZ", "a"},
					{1, "abZ", "ab"},
					{2, "abcZ", "abc"},
					{3, "abcdeZ", "abcde"},
				},
			},
			{
				Query: "delete from t where v1 >= 'A'",
				Expected: []sql.Row{
					{types.OkResult{RowsAffected: 4}},
				},
			},
			{
				Query:    "select * from t",
				Expected: []sql.Row{},
			},
		},
	},
	// TODO: these should eventually go in GMS, but it doesn't currently support index rewrite on column modify
	{
		Name: "drop prefix lengths when modifying column to non string type",
		SetUpScript: []string{
			"create table t (j varchar(100), index (j(10)))",
		},
		Assertions: []queries.ScriptTestAssertion{
			{
				Query:    "alter table t modify column j int",
				Expected: []sql.Row{{types.OkResult{}}},
			},
			{
				Query:    "show create table t",
				Expected: []sql.Row{{"t", "CREATE TABLE `t` (\n  `j` int,\n  KEY `j` (`j`)\n) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_bin"}},
			},
		},
	},
	{
		Name: "drop prefix length when modifying columns to invalid string type",
		SetUpScript: []string{
			"create table t (j varchar(100), index (j(10)))",
		},
		Assertions: []queries.ScriptTestAssertion{
			{
				Query:    "alter table t modify column j varchar(2)",
				Expected: []sql.Row{{types.OkResult{}}},
			},
			{
				Query:    "show create table t",
				Expected: []sql.Row{{"t", "CREATE TABLE `t` (\n  `j` varchar(2),\n  KEY `j` (`j`)\n) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_bin"}},
			},
		},
	},
	{
		Name: "preserve prefix length when modifying column to valid string type",
		SetUpScript: []string{
			"create table t (j varchar(100), index (j(10)))",
		},
		Assertions: []queries.ScriptTestAssertion{
			{
				Query:    "alter table t modify column j varchar(200)",
				Expected: []sql.Row{{types.OkResult{}}},
			},
			{
				Query:    "show create table t",
				Expected: []sql.Row{{"t", "CREATE TABLE `t` (\n  `j` varchar(200),\n  KEY `j` (`j`(10))\n) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_bin"}},
			},
		},
	},
	{
		Name: "preserve prefix lengths when there are other unchanged prefix lengths",
		SetUpScript: []string{
			"create table t (i varchar(100), j varchar(100), index (i(10), j(10)))",
		},
		Assertions: []queries.ScriptTestAssertion{
			{
				Query:    "alter table t modify column j int",
				Expected: []sql.Row{{types.OkResult{}}},
			},
			{
				Query:    "show create table t",
				Expected: []sql.Row{{"t", "CREATE TABLE `t` (\n  `i` varchar(100),\n  `j` int,\n  KEY `ij` (`i`(10),`j`)\n) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_bin"}},
			},
		},
	},
	{
		Name: "prefix length too long",
		SetUpScript: []string{
			"create table t (i blob, index(i(3072)))",
		},
		Assertions: []queries.ScriptTestAssertion{
			{
				Query:       "alter table t modify column i text",
				ExpectedErr: sql.ErrKeyTooLong,
			},
		},
	},
}

// DoltCallAsOf are tests of using CALL ... AS OF using commits
var DoltCallAsOf = []queries.ScriptTest{
	{
		Name: "Database syntax properly handles inter-CALL communication",
		SetUpScript: []string{
			`CREATE PROCEDURE p1()
BEGIN
	DECLARE str VARCHAR(20);
    CALL p2(str);
	SET str = CONCAT('a', str);
    SELECT str;
END`,
			`CREATE PROCEDURE p2(OUT param VARCHAR(20))
BEGIN
	SET param = 'b';
END`,
			"CALL DOLT_ADD('-A');",
			"CALL DOLT_COMMIT('-m', 'First procedures');",
			"CALL DOLT_BRANCH('p12');",
			"DROP PROCEDURE p1;",
			"DROP PROCEDURE p2;",
			`CREATE PROCEDURE p1()
BEGIN
	DECLARE str VARCHAR(20);
    CALL p2(str);
	SET str = CONCAT('c', str);
    SELECT str;
END`,
			`CREATE PROCEDURE p2(OUT param VARCHAR(20))
BEGIN
	SET param = 'd';
END`,
			"CALL DOLT_ADD('-A');",
			"CALL DOLT_COMMIT('-m', 'Second procedures');",
		},
		Assertions: []queries.ScriptTestAssertion{
			{
				Query:    "CALL p1();",
				Expected: []sql.Row{{"cd"}},
			},
			{
				Query:    "CALL `mydb/main`.p1();",
				Expected: []sql.Row{{"cd"}},
			},
			{
				Query:    "CALL `mydb/p12`.p1();",
				Expected: []sql.Row{{"ab"}},
			},
		},
	},
	{
		Name: "CALL ... AS OF references historic data through nested calls",
		SetUpScript: []string{
			"CREATE TABLE test (v1 BIGINT);",
			"INSERT INTO test VALUES (1);",
			`CREATE PROCEDURE p1()
BEGIN
	CALL p2();
END`,
			`CREATE PROCEDURE p2()
BEGIN
	SELECT * FROM test;
END`,
			"CALL DOLT_ADD('-A');",
			"CALL DOLT_COMMIT('-m', 'commit message');",
			"UPDATE test SET v1 = 2;",
			"CALL DOLT_ADD('-A');",
			"CALL DOLT_COMMIT('-m', 'commit message');",
			"UPDATE test SET v1 = 3;",
			"CALL DOLT_ADD('-A');",
			"CALL DOLT_COMMIT('-m', 'commit message');",
			"UPDATE test SET v1 = 4;",
			"CALL DOLT_ADD('-A');",
			"CALL DOLT_COMMIT('-m', 'commit message');",
		},
		Assertions: []queries.ScriptTestAssertion{
			{
				Query:    "CALL p1();",
				Expected: []sql.Row{{4}},
			},
			{
				Query:    "CALL p1() AS OF 'HEAD';",
				Expected: []sql.Row{{4}},
			},
			{
				Query:    "CALL p1() AS OF 'HEAD~1';",
				Expected: []sql.Row{{3}},
			},
			{
				Query:    "CALL p1() AS OF 'HEAD~2';",
				Expected: []sql.Row{{2}},
			},
			{
				Query:    "CALL p1() AS OF 'HEAD~3';",
				Expected: []sql.Row{{1}},
			},
		},
	},
	{
		Name: "CALL ... AS OF doesn't overwrite nested CALL ... AS OF",
		SetUpScript: []string{
			"CREATE TABLE myhistorytable (pk BIGINT PRIMARY KEY, s TEXT);",
			"INSERT INTO myhistorytable VALUES (1, 'first row, 1'), (2, 'second row, 1'), (3, 'third row, 1');",
			"CREATE PROCEDURE p1() BEGIN CALL p2(); END",
			"CREATE PROCEDURE p1a() BEGIN CALL p2() AS OF 'HEAD~2'; END",
			"CREATE PROCEDURE p1b() BEGIN CALL p2a(); END",
			"CREATE PROCEDURE p2() BEGIN SELECT * FROM myhistorytable; END",
			"CALL DOLT_ADD('-A');",
			"CALL DOLT_COMMIT('-m', 'commit message');",
			"DELETE FROM myhistorytable;",
			"INSERT INTO myhistorytable VALUES (1, 'first row, 2'), (2, 'second row, 2'), (3, 'third row, 2');",
			"CALL DOLT_ADD('-A');",
			"CALL DOLT_COMMIT('-m', 'commit message');",
			"DROP TABLE myhistorytable;",
			"CREATE TABLE myhistorytable (pk BIGINT PRIMARY KEY, s TEXT, c TEXT);",
			"INSERT INTO myhistorytable VALUES (1, 'first row, 3', '1'), (2, 'second row, 3', '2'), (3, 'third row, 3', '3');",
			"CREATE PROCEDURE p2a() BEGIN SELECT * FROM myhistorytable AS OF 'HEAD~1'; END",
			"CALL DOLT_ADD('-A');",
			"CALL DOLT_COMMIT('-m', 'commit message');",
		},
		Assertions: []queries.ScriptTestAssertion{
			{
				Query: "CALL p1();",
				Expected: []sql.Row{
					{int64(1), "first row, 3", "1"},
					{int64(2), "second row, 3", "2"},
					{int64(3), "third row, 3", "3"},
				},
			},
			{
				Query: "CALL p1a();",
				Expected: []sql.Row{
					{int64(1), "first row, 1"},
					{int64(2), "second row, 1"},
					{int64(3), "third row, 1"},
				},
			},
			{
				Query: "CALL p1b();",
				Expected: []sql.Row{
					{int64(1), "first row, 2"},
					{int64(2), "second row, 2"},
					{int64(3), "third row, 2"},
				},
			},
			{
				Query: "CALL p2();",
				Expected: []sql.Row{
					{int64(1), "first row, 3", "1"},
					{int64(2), "second row, 3", "2"},
					{int64(3), "third row, 3", "3"},
				},
			},
			{
				Query: "CALL p2a();",
				Expected: []sql.Row{
					{int64(1), "first row, 2"},
					{int64(2), "second row, 2"},
					{int64(3), "third row, 2"},
				},
			},
			{
				Query: "CALL p1() AS OF 'HEAD~2';",
				Expected: []sql.Row{
					{int64(1), "first row, 1"},
					{int64(2), "second row, 1"},
					{int64(3), "third row, 1"},
				},
			},
			{
				Query: "CALL p1a() AS OF 'HEAD';",
				Expected: []sql.Row{
					{int64(1), "first row, 1"},
					{int64(2), "second row, 1"},
					{int64(3), "third row, 1"},
				},
			},
			{
				Query: "CALL p1b() AS OF 'HEAD';",
				Expected: []sql.Row{
					{int64(1), "first row, 2"},
					{int64(2), "second row, 2"},
					{int64(3), "third row, 2"},
				},
			},
			{
				Query: "CALL p2() AS OF 'HEAD~2';",
				Expected: []sql.Row{
					{int64(1), "first row, 1"},
					{int64(2), "second row, 1"},
					{int64(3), "third row, 1"},
				},
			},
			{
				Query: "CALL p2a() AS OF 'HEAD';",
				Expected: []sql.Row{
					{int64(1), "first row, 2"},
					{int64(2), "second row, 2"},
					{int64(3), "third row, 2"},
				},
			},
		},
	},
	{
		Name: "CALL ... AS OF errors if attempting to modify a table",
		SetUpScript: []string{
			"CREATE TABLE test (v1 BIGINT);",
			"INSERT INTO test VALUES (2);",
			"CALL DOLT_ADD('-A');",
			"CALL DOLT_COMMIT('-m', 'commit message');",
			`CREATE PROCEDURE p1()
BEGIN
	UPDATE test SET v1 = v1 * 2;
END`,
			"CALL DOLT_ADD('-A');",
			"CALL DOLT_COMMIT('-m', 'commit message');",
		},
		Assertions: []queries.ScriptTestAssertion{
			{
				Query:    "SELECT * FROM test;",
				Expected: []sql.Row{{2}},
			},
			{
				Query:    "CALL p1();",
				Expected: []sql.Row{{types.OkResult{RowsAffected: 1, Info: plan.UpdateInfo{Matched: 1, Updated: 1}}}},
			},
			{
				Query:    "SELECT * FROM test;",
				Expected: []sql.Row{{4}},
			},
			{
				Query:       "CALL p1() AS OF 'HEAD~1';",
				ExpectedErr: sql.ErrProcedureCallAsOfReadOnly,
			},
		},
	},
	{
		Name: "Database syntax propogates to inner calls",
		SetUpScript: []string{
			"CALL DOLT_CHECKOUT('main');",
			`CREATE PROCEDURE p4()
BEGIN
	CALL p5();
END`,
			`CREATE PROCEDURE p5()
BEGIN
	SELECT 3;
END`,
			"CALL DOLT_ADD('-A');",
			"CALL DOLT_COMMIT('-m', 'commit message');",
			"CALL DOLT_BRANCH('p45');",
			"DROP PROCEDURE p4;",
			"DROP PROCEDURE p5;",
			`CREATE PROCEDURE p4()
BEGIN
	CALL p5();
END`,
			`CREATE PROCEDURE p5()
BEGIN
	SELECT 4;
END`,
			"CALL DOLT_ADD('-A');",
			"CALL DOLT_COMMIT('-m', 'commit message');",
		},
		Assertions: []queries.ScriptTestAssertion{
			{
				Query:    "CALL p4();",
				Expected: []sql.Row{{4}},
			},
			{
				Query:    "CALL p5();",
				Expected: []sql.Row{{4}},
			},
			{
				Query:    "CALL `mydb/main`.p4();",
				Expected: []sql.Row{{4}},
			},
			{
				Query:    "CALL `mydb/main`.p5();",
				Expected: []sql.Row{{4}},
			},
			{
				Query:    "CALL `mydb/p45`.p4();",
				Expected: []sql.Row{{3}},
			},
			{
				Query:    "CALL `mydb/p45`.p5();",
				Expected: []sql.Row{{3}},
			},
		},
	},
	{
		Name: "Database syntax with AS OF",
		SetUpScript: []string{
			"CREATE TABLE test (v1 BIGINT);",
			"INSERT INTO test VALUES (2);",
			`CREATE PROCEDURE p1()
BEGIN
	SELECT v1 * 10 FROM test;
END`,
			"CALL DOLT_ADD('-A');",
			"CALL DOLT_COMMIT('-m', 'commit message');",
			"CALL DOLT_BRANCH('other');",
			"DROP PROCEDURE p1;",
			`CREATE PROCEDURE p1()
BEGIN
	SELECT v1 * 100 FROM test;
END`,
			"UPDATE test SET v1 = 3;",
			"CALL DOLT_ADD('-A');",
			"CALL DOLT_COMMIT('-m', 'commit message');",
		},
		Assertions: []queries.ScriptTestAssertion{
			{
				Query:    "CALL p1();",
				Expected: []sql.Row{{300}},
			},
			{
				Query:    "CALL `mydb/main`.p1();",
				Expected: []sql.Row{{300}},
			},
			{
				Query:    "CALL `mydb/other`.p1();",
				Expected: []sql.Row{{30}},
			},
			{
				Query:    "CALL p1() AS OF 'HEAD';",
				Expected: []sql.Row{{300}},
			},
			{
				Query:    "CALL `mydb/main`.p1() AS OF 'HEAD';",
				Expected: []sql.Row{{300}},
			},
			{
				Query:    "CALL `mydb/other`.p1() AS OF 'HEAD';",
				Expected: []sql.Row{{30}},
			},
			{
				Query:    "CALL p1() AS OF 'HEAD~1';",
				Expected: []sql.Row{{200}},
			},
			{
				Query:    "CALL `mydb/main`.p1() AS OF 'HEAD~1';",
				Expected: []sql.Row{{200}},
			},
			{
				Query:    "CALL `mydb/other`.p1() AS OF 'HEAD~1';",
				Expected: []sql.Row{{20}},
			},
		},
	},
}
