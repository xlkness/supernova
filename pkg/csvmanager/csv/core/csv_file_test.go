package core

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

type RewardItem struct {
	ItemID int32
	Amount int32
}

func (self *RewardItem) Parse(text string) error {
	strs := strings.Split(text, ":")
	var err error
	var itemID, amount int
	if len(strs) != 2 {
		err = fmt.Errorf("reward item config[%v] format err",
			text)
		return err
	}
	itemID, err = strconv.Atoi(strs[0])
	self.ItemID = int32(itemID)
	if err != nil {
		return err
	}

	amount, err = strconv.Atoi(strs[1])
	self.Amount = int32(amount)
	if err != nil {
		return err
	}
	return nil
}

type ChargeConfigData struct {
	Type        int32         `csv:"type" index:"true"`
	ChargeID    string        `csv:"charge_id" group:"true" union:"charge"`
	Diamond     []*RewardItem `csv:"diamond"`
	MailGift    string        `csv:"mail_gift" union:"charge"`
	MailID      int32         `csv:"_"`
	MailItems   string        `csv:"_"`
	VipExp      int32         `csv:"vip_exp"`
	LimitNum    int32         `csv:"limit_num"`
	Cycle       int32         `csv:"cycle"`
	CardID      int32         `csv:"privilege_id" group:"true"`
	TimeLimit   int32         `csv:"times_max"`
	ShopData    string        `csv:"shop_data"`
	FirstCharge int32         `csv:"first_charge"`
	ShopTime    string        `csv:"shop_time" unionu:"charge"`
	OfflineMail string        `csv:"offline_mail" unionu:"charge"`
	// DTKJPrice   string        `csv:"dtkj_price"`
	// NoviceID int32 `csv:"novice_id"`
}

func loadFile(t *testing.T) {
	rows, err := ReadCsv("./charge_config.csv", reflect.TypeOf(ChargeConfigData{}))
	if err != nil {
		panic(err)
	}
	fmt.Printf("index:%+v,%+v\n", rows.KeyIndexInfo.KeyIndexColInStruct, rows.GroupIndexesInfo.GroupIndexColsInStruct)
	for _, v := range rows.Rows {
		fmt.Printf("--------------------------------\n")
		fmt.Printf("%v\n", v.MatchRegions)
		fmt.Printf("%v\n", v.MatchZones)
		fmt.Printf("%+v", v.GetRowData())
		if len(v.GetRowData().(*ChargeConfigData).Diamond) != 0 {
			fmt.Printf("------> diamond:%+v", v.GetRowData().(*ChargeConfigData).Diamond[0])
		}
		fmt.Printf("\n")
	}

	type CsvRecord struct {
		MetaInfo struct {
			Region           string
			Zone             int
			FileName         string
			FullPathFileName string
		}
		*CsvOriginRowsData
	}
	csvRecord := &CsvRecord{}
	csvRecord.MetaInfo.Region = "1"
	csvRecord.MetaInfo.Zone = 2
	csvRecord.MetaInfo.FileName = "charge_config.csv"
	csvRecord.MetaInfo.FullPathFileName = "./charge_config.csv"

	csvRecord.CsvOriginRowsData, err =
		rows.FilterWithRegionZone(csvRecord.MetaInfo.Region, csvRecord.MetaInfo.Zone)
	if err != nil {
		panic(err)
	}

	fmt.Printf("====主键=====\n")
	for k, v := range csvRecord.CsvOriginRowsData.KeyIndexData.KeyIndexMap {
		fmt.Printf("key:%v,%v\n", k, v)
	}
	fmt.Printf("====组主键=====\n")
	for _, v := range csvRecord.CsvOriginRowsData.GroupIndexData.GroupIndexMaps {
		fmt.Printf("-------次组主键----------\n")
		for k, v1 := range v {
			fmt.Printf("group:%v, list:%v\n", k, v1)
		}
	}
	fmt.Printf("=======联合索引======\n")
	for k, v := range csvRecord.CsvOriginRowsData.UnionGroupIndexData.UnionGroupIndexMaps {
		fmt.Printf("次索引:%v\n", k)
		for k1, v1 := range v {
			fmt.Printf("次组:%v, %+v\n", k1, v1)
		}
	}
	fmt.Printf("=======联合唯一索引======\n")
	for k, v := range csvRecord.CsvOriginRowsData.UnionUniqueIndexData.UnionUniqueIndexMaps {
		fmt.Printf("次索引:%v\n", k)
		for k1, v1 := range v {
			fmt.Printf("次组:%v, %+v\n", k1, v1)
		}
	}
}

func parseTest() {
	type Struct struct {
		ID             int    `csv:"id" index:"true"`
		Group1         int    `csv:"group1" group:"true"`
		Group2         string `csv:"group2" group:"true"`
		UnionGroup1_1  int    `csv:"ug11" union:"group1"`
		UnionGroup2_1  string `csv:"ug21" union:"group2"`
		UnionGroup1_2  string `csv:"ug12" union:"group1"`
		UniqueGroup1_1 string `csv:"qg11" unionu:"unique1"`
		UniqueGroup1_2 int64  `csv:"qg12" unionu:"unique1"`
		UnionGroup2_2  bool   `csv:"ug22" union:"group2"`
		UniqueGroup2_1 string `csv:"qg21" unionu:"unique2"`
		UniqueGroup2_2 int64  `csv:"qg22" unionu:"unique2"`
	}

	fields := []string{"id", "group1", "group2", "ug11", "ug21", "ug12", "qg11", "qg12", "ug22", "qg21", "qg22"}
	rows := [][]string{
		[]string{"1", "1", "group1", "11", "ug21", "ug11", "qg11", "12", "false", "qg21", "22"},
		[]string{"2", "1", "group2", "11", "ug21", "ug11", "qg12", "13", "false", "qg21", "23"},
		[]string{"3", "1", "group2", "11", "ug21", "ug11", "qg11", "14", "false", "qg21", "24"},
	}
	csvRows, err := parseOriginFileData(fields, rows, reflect.TypeOf(Struct{}))
	if err != nil {
		panic(err)
	}

	csvOriginRows, err := csvRows.FilterWithRegionZone("1", 2)
	if err != nil {
		panic(err)
	}
	for _, v := range csvOriginRows.Rows {
		fmt.Printf("data:%+v\n", v)
	}
	fmt.Printf("主键===========\n")
	for k, v := range csvOriginRows.KeyIndexData.KeyIndexMap {
		fmt.Printf("%v:%v\n", k, v)
	}
	fmt.Printf("组主键=============\n")
	for i, v := range csvOriginRows.GroupIndexData.GroupIndexMaps {
		fmt.Printf("次组主键[%v]---------\n", i)
		for k1, v1 := range v {
			fmt.Printf("%v:%+v\n", k1, v1)
		}
	}
	fmt.Printf("联合主键=================\n")
	for k, v := range csvOriginRows.UnionGroupIndexData.UnionGroupIndexMaps {
		fmt.Printf("次联合主键[%v]---------\n", k)
		for k1, v1 := range v {
			fmt.Printf("%v:%+v\n", k1, v1)
		}
	}
	fmt.Printf("联合唯一主键===============\n")
	for k, v := range csvOriginRows.UnionUniqueIndexData.UnionUniqueIndexMaps {
		fmt.Printf("次联合唯一主键[%v]---------\n", k)
		for k1, v1 := range v {
			fmt.Printf("%v:%v\n", k1, v1)
		}
	}
}

func TestLoadCsv(t *testing.T) {
	// loadFile(t)
	parseTest()
}
