package dbdiff

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/go-sql-driver/mysql"
)

type Driver struct {
	newDb *sql.DB
	oldDb *sql.DB
}

// New creates a new Driver driver.
// The DSN is documented here: https://github.com/go-sql-driver/mysql#dsn-data-source-name
func NewMysqlDiffer(newDsn, oldDsn string) (Differ, error) {
	parsedNewDSN, err := mysql.ParseDSN(newDsn)
	if err != nil {
		return nil, err
	}

	parsedNewDSN.MultiStatements = true
	newDb, err := sql.Open("mysql", parsedNewDSN.FormatDSN())
	if err != nil {
		return nil, err
	}
	if err := newDb.Ping(); err != nil {
		return nil, err
	}

	parsedOldDSN, err := mysql.ParseDSN(oldDsn)
	if err != nil {
		return nil, err
	}

	parsedOldDSN.MultiStatements = true
	oldDb, err := sql.Open("mysql", parsedOldDSN.FormatDSN())
	if err != nil {
		return nil, err
	}
	if err := oldDb.Ping(); err != nil {
		return nil, err
	}

	d := &Driver{
		newDb: newDb,
		oldDb: oldDb,
	}
	return d, nil
}

// NewFromDB returns a mysql driver from a sql.DB
func NewMysqlDifferFromDB(newDb, oldDb *sql.DB) (Differ, error) {
	if _, ok := newDb.Driver().(*mysql.MySQLDriver); !ok {
		return nil, errors.New("new database instance is not using the MySQL driver")
	}
	if _, ok := oldDb.Driver().(*mysql.MySQLDriver); !ok {
		return nil, errors.New("old database instance is not using the MySQL driver")
	}

	if err := newDb.Ping(); err != nil {
		return nil, err
	}

	if err := oldDb.Ping(); err != nil {
		return nil, err
	}

	d := &Driver{
		newDb: newDb,
		oldDb: oldDb,
	}
	return d, nil
}

// Close closes the connection to the Driver server.
func (d *Driver) Close() error {
	err := d.newDb.Close()
	if err != nil {
		return err
	}

	err = d.oldDb.Close()
	if err != nil {
		return err
	}
	return nil
}

