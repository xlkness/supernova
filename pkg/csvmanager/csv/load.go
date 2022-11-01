package csv

import (
	"fmt"
	"path/filepath"
	"reflect"

	"joynova.com/library/supernova/pkg/csvmanager/csv/core"
)

func ReadCsv(region string, zones []int, file string, templateTypeRecord interface{},
	extraLoadFun func([]interface{}) (interface{}, error)) ([]*CsvTable, error) {
	csvOriginData, err := core.ReadCsv(file, reflect.TypeOf(templateTypeRecord))
	if err != nil {
		return nil, err
	}

	list := make([]*CsvTable, 0, len(zones))
	for _, zone := range zones {
		csvOriginParsedData, err := csvOriginData.FilterWithRegionZone(region, zone)
		if err != nil {
			return nil, err
		}

		csvTable := newCsvTable(file, region, zone)
		csvTable.CsvOriginRowsData = csvOriginParsedData

		if extraLoadFun != nil {
			extraData, err := extraLoadFun(csvOriginParsedData.Rows)
			if err != nil {
				return nil, fmt.Errorf("表[%v]加载额外数据错误:%v", filepath.Base(file), err)
			}
			csvTable.extraData = extraData
		}

		list = append(list, csvTable)
	}

	return list, nil
}

func newCsvTable(file string, region string, zone int) *CsvTable {
	table := new(CsvTable)
	table.MetaInfo.FileName = filepath.Base(file)
	table.MetaInfo.FullPathFileName = file
	table.MetaInfo.Region = region
	table.MetaInfo.Zone = zone
	return table
}

func (cr *CsvTable) GetExtraData() interface{} {
	return cr.extraData
}
