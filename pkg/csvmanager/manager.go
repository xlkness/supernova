package csvmanager

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"sync"

	"joynova.com/library/supernova/pkg/csvmanager/csv"
	"joynova.com/library/supernova/pkg/csvmanager/utils"
)

var defaultAllTablesMetaData = map[int]*TableMetaData{}

// TableMetaData 表元数据
type TableMetaData struct {
	No              int                                      // 表序号，唯一
	St              interface{}                              // 表结构体
	File            string                                   // 文件全路径
	ExtraDataGenFun func([]interface{}) (interface{}, error) // 额外生成表数据的函数
}

func (d *TableMetaData) Register() *TableMetaData {
	defaultAllTablesMetaData[d.No] = d
	return d
}

func GetTableMetaDataByFileName(file string) *TableMetaData {
	for _, v := range defaultAllTablesMetaData {
		if v.File == file {
			return v
		}
	}
	return nil
}

type CsvManager struct {
	MetaData struct {
		Region         string
		Zones          []int
		Path           string
		TablesMetaData map[int]struct {
			Data *TableMetaData
			Lock *sync.Mutex
		}
		// AfterLoadMeta map[int]func(manager *CsvZoneManager) interface{}
		AfterLoadMeta *sync.Map
	}
	version      int
	tablesMD5    map[int]string
	zonesManager *sync.Map
	lock         *sync.Mutex
}

func SetOpenFileFunc(f func(string) (fs.File, error)) {
	utils.OpenFileFunc = f
}

// New 创建一个配置表管理器，指定region、zones、tables，不支持zones、tables动态改变
func New(region string, zones []int, path string) (*CsvManager, error) {
	return NewSpecMeta(region, zones, path, defaultAllTablesMetaData)
}

func NewSpecMeta(region string, zones []int, path string, tables map[int]*TableMetaData) (*CsvManager, error) {
	m := new(CsvManager)
	m.version = 0
	m.MetaData.Region = region
	m.MetaData.Zones = zones
	m.MetaData.Path = parseGamedataPath(path)
	m.MetaData.TablesMetaData = make(map[int]struct {
		Data *TableMetaData
		Lock *sync.Mutex
	})
	m.MetaData.AfterLoadMeta = new(sync.Map)
	for k, v := range tables {
		value := m.MetaData.TablesMetaData[k]
		value.Data = v
		value.Lock = new(sync.Mutex)
		m.MetaData.TablesMetaData[k] = value
	}
	m.tablesMD5 = make(map[int]string)
	m.zonesManager = new(sync.Map)
	m.lock = new(sync.Mutex)
	err := m.ReloadRefresh()
	return m, err
}

// RegisterAfterLoadLogic 关联多张表的到一个内存数据的逻辑
func (m *CsvManager) RegisterAfterLoadLogic(id int, f func(manager *CsvZoneManager) interface{}) {
	m.MetaData.AfterLoadMeta.Store(id, f)
}

// ReloadRefresh 重读设置状态，清空md5改变的配置表数据，等待下次使用时加载最新的数据
func (m *CsvManager) ReloadRefresh() error {
	// 计算当前所有文件的md5
	allFilesNewMD5, err := m.md5AllFiles()
	if err != nil {
		return err
	}

	newZonesManager := make([]*CsvZoneManager, 0, len(m.MetaData.Zones))

	for _, z := range m.MetaData.Zones {
		zm := newCsvZoneManager(z, m)

		// 将md5没改变的表旧数据放入
		for k, newMD5 := range allFilesNewMD5 {
			oldMD5 := m.tablesMD5[k]
			if oldMD5 == newMD5 {
				// md5一致，查找旧zone的数据
				oldZm, find := m.GetZoneCsvManager(z)
				if find {
					oldTable, find := oldZm.getTable(k)
					if find {
						zm.tables.Store(k, oldTable)
					}
				}
			}
		}
		newZonesManager = append(newZonesManager, zm)
	}

	// 防止多个重读操作并发，将以下操作原子化
	m.lock.Lock()
	defer m.lock.Unlock()

	if allFilesNewMD5 != nil {
		m.tablesMD5 = allFilesNewMD5
	}
	m.version += 1

	for _, zm := range newZonesManager {
		// 重新插入空的区服表数据数据
		zm.version = m.version
		m.zonesManager.Store(zm.Zone, zm)
	}
	return nil
}

func (m *CsvManager) GetZoneCsvManager(zone int) (*CsvZoneManager, bool) {
	zoneManager, find := m.zonesManager.Load(zone)
	if find {
		return zoneManager.(*CsvZoneManager), find
	}
	return nil, false
}

// LoadAllCsv 加载所有配置表，只用于校验
func (m *CsvManager) CheckAllCsvCanLoad(clone bool) error {
	var newM = m
	if clone {
		var err error
		newM, err = New(m.MetaData.Region, m.MetaData.Zones, m.MetaData.Path)
		if err != nil {
			return err
		}
	}
	for _, v := range newM.MetaData.TablesMetaData {
		err := newM.loadCsv(v.Data)
		if err != nil {
			return err
		}
	}
	return nil
}

// loadCsv 读取某个csv文件
func (m *CsvManager) loadCsv(table *TableMetaData) error {
	_, find := m.MetaData.TablesMetaData[table.No]
	if !find {
		return fmt.Errorf("初始化csv列表中没有table[%v]，不支持动态新增配置表", filepath.Base(table.File))
	}

	// 锁住这个表的读取事件
	l := m.MetaData.TablesMetaData[table.No].Lock
	l.Lock()
	defer l.Unlock()

	// double check
	tmpZm, find := m.GetZoneCsvManager(m.MetaData.Zones[0])
	if !find {
		return fmt.Errorf("读取表[%v]之前尝试再次检查zone是否已经读取过数据没有找到zone[%v]",
			filepath.Base(table.File), m.MetaData.Zones[0])
	}
	_, find = tmpZm.getTable(table.No)
	if find {
		// 找到配置表数据，不需要重新读取
		return nil
	}

	csvZonesData, err :=
		csv.ReadCsv(m.MetaData.Region, m.MetaData.Zones, m.MetaData.Path+table.File, table.St, table.ExtraDataGenFun)
	if err != nil {
		panic(fmt.Errorf("读取csv表[%v]错误:%v", filepath.Base(table.File), err))
	}
	for i, v := range csvZonesData {
		zm, find := m.GetZoneCsvManager(m.MetaData.Zones[i])
		if !find {
			panic(fmt.Errorf("读取csv表[%v]数据，查找当前manager区服[%v]时没找到", filepath.Base(table.File), m.MetaData.Zones[i]))
		}
		zm.tables.Store(table.No, v)
		m.zonesManager.Store(m.MetaData.Zones[i], zm)
	}
	return nil
}

// md5AllFiles 计算所有csv文件的md5
func (m *CsvManager) md5AllFiles() (map[int]string, error) {
	newMD5Map := make(map[int]string)
	for k, v := range m.MetaData.TablesMetaData {
		newMD5, _, err := utils.CheckMD5(m.MetaData.Path+v.Data.File, "")
		if err != nil {
			return nil, fmt.Errorf("校验文件[%v]md5错误:%v", filepath.Base(v.Data.File), err)
		}
		newMD5Map[k] = newMD5
	}
	return newMD5Map, nil
}

func parseGamedataPath(gameDataPath string) string {
	if gameDataPath == "" || gameDataPath == "./" || gameDataPath == "." {
		gameDataPath = ""
	} else if gameDataPath[len(gameDataPath)-1] != '/' {
		gameDataPath += "/"
	}

	return gameDataPath
}
