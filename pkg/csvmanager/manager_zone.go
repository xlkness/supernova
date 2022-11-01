package csvmanager

import (
	"sync"

	"joynova.com/library/supernova/pkg/csvmanager/csv"
)

func newCsvZoneManager(zone int, csvM *CsvManager) *CsvZoneManager {
	m := new(CsvZoneManager)
	m.Zone = zone
	m.tables = new(sync.Map)
	m.extraMultiTablesJoinData = new(sync.Map)
	m.csvManager = csvM
	return m
}

type CsvZoneManager struct {
	version                  int
	Region                   string
	Zone                     int
	tables                   *sync.Map
	extraMultiTablesJoinData *sync.Map
	// 存储父类，读取配置表时可能触发重读，去调用父类重读
	csvManager *CsvManager
}

// GetTable 获取一个配置表数据，如果找不到，会进行csv文件读取，然后刷新所有区服当前表数据
func (m *CsvZoneManager) GetTable(table *TableMetaData) (tableData *csv.CsvTable, triggerLoad bool, find bool) {
	data, find := m.getTable(table.No)
	if find {
		return data, false, find
	}

	curM, find := m.csvManager.GetZoneCsvManager(m.Zone)
	if !find {
		return data, false, find
	}

	if m.version < curM.version {
		// 说明当前区服配置表管理器是旧版，且旧版还没有读取过当前表，使用最新版本的数据
		// NOTE:
		// 如果一个逻辑中使用到配置表a、b，用完a触发重读，逻辑再读取b，则a、b版本不一致，
		// 可能出现数据不一致，但这种一个逻辑中的执行很快，可以不考虑数据不一致的情况，
		// 且如果要做强一致必须读a、b都作为原子操作，实操比较复杂
		data, find := curM.getTable(table.No)
		return data, false, find
	}

	err := m.csvManager.loadCsv(table)
	if err != nil {
		return nil, true, false
	}

	// 重读之后再次读取
	data, find = m.getTable(table.No)
	return data, true, find
}

func (m *CsvZoneManager) GetAfterLoadData(id int) (interface{}, bool) {
	data, find := m.extraMultiTablesJoinData.Load(id)
	if !find {
		loadFun, find := m.csvManager.MetaData.AfterLoadMeta.Load(id)
		if !find {
			return nil, false
		}
		data = loadFun.(func(manager *CsvZoneManager) interface{})(m)
		m.extraMultiTablesJoinData.Store(id, data)
	}
	return data, true
}

func (m *CsvZoneManager) getTable(no int) (*csv.CsvTable, bool) {
	d, find := m.tables.Load(no)
	if find {
		return d.(*csv.CsvTable), find
	}
	return nil, find
}