func (d *Driver) Diff(prefix string) (diff *Result, err error) {
	// retrive new database structure
	newtables, newtablespos, err := tables(d.newDb, prefix)
	if err != nil {
		return nil, err
	}
	newtablefields := make(map[string][]Field, len(newtables))
	newtablefieldspos := make(map[string]map[string]int, len(newtables))
	newtableindexes := make(map[string][]Index, len(newtables))
	newtableindexespos := make(map[string]map[string]int, len(newtables))
	for _, table := range newtables {
		newtablefields[table.Name], newtablefieldspos[table.Name], err = fields(d.newDb, table.Name)
		if err != nil {
			return nil, err
		}
		newtableindexes[table.Name], newtableindexespos[table.Name], err = indexes(d.newDb, table.Name)
		if err != nil {
			return nil, err
		}
	}

	// retrive old database structure
	oldtables, oldtablespos, err := tables(d.oldDb, prefix)
	if err != nil {
		return nil, err
	}
	oldtablefields := make(map[string][]Field, len(oldtables))
	oldtablefieldspos := make(map[string]map[string]int, len(oldtables))
	oldtableindexes := make(map[string][]Index, len(oldtables))
	oldtableindexespos := make(map[string]map[string]int, len(oldtables))
	for _, table := range oldtables {
		oldtablefields[table.Name], oldtablefieldspos[table.Name], err = fields(d.oldDb, table.Name)
		if err != nil {
			return nil, err
		}
		oldtableindexes[table.Name], oldtableindexespos[table.Name], err = indexes(d.oldDb, table.Name)
		if err != nil {
			return nil, err
		}
	}

	// compare
	result := Result{
		Drop:   []Table{},
		Create: []Table{},
		Change: []Table{},
	}

	// table
	for _, olddetail := range oldtables {
		// table is not exist in new database, drop it
		if _, exist := newtablespos[olddetail.Name]; !exist {
			result.Drop = append(result.Drop, olddetail)
		}
	}
	for _, newdetail := range newtables {
		// create tables, create fields, create indexes
		if _, exist := oldtablespos[newdetail.Name]; !exist {
			newdetail.Fields.Create = newtablefields[newdetail.Name]
			newdetail.Indexes.Create = newtableindexes[newdetail.Name]
			result.Create = append(result.Create, newdetail)
		} else {
			// diff tables
			change := Table{
				Name:    newdetail.Name,
				Fields:  ResultFields{},
				Indexes: ResultIndexes{},
			}
			olddetail := oldtables[oldtablespos[newdetail.Name]]
			if !olddetail.Equal(newdetail) {
				change = newdetail
			}

			newindexes := newtableindexes[newdetail.Name]
			newindexespos := newtableindexespos[newdetail.Name]
			oldindexes := oldtableindexes[olddetail.Name]
			oldindexespos := oldtableindexespos[olddetail.Name]

			for _, oldindex := range oldindexes {
				if pos, exist := newindexespos[oldindex.KeyName]; !exist {
					// drop index
					change.Indexes.Drop = append(change.Indexes.Drop, oldindex)
				} else {
					// alter index
					if oldindex.Equal(newindexes[pos]) {
						continue
					}
					change.Indexes.Drop = append(change.Indexes.Drop, oldindex)
					change.Indexes.Add = append(change.Indexes.Add, newindexes[pos])
				}
			}
			for _, newindex := range newindexes {
				if _, exist := oldindexespos[newindex.KeyName]; !exist {
					// add index
					change.Indexes.Add = append(change.Indexes.Add, newindex)
				}
			}

			newfields := newtablefields[newdetail.Name]
			newfieldspos := newtablefieldspos[newdetail.Name]
			oldfields := oldtablefields[olddetail.Name]
			oldfieldspos := oldtablefieldspos[olddetail.Name]

			for _, oldfield := range oldfields {
				if pos, exist := newfieldspos[oldfield.Field]; !exist {
					// drop field
					change.Fields.Drop = append(change.Fields.Drop, oldfield)
				} else {
					// alter field
					if oldfield.Equal(newfields[pos]) {
						continue
					}
					change.Fields.Change = append(change.Fields.Change, newfields[pos])
				}
			}

			for _, newfield := range newfields {
				if _, exist := oldfieldspos[newfield.Field]; !exist {
					// add field
					change.Fields.Add = append(change.Fields.Add, newfield)
				}
			}

			if !change.IsEmpty() {
				result.Change = append(result.Change, change)
			}
		}
	}

	return &result, nil
}

func (d *Driver) generateDrop(table Table, shardNum int) (string, []string) {
	dropSql := "DROP TABLE IF EXISTS `%s`"
	return getTableShardSql(dropSql, table.Name, shardNum)
}

func (d *Driver) generateCreate(table Table, shardNum int) (string, []string) {
	createSql := "CREATE TABLE IF NOT EXISTS `%s` ("
	fieldstr := make([]string, 0)
	partitionStr := ""
	for _, field := range table.Fields.Create {
		fieldstr = append(fieldstr, "`"+field.Field+"` "+field.Type+
			sqlnull(field.Null)+sqldefault(field.Type, field.Default)+sqlextra(field.Extra)+sqlcomment(field.Comment))
		if strings.LastIndex(field.Comment, "PARTITION") >= 0 {
			i := strings.LastIndex(field.Comment, "PARTITION")
			strs := strings.Split(field.Comment[i:], ":")
			strs1 := strings.Split(strs[1], "|")
			pm := strs1[0] // 分区方法
			pn := strs1[1] // 分区数量
			partitionStr = "PARTITION BY " + pm + " (" + field.Field + ") " + "PARTITIONS " + pn
		}
	}
	for _, index := range table.Indexes.Create {
		if index.KeyName == "PRIMARY" {
			fieldstr = append(fieldstr, " PRIMARY KEY (`"+strings.Join(index.ColumnName, "`, `")+"`)")
		} else {
			fieldstr = append(fieldstr, sqluniq(index.NonUnique)+" `"+index.KeyName+"` (`"+strings.Join(index.ColumnName, "`, `")+"`)")
		}
	}
	chars := strings.Split(table.Collation, "_")
	createSql += strings.Join(fieldstr, ", ") + ") ENGINE = " + table.Engine + " DEFAULT CHARSET = " + chars[0]
	if partitionStr != "" {
		createSql += " " + partitionStr
	}
	createSql += ";"

	return getTableShardSql(createSql, table.Name, shardNum)
}

