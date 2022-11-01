package mysql

import (
	"encoding/json"
	"reflect"
)

type LogWriteField struct {
	Field    string
	OldValue interface{}
	NewValue interface{}
	Delta    interface{}
}

type LogWhere struct {
	Key    string
	Assign string
	Value  interface{}
}

type LogWriteOp struct {
	RoleID         int64
	Table          string
	Op             string
	InsertRow      LogRowData
	WhereKeyValues []*LogWhere
	UpdateFields   []*LogWriteField
	AffectedRows   int
}

func NewRoleLogInsertOp(RoleID int64, Table string) *LogWriteOp {
	return &LogWriteOp{RoleID: RoleID, Op: "insert", AffectedRows: 1, Table: Table}
}

func NewRoleLogUpdateOneOp(RoleID int64, Table string) *LogWriteOp {
	return &LogWriteOp{RoleID: RoleID, Op: "update", AffectedRows: 1, Table: Table}
}

func NewRoleLogUpdateMultiOp(RoleID int64, Table string, affectedRows int) *LogWriteOp {
	return &LogWriteOp{RoleID: RoleID, Op: "update", AffectedRows: affectedRows, Table: Table}
}

func (wo *LogWriteOp) AppendWhere(key string, assign string, value interface{}) *LogWriteOp {
	wo.WhereKeyValues = append(wo.WhereKeyValues, &LogWhere{Key: key, Assign: assign, Value: value})
	return wo
}

func (wo *LogWriteOp) AppendWhereEq(key string, value interface{}) *LogWriteOp {
	wo.WhereKeyValues = append(wo.WhereKeyValues, &LogWhere{Key: key, Assign: "=", Value: value})
	return wo
}

func (wo *LogWriteOp) AppendWhereGt(key string, value interface{}) *LogWriteOp {
	wo.WhereKeyValues = append(wo.WhereKeyValues, &LogWhere{Key: key, Assign: ">", Value: value})
	return wo
}

func (wo *LogWriteOp) AppendWhereGe(key string, value interface{}) *LogWriteOp {
	wo.WhereKeyValues = append(wo.WhereKeyValues, &LogWhere{Key: key, Assign: ">=", Value: value})
	return wo
}

func (wo *LogWriteOp) AppendWhereLt(key string, value interface{}) *LogWriteOp {
	wo.WhereKeyValues = append(wo.WhereKeyValues, &LogWhere{Key: key, Assign: "<", Value: value})
	return wo
}

func (wo *LogWriteOp) AppendWhereLe(key string, value interface{}) *LogWriteOp {
	wo.WhereKeyValues = append(wo.WhereKeyValues, &LogWhere{Key: key, Assign: "<=", Value: value})
	return wo
}

type LogRowData interface {
	String() string
}

func (wo *LogWriteOp) AppendInsert(row LogRowData) *LogWriteOp {
	wo.InsertRow = row
	return wo
}

func (wo *LogWriteOp) AppendUpdate(field string, oldValue, newValue interface{}) *LogWriteOp {
	writeField := &LogWriteField{Field: field, OldValue: oldValue, NewValue: newValue}
	ov := reflect.ValueOf(oldValue)
	switch ov.Kind() {
	case reflect.Int:
		writeField.Delta = newValue.(int) - oldValue.(int)
	case reflect.Int64:
		writeField.Delta = newValue.(int64) - oldValue.(int64)
	case reflect.Float32:
		writeField.Delta = newValue.(float32) - oldValue.(float32)
	case reflect.Float64:
		writeField.Delta = newValue.(float64) - oldValue.(float64)
	case reflect.Ptr:
		data, _ := json.Marshal(oldValue)
		writeField.OldValue = string(data)
		data1, _ := json.Marshal(newValue)
		writeField.NewValue = string(data1)
	case reflect.Struct:
		data, _ := json.Marshal(&oldValue)
		writeField.OldValue = string(data)
		data1, _ := json.Marshal(&newValue)
		writeField.NewValue = string(data1)
	case reflect.Map:

	}
	wo.UpdateFields = append(wo.UpdateFields, writeField)
	return wo
}

func (wo *LogWriteOp) Affected(aff int) *LogWriteOp {
	wo.AffectedRows = aff
	return wo
}

func (wo *LogWriteOp) Map() map[string]map[string]string {
	return nil
}

func (wo *LogWriteOp) String() string {
	if wo == nil {
		return "{write_log: null}"
	}
	data, _ := json.Marshal(wo)
	return string(data)
}
