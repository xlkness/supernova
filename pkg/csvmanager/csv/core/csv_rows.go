package core

import (
	"fmt"
	"reflect"
	"strconv"
)

// CsvOriginRowsData 原始行数据，包含索引数据
type CsvOriginRowsData struct {
	Region       string
	Zone         int
	Rows         []interface{}
	KeyIndexData struct {
		HasKeyIndex bool
		KeyIndexMap map[interface{}]int
	}
	GroupIndexData struct {
		HasGroupIndex  bool
		GroupIndexMaps []map[interface{}][]int // 支持多个group索引
	}
	UnionGroupIndexData struct {
		HasUnionGroupIndex  bool
		UnionGroupIndexMaps map[string]map[string][]int // 支持多个联合索引
	}
	UnionUniqueIndexData struct {
		HasUnionUniqueIndex  bool
		UnionUniqueIndexMaps map[string]map[string]int // 支持多个联合唯一索引
	}
}

func (td *CsvOriginRowsData) Range(f func(interface{})) {
	for _, v := range td.Rows {
		f(v)
	}
}

func (td *CsvOriginRowsData) Record(i int) interface{} {
	if 0 <= i && i < len(td.Rows) {
		return td.Rows[i]
	}
	return nil
}

func (td *CsvOriginRowsData) NumRecord() int {
	return len(td.Rows)
}

func (td *CsvOriginRowsData) Index(key interface{}) interface{} {
	var indexKey interface{} = td.parseNumberKey(key)
	row, find := td.KeyIndexData.KeyIndexMap[indexKey]
	if find {
		return td.Rows[row]
	}
	return nil
}

// IndexGroup 索引组主键，groupSequece代表group在结构体字段里出现的顺序，从0开始
// 例如：
// type Test struct {
//     A int `csv:"a" group:"true"`
//     B int `csv:"b" group:"true"`
// }
// 要索引字段b，就IndexGroup(1, xx)
func (td *CsvOriginRowsData) IndexGroup(groupSequece int, key interface{}) []interface{} {
	var indexKey = td.parseNumberKey(key)
	lists := td.GroupIndexData.GroupIndexMaps[groupSequece]
	list, find := lists[indexKey]
	if !find {
		return nil
	}
	ret := make([]interface{}, 0, len(list))
	for _, v := range list {
		ret = append(ret, td.Rows[v])
	}
	return ret
}

// IndexUnionGroup 联合索引
// 例如：
// type Test struct {
//     A int `csv:"a" union:"logic1"`
//     B int `csv:"b" union:"logic1"`
// }
// IndexUnionGroup("logic1", 12, 23)
func (td *CsvOriginRowsData) IndexUnionGroup(group string, key ...interface{}) []interface{} {
	m, find := td.UnionGroupIndexData.UnionGroupIndexMaps[group]
	if !find {
		return nil
	}
	indexKey := ""
	for _, v := range key {
		indexKey += td.parseNumberKey2String(v)
	}
	list, find := m[indexKey]
	if !find {
		return nil
	}
	ret := make([]interface{}, 0, len(list))
	for _, v := range list {
		ret = append(ret, td.Rows[v])
	}
	return ret
}

// IndexUnionUnique 联合索引
// 例如：
// type Test struct {
//     A int `csv:"a" unionu:"logic1"`
//     B int `csv:"b" unionu:"logic1"`
// }
// IndexUnionUnique("logic1", 12, 23)
func (td *CsvOriginRowsData) IndexUnionUnique(group string, key ...interface{}) interface{} {
	m, find := td.UnionUniqueIndexData.UnionUniqueIndexMaps[group]
	if !find {
		return nil
	}
	indexKey := ""
	for _, v := range key {
		indexKey += td.parseNumberKey2String(v)
	}
	idx, find := m[indexKey]
	if !find {
		return nil
	}
	return td.Rows[idx]
}

func (td *CsvOriginRowsData) parseNumberKey2String(key interface{}) string {
	var indexKey string
	switch reflect.ValueOf(key).Kind() {
	case reflect.Int:
		indexKey = strconv.FormatInt(int64(key.(int)), 10)
	case reflect.Int32:
		indexKey = strconv.FormatInt(int64(key.(int32)), 10)
	case reflect.Int64:
		indexKey = strconv.FormatInt(key.(int64), 10)
	case reflect.String:
		indexKey = key.(string)
	case reflect.Bool:
		indexKey = strconv.FormatBool(key.(bool))
	default:
		panic(fmt.Errorf("unsupport csv key type:%v", reflect.ValueOf(key).Kind()))
	}
	return indexKey
}

func (td *CsvOriginRowsData) parseNumberKey(key interface{}) interface{} {
	var indexKey interface{}
	switch reflect.ValueOf(key).Kind() {
	case reflect.Int:
		indexKey = int64(key.(int))
	case reflect.Int32:
		indexKey = int64(key.(int32))
	case reflect.Int64:
		indexKey = key.(int64)
	case reflect.String:
		indexKey = key.(string)
	case reflect.Bool:
		indexKey = strconv.FormatBool(key.(bool))
	default:
		panic(fmt.Errorf("unsupport csv key type:%v", reflect.ValueOf(key).Kind()))
	}
	return indexKey
}

type csvRowsData struct {
	Rows         []*csvRowData // 原始数据
	KeyIndexInfo struct {
		KeyIndexColInStruct int // 主键字段在结构体的索引位置
	}
	GroupIndexesInfo struct {
		GroupIndexColsInStruct []int // 主键字段在结构体的索引位置，支持多个group索引
	}
	UnionGroupIndexesInfo struct {
		UnionGroupIndexColsInStruct map[string][]int // 联合主键在结构体的索引位置，支持多个联合索引
	} // 普通联合索引，值是个列表
	UnionUniqueIndexesInfo struct {
		UnionUniqueIndexColInStruct map[string][]int // 联合唯一主键
	}
}