func (d *Driver) generateAlter(table Table, shardNum int) ([]string, []string) {
	noShardSqls := make([]string, 0)
	shardSqls := make([]string, 0)
	if table.Engine != "" || table.RowFormat != "" || table.Comment != "" || table.Collation != "" {
		// table structure has changed
		// sql := "ALTER TABLE `" + table.Name + "`"
		alterSql := "ALTER TABLE `%s`"
		alterSql += " ENGIINE = '" + table.Engine + "'"
		alterSql += " ROWFORMAT = '" + table.RowFormat + "'"
		alterSql += " COMMENT = '" + table.Comment + "'"
		chars := strings.Split(table.Collation, "_")
		alterSql += " DEFAULT CHARACTER SET " + chars[0] + " COLLATE " + table.Collation + ";"

		tmpNoShardSql, tmpShardSqls := getTableShardSql(alterSql, table.Name, shardNum)
		noShardSqls = append(noShardSqls, tmpNoShardSql)
		shardSqls = append(shardSqls, tmpShardSqls...)
	}
	for _, index := range table.Indexes.Drop {
		if index.KeyName == "PRIMARY" {
			// sqls = append(sqls, "ALTER TABLE `"+index.Table+"` DROP PRIMARY KEY;")
			alterSql := "ALTER TABLE `" + "%v" + "` DROP PRIMARY KEY;"
			tmpNoShardSql, tmpShardSqls := getTableShardSql(alterSql, table.Name, shardNum)
			noShardSqls = append(noShardSqls, tmpNoShardSql)
			shardSqls = append(shardSqls, tmpShardSqls...)
		} else {
			// sqls = append(sqls, "ALTER TABLE `"+index.Table+"` DROP INDEX `"+index.KeyName+"`;")
			alterSql := "ALTER TABLE `" + "%v" + "` DROP INDEX `" + index.KeyName + "`;"
			tmpNoShardSql, tmpShardSqls := getTableShardSql(alterSql, table.Name, shardNum)
			noShardSqls = append(noShardSqls, tmpNoShardSql)
			shardSqls = append(shardSqls, tmpShardSqls...)
		}
	}

	for _, field := range table.Fields.Drop {
		// sqls = append(sqls, "ALTER TABLE `"+table.Name+"` DROP `"+field.Field+"`;")
		alterSql := "ALTER TABLE `" + "%v" + "` DROP `" + field.Field + "`;"
		tmpNoShardSql, tmpShardSqls := getTableShardSql(alterSql, table.Name, shardNum)
		noShardSqls = append(noShardSqls, tmpNoShardSql)
		shardSqls = append(shardSqls, tmpShardSqls...)
	}

	for _, field := range table.Fields.Add {
		// sqls = append(sqls, "ALTER TABLE `"+table.Name+"` ADD `"+field.Field+"` "+field.Type+
		// 	sqlcol(field.Collation)+sqlnull(field.Null)+sqldefault(field.Type, field.Default)+
		// 	sqlextra(field.Extra)+sqlcomment(field.Comment)+after(field.After)+";")

		alterSql := "ALTER TABLE `" + "%v" + "` ADD `" + field.Field + "` " + field.Type +
			sqlcol(field.Collation) + sqlnull(field.Null) + sqldefault(field.Type, field.Default) +
			sqlextra(field.Extra) + sqlcomment(field.Comment) + after(field.After) + ";"
		tmpNoShardSql, tmpShardSqls := getTableShardSql(alterSql, table.Name, shardNum)
		noShardSqls = append(noShardSqls, tmpNoShardSql)
		shardSqls = append(shardSqls, tmpShardSqls...)
	}

	for _, field := range table.Fields.Change {
		// sqls = append(sqls, "ALTER TABLE `"+table.Name+"` CHANGE `"+field.Field+"` `"+field.Field+"` "+
		// 	field.Type+sqlcol(field.Collation)+sqlnull(field.Null)+sqldefault(field.Type, field.Default)+
		// 	sqlextra(field.Extra)+sqlcomment(field.Comment)+";")
		alterSql := "ALTER TABLE `" + "%v" + "` CHANGE `" + field.Field + "` `" + field.Field + "` " +
			field.Type + sqlcol(field.Collation) + sqlnull(field.Null) + sqldefault(field.Type, field.Default) +
			sqlextra(field.Extra) + sqlcomment(field.Comment) + ";"
		tmpNoShardSql, tmpShardSqls := getTableShardSql(alterSql, table.Name, shardNum)
		noShardSqls = append(noShardSqls, tmpNoShardSql)
		shardSqls = append(shardSqls, tmpShardSqls...)
	}

	for _, index := range table.Indexes.Add {
		if index.KeyName == "PRIMARY" {
			// sqls = append(sqls, "ALTER TABLE `"+index.Table+"` ADD PRIMARY KEY (`"+strings.Join(index.ColumnName, "`, `")+"`);")
			alterSql := "ALTER TABLE `" + "%v" + "` ADD PRIMARY KEY (`" + strings.Join(index.ColumnName, "`, `") + "`);"
			tmpNoShardSql, tmpShardSqls := getTableShardSql(alterSql, table.Name, shardNum)
			noShardSqls = append(noShardSqls, tmpNoShardSql)
			shardSqls = append(shardSqls, tmpShardSqls...)
		} else {
			// sqls = append(sqls, "ALTER TABLE `"+index.Table+"` ADD "+sqluniq(index.NonUnique)+" `"+index.KeyName+"` (`"+
			// 	strings.Join(index.ColumnName, "`, `")+"`);")
			alterSql := "ALTER TABLE `" + "%v" + "` ADD " + sqluniq(index.NonUnique) + " `" + index.KeyName + "` (`" +
				strings.Join(index.ColumnName, "`, `") + "`);"
			tmpNoShardSql, tmpShardSqls := getTableShardSql(alterSql, table.Name, shardNum)
			noShardSqls = append(noShardSqls, tmpNoShardSql)
			shardSqls = append(shardSqls, tmpShardSqls...)
		}
	}

	return noShardSqls, shardSqls
}

