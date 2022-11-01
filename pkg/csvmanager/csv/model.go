package csv

import (
	"joynova.com/library/supernova/pkg/csvmanager/csv/core"
)

var Comma = '\t'
var Comment = '#'

type Parser interface {
	Parse(string) error
}

type DataChecker interface {
	// Check 检查单行数据，omit为true表示读取时忽略这行数据，err不为空整个加载过程报错
	CheckOrParse() (omit bool, err error)
}

type CsvTable struct {
	MetaInfo struct {
		Region           string
		Zone             int
		FileName         string
		FullPathFileName string
	}
	*core.CsvOriginRowsData
	extraData interface{} // 额外数据
}
