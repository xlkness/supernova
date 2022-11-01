package core

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

type DataChecker interface {
	// Check 检查单行数据，omit为true表示读取时忽略这行数据，err不为空整个加载过程报错
	CheckOrParse() (omit bool, err error)
}

type originRow []string

func (r originRow) parse(fieldsNameRowInFile []string, templateRecord *templateTypeRecord) (*csvRowData, error) {
	matchRegions := r.getMatchRegions(fieldsNameRowInFile)
	matchZones, err := r.getMatchZones(fieldsNameRowInFile)
	if err != nil {
		return nil, err
	}
	structRowData, err := r.parseRowValuesToStruct(templateRecord.to, fieldsNameRowInFile)
	if err != nil {
		return nil, err
	}

	c, ok := structRowData.Interface().(DataChecker)
	if ok {
		c.CheckOrParse()
	}

	rowData := &csvRowData{
		RowData:      structRowData,
		MatchRegions: matchRegions,
		MatchZones:   matchZones,
	}
	return rowData, nil
}

func (r originRow) getMatchRegions(fieldsNameRow []string) []string {
	for i := range fieldsNameRow {
		if fieldsNameRow[i] == "region" {
			regionsString := r[i]
			if regionsString == "" {
				return nil
			}
			return strings.Split(regionsString, ",")
		}
	}
	return nil
}

func (r originRow) getMatchZones(fieldsNameRow []string) (list []int, err error) {
	for i := range fieldsNameRow {
		if fieldsNameRow[i] == "zone_id" {
			zonesString := r[i]
			if zonesString == "" {
				break
			}
			strs := strings.Split(zonesString, ",")
			list = make([]int, 0, len(strs))
			for _, zoneStr := range strs {
				zone, err := strconv.Atoi(zoneStr)
				if err != nil {
					return nil, fmt.Errorf("zone必须为数字分隔:%+v", zonesString)
				}
				list = append(list, zone)
			}
			break
		}
	}
	return
}

// setRowValuesToStruct 将一行数据怼给对应的结构体数据返回出去
func (r originRow) parseRowValuesToStruct(typeRecord reflect.Type, fieldsNameRowInFile []string) (reflect.Value, error) {
	dataRow := []string(r)

	if len(dataRow) != len(fieldsNameRowInFile) {
		return reflect.Value{}, fmt.Errorf("字段数量与表头数不一致:%v,%v", len(dataRow), len(fieldsNameRowInFile))
	}

	// new一个表数据结构体
	value := reflect.New(typeRecord)
	record := value.Elem()

	// 给每个字段赋值
	for i := 0; i < typeRecord.NumField(); i++ {
		f := typeRecord.Field(i)
		fieldTagName := f.Tag.Get("csv")
		if fieldTagName == "" {
			continue
		}
		if fieldTagName == "_" {
			continue
		}

		findFieldColInFile := -1
		for iInFile, fieldNameInFile := range fieldsNameRowInFile {
			if fieldTagName == fieldNameInFile {
				// 结构体字段在文件也定义了
				findFieldColInFile = iInFile
				break
			}
		}

		if findFieldColInFile < 0 {
			return reflect.Value{},
				fmt.Errorf("代码和配置表不匹配，表结构体字段[%v,%s]没有在配置表找到:%+v", f.Name, f.Tag, fieldsNameRowInFile)
		}

		fieldStr := dataRow[findFieldColInFile]

		field := record.Field(i)
		if !field.CanSet() {
			continue
		}

		err := setValue(field, fieldStr)
		if err != nil {
			return reflect.Value{},
				fmt.Errorf("字段[%v]，值[%v]转换失败:%v",
					fieldTagName, fieldStr, err)
		}
	}

	return value, nil
}

type csvRowData struct {
	RowData      reflect.Value // 原始行数据
	MatchRegions []string      // 匹配的region
	MatchZones   []int         // 匹配的区服
}

func (row *csvRowData) GetRowData() interface{} {
	return row.RowData.Interface()
}

func (row *csvRowData) GetColValue(col int) reflect.Value {
	return row.RowData.Elem().Field(col)
}

func (row *csvRowData) getKeyColValue(col int) (interface{}, reflect.Value) {
	keyFieldValue := row.GetColValue(col)
	var parsedKeyFieldValue interface{}
	switch keyFieldValue.Kind() {
	case reflect.Int:
		parsedKeyFieldValue = int64(keyFieldValue.Interface().(int))
	case reflect.Int32:
		parsedKeyFieldValue = int64(keyFieldValue.Interface().(int32))
	case reflect.Int64:
		parsedKeyFieldValue = keyFieldValue.Interface().(int64)
	default:
		parsedKeyFieldValue = keyFieldValue.Interface()
	}
	return parsedKeyFieldValue, keyFieldValue
}

func (row *csvRowData) getKeyColString(col int) string {
	keyFieldValue := row.GetColValue(col)
	var parsedKeyFieldValue string
	switch keyFieldValue.Kind() {
	case reflect.Int:
		parsedKeyFieldValue = strconv.FormatInt(int64(keyFieldValue.Interface().(int)), 10)
	case reflect.Int32:
		parsedKeyFieldValue = strconv.FormatInt(int64(keyFieldValue.Interface().(int32)), 10)
	case reflect.Int64:
		parsedKeyFieldValue = strconv.FormatInt(keyFieldValue.Interface().(int64), 10)
	case reflect.Bool:
		if keyFieldValue.Interface().(bool) {
			parsedKeyFieldValue = "true"
		} else {
			parsedKeyFieldValue = "false"
		}
	case reflect.String:
		parsedKeyFieldValue = keyFieldValue.Interface().(string)
	default:
		panic(fmt.Errorf("unsupport csv type:%v", keyFieldValue.Kind()))
	}
	return parsedKeyFieldValue
}

func (row *csvRowData) matchRegionAndZone(region string, zone int) bool {
	canMatch := false
	if len(row.MatchRegions) <= 0 {
		// 匹配所有region
		canMatch = true
	} else {
		for _, curRegion := range row.MatchRegions {
			if curRegion == region {
				canMatch = true
				break
			}
		}
	}
	if canMatch {
		canMatch = false
		if len(row.MatchZones) <= 0 {
			canMatch = true
		} else {
			for _, curZone := range row.MatchZones {
				if curZone == zone {
					canMatch = true
					break
				}
			}
		}
	}
	return canMatch
}