func getTableShardSql(sqlFormat string, tableName string, shardNum int) (string, []string) {
	tmpSqls := make([]string, 0, shardNum)
	if shardNum <= 1 {
		tmpSqls = append(tmpSqls, fmt.Sprintf(sqlFormat, tableName))
	} else {
		for i := 0; i < shardNum; i++ {
			tmpSqls = append(tmpSqls, fmt.Sprintf(sqlFormat, getTableShardName(tableName, i)))
		}
	}

	return fmt.Sprintf(sqlFormat, tableName), tmpSqls
}

func getTableShardName(tableName string, shardNo int) string {
	return fmt.Sprintf("%v_%v", tableName, shardNo)
}

func (d *Driver) Generate(result *Result, shardInfo map[string]int) (map[string][]string, map[string][]string) {
	noShardSqls := make(map[string][]string)
	shardSqls := make(map[string][]string)

	addF := func(table Table, tmpNoShardSqls []string, tmpShardSqls []string) {
		list, find := noShardSqls[table.Name]
		if !find {
			list = make([]string, 0)
		}
		list = append(list, tmpNoShardSqls...)
		noShardSqls[table.Name] = list

		list1, find := shardSqls[table.Name]
		if !find {
			list1 = make([]string, 0)
		}
		list1 = append(list1, tmpShardSqls...)
		shardSqls[table.Name] = list1
	}
	for _, table := range result.Drop {
		tmpNoShard, tmpShard := d.generateDrop(table, shardInfo[table.Name])
		addF(table, []string{tmpNoShard}, tmpShard)
	}
	for _, table := range result.Create {
		tmpNoShard, tmpShard := d.generateCreate(table, shardInfo[table.Name])
		addF(table, []string{tmpNoShard}, tmpShard)
	}
	for _, table := range result.Change {
		tmpNoShard, tmpShard := d.generateAlter(table, shardInfo[table.Name])
		addF(table, tmpNoShard, tmpShard)
	}
	return noShardSqls, shardSqls
}

