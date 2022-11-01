package csvmanager

import (
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"
)

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
	ShopTime    string        `csv:"shop_time"`
	OfflineMail string        `csv:"offline_mail"`
	// DTKJPrice   string        `csv:"dtkj_price"`
	// NoviceID int32 `csv:"novice_id"`
}

func (c *ChargeConfigData) CheckOrParse() (omit bool, err error) {
	fmt.Printf("entry check\n")
	return false, nil
}

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

func TestManager(t *testing.T) {
	chargeTableMeta := (&TableMetaData{
		No:              1,
		St:              ChargeConfigData{},
		File:            "./csv/core/charge_config.csv",
		ExtraDataGenFun: nil,
	}).Register()
	chargeTableMeta1 := (&TableMetaData{
		No:              2,
		St:              ChargeConfigData{},
		File:            "./csv/core/charge_config1.csv",
		ExtraDataGenFun: nil,
	}).Register()
	mgr, err := New("1", []int{1, 2}, "./")
	if err != nil {
		panic(err)
	}

	mgr.RegisterAfterLoadLogic(1, func(zones *CsvZoneManager) interface{} {
		table1, _, find := zones.GetTable(chargeTableMeta)
		if !find {
			panic(find)
		}

		table2, _, find := zones.GetTable(chargeTableMeta1)
		if !find {
			panic(find)
		}

		list := []interface{}{table1.Record(table1.NumRecord() - 1).(*ChargeConfigData)}
		list = append(list, []interface{}{table2.Record(table2.NumRecord() - 1).(*ChargeConfigData)}...)
		return list
	})

	zoneMgr, find := mgr.GetZoneCsvManager(1)
	if !find {
		panic(find)
	}

	afterLoadData, find := zoneMgr.GetAfterLoadData(1)
	if !find {
		panic(find)
	}
	for _, v := range afterLoadData.([]interface{}) {
		fmt.Printf("after load:%v\n", v.(*ChargeConfigData).ChargeID)
	}

	zoneMgr1, _ := mgr.GetZoneCsvManager(2)
	afterLoadData, find = zoneMgr1.GetAfterLoadData(1)
	if !find {
		panic(find)
	}
	for _, v := range afterLoadData.([]interface{}) {
		fmt.Printf("after load1:%v\n", v.(*ChargeConfigData).ChargeID)
	}

	return

	table, triggerLoad, find := zoneMgr.GetTable(chargeTableMeta)
	if !find {
		panic(find)
	}
	if !triggerLoad {
		panic(triggerLoad)
	}
	if table.Zone != 1 {
		panic(table)
	}
	table.Range(func(row interface{}) {
		data := row.(*ChargeConfigData)
		fmt.Printf("%v\n", data.ChargeID)
	})

	table1, tri, f := zoneMgr.GetTable(chargeTableMeta1)
	if !f {
		panic(f)
	}
	if !tri {
		panic(tri)
	}
	fmt.Printf("%+v\n", table1.Index(1).(*ChargeConfigData))

	return

	time.Sleep(time.Second * 10)

	// 重读---修改配置表
	mgr.ReloadRefresh()

	zoneMgr, find = mgr.GetZoneCsvManager(1)
	if !find {
		panic(find)
	}
	table, triggerLoad, find = zoneMgr.GetTable(chargeTableMeta)
	if !find {
		panic(find)
	}
	if !triggerLoad {
		panic(triggerLoad)
	}
	if table.Zone != 1 {
		panic(table)
	}
	table.Range(func(row interface{}) {
		data := row.(*ChargeConfigData)
		fmt.Printf("%v\n", data.ChargeID)
	})

	// 重读---不修改配置表
	mgr.ReloadRefresh()

	zoneMgr, find = mgr.GetZoneCsvManager(1)
	if !find {
		panic(find)
	}
	table, triggerLoad, find = zoneMgr.GetTable(chargeTableMeta)
	if !find {
		panic(find)
	}
	if triggerLoad {
		panic(triggerLoad)
	}
	if table.Zone != 1 {
		panic(table)
	}
	table.Range(func(row interface{}) {
		data := row.(*ChargeConfigData)
		fmt.Printf("%v\n", data.ChargeID)
	})
}