// FilterWithRegionZone 原始数据按指定region、zone过滤出数据
func (rows *csvRowsData) FilterWithRegionZone(region string, zone int) (parsedRowsData *CsvOriginRowsData, err error) {

	var (
		originRows                 []interface{}
		keyIndexValuesMap          map[interface{}]int
		groupIndexValuesMaps       []map[interface{}][]int
		unionGroupIndexValuesMaps  map[string]map[string][]int
		unionUniqueIndexValuesMaps map[string]map[string]int
	)

	parsedRowsData = &CsvOriginRowsData{
		Region: region,
		Zone:   zone,
	}

	// 初始化过滤的主键数据
	if rows.KeyIndexInfo.KeyIndexColInStruct >= 0 {
		keyIndexValuesMap = make(map[interface{}]int)
		parsedRowsData.KeyIndexData.HasKeyIndex = true
	}
	if len(rows.GroupIndexesInfo.GroupIndexColsInStruct) > 0 {
		groupIndexValuesMaps = make([]map[interface{}][]int, len(rows.GroupIndexesInfo.GroupIndexColsInStruct))
		for i := range rows.GroupIndexesInfo.GroupIndexColsInStruct {
			groupIndexValuesMaps[i] = make(map[interface{}][]int)
		}
		parsedRowsData.GroupIndexData.HasGroupIndex = true
	}
	if len(rows.UnionGroupIndexesInfo.UnionGroupIndexColsInStruct) > 0 {
		unionGroupIndexValuesMaps = make(map[string]map[string][]int)
		for k := range rows.UnionGroupIndexesInfo.UnionGroupIndexColsInStruct {
			unionGroupIndexValuesMaps[k] = make(map[string][]int) // 存储具体的值对应的行
		}
		parsedRowsData.UnionGroupIndexData.HasUnionGroupIndex = true
	}
	if len(rows.UnionUniqueIndexesInfo.UnionUniqueIndexColInStruct) > 0 {
		unionUniqueIndexValuesMaps = make(map[string]map[string]int)
		for k := range rows.UnionUniqueIndexesInfo.UnionUniqueIndexColInStruct {
			unionUniqueIndexValuesMaps[k] = make(map[string]int) // 存储具体的值对应的行
		}
		parsedRowsData.UnionUniqueIndexData.HasUnionUniqueIndex = true
	}

	for originI, v := range rows.Rows {
		if v.matchRegionAndZone(region, zone) {
			originRows = append(originRows, v.GetRowData())
			rowIndex := len(originRows) - 1

			// 过滤主键
			if rows.KeyIndexInfo.KeyIndexColInStruct >= 0 {
				parsedKeyFieldValue, _ := v.getKeyColValue(rows.KeyIndexInfo.KeyIndexColInStruct)
				_, find := keyIndexValuesMap[parsedKeyFieldValue]
				if find {
					err = fmt.Errorf("唯一索引在第[%v]行出现重复值[%v]", originI+1, parsedKeyFieldValue)
					return
				}
				keyIndexValuesMap[parsedKeyFieldValue] = rowIndex
			}
			// 过滤组索引
			for i, col := range rows.GroupIndexesInfo.GroupIndexColsInStruct {
				// 取设置了group索引的列的值
				parsedKeyFieldValue, _ := v.getKeyColValue(col)
				curGroupMap := groupIndexValuesMaps[i]
				list, find := curGroupMap[parsedKeyFieldValue]
				if !find {
					list = make([]int, 0, 1)
				}
				list = append(list, rowIndex)
				curGroupMap[parsedKeyFieldValue] = list
				groupIndexValuesMaps[i] = curGroupMap
			}
			// 过滤联合普通索引
			for k, is := range rows.UnionGroupIndexesInfo.UnionGroupIndexColsInStruct {
				unionKey := ""
				for _, i := range is {
					fieldValueString := v.getKeyColString(i)
					unionKey += fieldValueString + "."
				}
				valuesM, _ := unionGroupIndexValuesMaps[k]
				list := valuesM[unionKey]
				list = append(list, rowIndex)
				valuesM[unionKey] = list
				unionGroupIndexValuesMaps[k] = valuesM
			}
			// 过滤联合唯一索引
			for k, is := range rows.UnionUniqueIndexesInfo.UnionUniqueIndexColInStruct {
				unionKey := ""
				for _, i := range is {
					fieldValueString := v.getKeyColString(i)
					unionKey += fieldValueString + "."
				}
				valuesM, _ := unionUniqueIndexValuesMaps[k]
				_, find := valuesM[unionKey]
				if find {
					err = fmt.Errorf("联合唯一索引[%v]在[%v]行出现重复值[%v]", k, originI+1, unionKey)
					return
				}
				valuesM[unionKey] = rowIndex
				unionUniqueIndexValuesMaps[k] = valuesM
			}
		}
	}

	parsedRowsData.Rows = originRows
	parsedRowsData.KeyIndexData.KeyIndexMap = keyIndexValuesMap
	parsedRowsData.GroupIndexData.GroupIndexMaps = groupIndexValuesMaps
	parsedRowsData.UnionGroupIndexData.UnionGroupIndexMaps = unionGroupIndexValuesMaps
	parsedRowsData.UnionUniqueIndexData.UnionUniqueIndexMaps = unionUniqueIndexValuesMaps

	return
}