func getShardTablesResult(sqls map[string][]string, format string, tableOriginName string, shard map[string]int) {
	results := getShardTableResult(format, tableOriginName, shard)
	old, find := sqls[tableOriginName]
	if !find {
		sqls[tableOriginName] = results
	} else {
		sqls[tableOriginName] = append(old, results...)
	}
}

func getShardTableResult(format string, tableOriginName string, shard map[string]int) []string {
	shardTablesName := getShardTableName(tableOriginName, shard)
	list := make([]string, 0, len(shardTablesName))
	for _, v := range shardTablesName {
		list = append(list, fmt.Sprintf(format, v))
	}
	return list
}

func getShardTableName(tableOriginName string, shard map[string]int) []string {
	shardNum, find := shard[tableOriginName]
	if !find {
		return []string{tableOriginName}
	}

	if shardNum <= 1 {
		return []string{tableOriginName}
	}

	list := make([]string, 0, shardNum)
	for i := 0; i < shardNum; i++ {
		list = append(list, fmt.Sprintf("%v_%v", tableOriginName, i))
	}
	return list
}

func tables(db *sql.DB, prefix string) ([]Table, map[string]int, error) {
	query := "SHOW TABLE STATUS;"
	if prefix != "" {
		query = "SHOW TABLE STATUS LIKE '" + prefix + "%';"
	}
	resultrows, err := db.Query(query)
	if err != nil {
		return nil, nil, err
	}
	defer resultrows.Close()
	tablespos := make(map[string]int)
	tables := make([]Table, 0)
	for resultrows.Next() {

		var (
			name            string
			engine          string
			version         string
			row_format      string
			rows            int64
			avg_row_length  int64
			data_length     int64
			max_data_length int64
			index_length    int64
			data_free       int64
			auto_increment  *int64
			create_time     *string
			update_time     *string
			check_time      *string
			collection      string
			checksum        *string
			create_options  string
			comment         string
		)
		if err := resultrows.Scan(&name, &engine, &version, &row_format, &rows, &avg_row_length, &data_length, &max_data_length, &index_length, &data_free, &auto_increment, &create_time, &update_time, &check_time, &collection, &checksum, &create_options, &comment); err != nil {
			return nil, nil, err
		}
		tables = append(tables, Table{
			Name:      name,
			Engine:    engine,
			Version:   version,
			RowFormat: row_format,
			Options:   create_options,
			Comment:   comment,
			Collation: collection,
		})
		tablespos[name] = len(tables) - 1
	}
	return tables, tablespos, nil
}

func fields(db *sql.DB, table string) ([]Field, map[string]int, error) {
	resultrows, err := db.Query("SHOW FULL FIELDS FROM `" + table + "`;")
	if err != nil {
		return nil, nil, err
	}
	defer resultrows.Close()
	fieldspos := make(map[string]int)
	fields := make([]Field, 0)
	lastfield := ""
	for resultrows.Next() {
		var (
			field      string
			typ        string
			collation  *string
			null       string
			key        string
			def        *string
			extra      string
			privileges string
			comment    string
		)
		if err := resultrows.Scan(&field, &typ, &collation, &null, &key, &def, &extra, &privileges, &comment); err != nil {
			return nil, nil, err
		}
		fields = append(fields, Field{
			Field:     field,
			Type:      typ,
			Collation: collation,
			Null:      null,
			Key:       key,
			Default:   def,
			Extra:     extra,
			Comment:   comment,
			After:     lastfield,
		})
		fieldspos[field] = len(fields) - 1
		lastfield = field
	}
	return fields, fieldspos, nil
}

