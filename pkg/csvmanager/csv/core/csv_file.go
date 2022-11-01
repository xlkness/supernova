package core

import (
	"encoding/csv"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"joynova.com/library/supernova/pkg/csvmanager/utils"
)

var Comma rune = '\t'
var Comment = '#'

type parser interface {
	Parse(string) error
}

// ReadCsv 读取csv文件，解析出原始行数据
func ReadCsv(fileName string, rowDataTemplateTypeRecord reflect.Type) (originRecords *csvRowsData, err error) {
	file, err := utils.OpenFileFunc(fileName)
	if err != nil {
		return nil, fmt.Errorf("打开文件[%v]错误:%v", fileName, err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.Comma = Comma
	reader.Comment = Comment

	_fieldTypeRow, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("读取文件[%v]字段类型行错误:%v", fileName, err)
	}

	if _fieldTypeRow == nil {
		// 字段类型没有用
	}

	fieldNameRow, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("读取文件[%v]字段名行错误:%v", fileName, err)
	}

	dataRows, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("读取文件[%v]所有数据行错误:%v", fileName, err)
	}

	rows, err := parseOriginFileData(fieldNameRow, dataRows, rowDataTemplateTypeRecord)
	if err != nil {
		return nil, fmt.Errorf("转换文件[%v]数据错误:%v", fileName, err)
	}
	return rows, nil
}

// parseOriginFileData 以表头、原始行数据、csv结构体描述信息解析数据
func parseOriginFileData(fieldsNameRowInFile []string, dataRows [][]string,
	rowDataTemplateTypeRecord reflect.Type) (originRecords *csvRowsData, err error) {
	if err := checkStFieldKind(rowDataTemplateTypeRecord); err != nil {
		return nil, fmt.Errorf("校验代码结构体错误:%v", err)
	}

	if err := checkFileFiedlsName(fieldsNameRowInFile); err != nil {
		return nil, fmt.Errorf("校验文件字段错误:%v", err)
	}

	originRecords = &csvRowsData{}

	// 遍历每一行数据解析
	templateRecord := &templateTypeRecord{rowDataTemplateTypeRecord}

	keyColIndexInStruct := templateRecord.getKeyColIndex()
	groupColIndexInStruct := templateRecord.getGroupKeyColsIndex()
	unionGroupColIndexInStruct := templateRecord.getUnionGroupKeyColsIndex()
	unionUniqueColIndexInStruct := templateRecord.getUnionUniqueGroupKeyColsIndex()

	originRecords.KeyIndexInfo.KeyIndexColInStruct = keyColIndexInStruct
	originRecords.GroupIndexesInfo.GroupIndexColsInStruct = groupColIndexInStruct
	originRecords.UnionGroupIndexesInfo.UnionGroupIndexColsInStruct = unionGroupColIndexInStruct
	originRecords.UnionUniqueIndexesInfo.UnionUniqueIndexColInStruct = unionUniqueColIndexInStruct

	for i, dataRow := range dataRows {
		csvRowData, err := originRow(dataRow).parse(fieldsNameRowInFile, templateRecord)
		if err != nil {
			return nil, fmt.Errorf("第[%v]行数据转换为代码结构体错误，详情:%v", i+1, err)
		}

		originRecords.Rows = append(originRecords.Rows, csvRowData)
	}
	return
}

// setValue 设置结构体一个字段的值
func setValue(field reflect.Value, value string) error {
	if field.Kind() == reflect.Ptr {
		if value == "" {
			return nil
		}

		if field.IsNil() {
			field.Set(reflect.New(field.Type().Elem()))
		}
		field = field.Elem()
	}
	switch field.Kind() {
	case reflect.String:
		field.SetString(value)
	case reflect.Bool:
		b, err := strconv.ParseBool(value)
		if err != nil {
			return err
		}
		field.SetBool(b)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if value == "" {
			field.SetInt(0)
		} else {
			i, err := strconv.ParseInt(value, 0, field.Type().Bits())
			if err != nil {
				return err
			}
			field.SetInt(i)
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		ui, err := strconv.ParseUint(value, 0, field.Type().Bits())
		if err != nil {
			return err
		}
		field.SetUint(ui)
	case reflect.Float32, reflect.Float64:
		if value == "" {
			field.SetFloat(0)
			break
		}
		f, err := strconv.ParseFloat(value, field.Type().Bits())
		if err != nil {
			return err
		}
		field.SetFloat(f)
	case reflect.Struct:
		field = field.Addr()
		parser, ok := field.Interface().(parser)
		if !ok {
			return fmt.Errorf("struct no implement Parser")
		}
		err := parser.Parse(value)
		if err != nil {
			return fmt.Errorf("parse struct[%v] err: %s", field.Type(), err)
		}
	case reflect.Slice:
		values := strings.Split(value, ",")
		if len(values) == 1 && values[0] == "" {
			values = []string{}
		}
		field.Set(reflect.MakeSlice(field.Type(), len(values), len(values)))
		for i := 0; i < len(values); i++ {
			err := setValue(field.Index(i), values[i])
			if err != nil {
				return err
			}
		}

	default:
		return fmt.Errorf("no support type %s", field.Type())
	}
	return nil
}