func indexes(db *sql.DB, table string) ([]Index, map[string]int, error) {
	resultrows, err := db.Query("SHOW INDEX FROM `" + table + "`;")
	if err != nil {
		return nil, nil, err
	}
	defer resultrows.Close()
	columns, err := resultrows.Columns()
	if err != nil {
		return nil, nil, err
	}
	column_len := len(columns)
	if column_len != 13 && column_len != 15 {
		return nil, nil, fmt.Errorf("returned %d columns while listing index", len(columns))
	}
	indexes := make([]Index, 0)
	indexpos := make(map[string]int)
	for resultrows.Next() {
		var (
			table         string
			non_unique    int
			key_name      string
			seq_in_index  int
			column_name   string
			collation     string
			cardinality   int
			sub_part      *string
			packed        *string
			null          string
			index_type    string
			comment       string
			index_comment string
			visible       string
			expression    *string
		)
		switch column_len {
		case 13:
			if err := resultrows.Scan(&table, &non_unique, &key_name, &seq_in_index, &column_name, &collation, &cardinality, &sub_part, &packed, &null, &index_type, &comment, &index_comment); err != nil {
				return nil, nil, err
			}
		case 15:
			if err := resultrows.Scan(&table, &non_unique, &key_name, &seq_in_index, &column_name, &collation, &cardinality, &sub_part, &packed, &null, &index_type, &comment, &index_comment, &visible, &expression); err != nil {
				return nil, nil, err
			}
		}

		if pos, exist := indexpos[key_name]; exist {
			indexes[pos].ColumnName = append(indexes[pos].ColumnName, column_name)
		} else {
			indexes = append(indexes, Index{
				Table:        table,
				NonUnique:    non_unique,
				KeyName:      key_name,
				ColumnName:   []string{column_name},
				Collation:    collation,
				IndexType:    index_type,
				Comment:      comment,
				IndexComment: index_comment,
			})
			indexpos[key_name] = len(indexes) - 1
		}
	}
	return indexes, indexpos, nil
}

func sqlnull(s string) string {
	switch s {
	case "NO":
		return " NOT NULL"
	case "YES":
		return " NULL"
	default:
		return ""
	}
}

func sqldefault(typ string, s *string) string {
	if s == nil {
		return ""
	}
	if strings.Contains(typ, "(") {
		typ = typ[:strings.Index(typ, "(")]
	}
	switch strings.ToLower(typ) {
	case "varchar", "char", "blob", "text", "tinyblob", "tinytext", "mediumblob", "mediumtext", "longblob", "longtext", "enum":
		return " DEFAULT '" + escape(*s) + "'"
	default:
		return " DEFAULT " + *s
	}
}

func sqlextra(s string) string {
	return " " + strings.Replace(s, "DEFAULT_GENERATED", "", -1) // mysql 8 added this extra, should ignore
}

func sqlcomment(s string) string {
	switch s {
	case "":
		return ""
	default:
		return " COMMENT '" + escape(s) + "'"
	}
}

func sqluniq(s int) string {
	if s == 0 {
		return "UNIQUE"
	}
	return "INDEX"
}

func sqlcol(s *string) string {
	switch s {
	case nil:
		return ""
	default:
		chars := strings.Split(*s, "_")
		return " CHARACTER SET " + chars[0] + " COLLATE " + *s
	}
}

func escape(s string) string {
	replacer := strings.NewReplacer(
		`\`, `\\`,
		`'`, `\'`,
	)
	return replacer.Replace(s)
}

func after(s string) string {
	if s == "" {
		return ""
	}
	return " AFTER `" + s + "`"
}
