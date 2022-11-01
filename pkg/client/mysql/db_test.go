package mysql

import (
	"fmt"
	"math/rand"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"xorm.io/xorm"
)

type RoleBase struct {
	Id        int64     `xorm:"pk autoincr BIGINT(10)"`
	RoleId    int64     `xorm:"not null unique BIGINT(10)"`
	NickName  string    `xorm:"not null VARCHAR(255)"`
	Age       int       `xorm:"default 0 INT(3)"`
	AccountId int64     `xorm:"not null BIGINT(10)"`
	Zone      int       `xorm:"not null index INT(5)"`
	CreatedAt time.Time `xorm:"created"`
	UpdatedAt time.Time `xorm:"updated"`
	DeletedAt time.Time `xorm:"deleted"`
}
type BagFootballer struct {
	Id         int64     `xorm:"pk autoincr BIGINT(10)"`
	RoleId     int64     `xorm:"not null index BIGINT(11)"`
	ConfigId   int       `xorm:"not null comment('配置表id') index INT(10)"`
	Level      int       `xorm:"not null default 0 comment('等级') INT(4)"`
	Exp        int       `xorm:"not null default 0 comment('经验，一般是相对经验') INT(10)"`
	Grade      int       `xorm:"not null default 0 comment('阶级') INT(4)"`
	ExpireTime int64     `xorm:"not null default 0 comment('到期时间') BIGINT(11)"`
	Ftype      int       `xorm:"not null default 0 comment('球员类型') INT(10)"`
	LockFlag   int       `xorm:"not null default 0 comment('球员锁定') TINYINT(1)"`
	LuckyPoint int       `xorm:"not null default 0 comment('幸运点') INT(10)"`
	CreatedAt  time.Time `xorm:"created"`
	UpdatedAt  time.Time `xorm:"updated"`
	DeletedAt  time.Time `xorm:"deleted"`
}

func (tb *RoleBase) TableName() string {
	return "role_base"
}

type MarketTransPrice struct {
	FootballerId      int       `xorm:"not null pk unique(UQE_market_avg_price_footballer_id) INT(11)"`
	PriceType         int       `xorm:"not null pk comment('1:实时均价，2：每日均价，3：最高价格') unique(UQE_market_avg_price_footballer_id) SMALLINT(2)"`
	DailyIndex        int       `xorm:"not null pk default 0 comment('每日均价里周期自增，实时均价为0') unique(UQE_market_avg_price_footballer_id) INT(10)"`
	Grade             int       `xorm:"not null pk unique(UQE_market_avg_price_footballer_id) INT(4)"`
	TotalCount        int       `xorm:"not null default 0 comment('总量') INT(10)"`
	UpperPrice        int64     `xorm:"not null default 0 comment('均价高位(除以1000000的值)') BIGINT(20)"`
	LowerPrice        int64     `xorm:"not null default 0 comment('均价低位(模1000000的值)') BIGINT(20)"`
	HistoryPrice      int64     `xorm:"not null default 0 comment('如果真实的均价为0，表示刚清零还没有新订单，用这个字段') BIGINT(20)"`
	HistoryTotalCount int       `xorm:"not null default 0 comment('历史总量') INT(10)"`
	MaxPrice          int64     `xorm:"comment('最高成交价') BIGINT(20)"`
	MaxPriceCreatedat time.Time `xorm:"comment('最高价发生时间') DATETIME"`
	CreatedAt         time.Time `xorm:"created"`
	UpdatedAt         time.Time `xorm:"updated"`
	DeletedAt         time.Time `xorm:"deleted"`
}

func TestDB(t *testing.T) {

	db, err := NewDB(&Config{
		MasterDsn: "dev:dev123@tcp(192.168.1.22:3306)/greenly-likun?charset=utf8&parseTime=True&loc=Local",
		SlavesDsn: []string{"dev:dev123@tcp(192.168.1.22:3306)/greenly-likun-slave?charset=utf8&parseTime=True&loc=Local"},
	})
	if err != nil {
		panic(err)
	}

	type Price struct {
		FootballerId int
		Grade        int
		MaxPrice     int64
	}

	type GlobalLike struct {
		FootballerIDsdfsdf int `xorm:"footballer_id"`
		LikeNum            int
		SearchNum          int64
	}

	list12 := make([]*GlobalLike, 0)
	db.WriteEngine().Table("market_like_global").
		Select(fmt.Sprintf("footballer_id,sum(like_num)*%v as like_num,sum(search_num) as search_num", 2)).
		GroupBy("footballer_id,like_num,search_num").
		Find(&list12)

	for _, v := range list12 {
		fmt.Printf("%+v\n", v)
	}
	return

	aff, errd := db.WriteEngine().Table("bag_footballer").In("id", []int64{1, 2, 3}).
		Where("role_id=?", 624465741).Unscoped().Delete(&BagFootballer{})
	if errd != nil {
		panic(errd)
	}

	fmt.Printf("%v\n", aff)

	return

	data := &RoleBase{}
	has, err := db.ReadEngine().Where("role_id=?", 123).Get(data)
	if err != nil {
		panic(err)
	}
	if !has {
		panic(has)
	}
	if data.NickName != "slave-role" {
		panic(data)
	}

	rand.Seed(time.Now().UnixNano())

	_, err = db.WriteEngine().After(func(row interface{}) {
		data := row.(*RoleBase)
		fmt.Printf("temproay callback, insert ok:%+v\n", data)
	}).Insert(&RoleBase{
		RoleId:    int64(rand.Int()),
		NickName:  "fsdfds",
		Age:       12,
		AccountId: 123,
		Zone:      123,
	})
	if err != nil {
		panic(err)
	}

	_, err = db.WriteEngine().Insert(&RoleBase{
		RoleId:    123,
		NickName:  "master-role",
		Age:       12,
		AccountId: 123,
		Zone:      123,
	})
	if err == nil {
		panic(err)
	}

	list := make([]*RoleBase, 0)
	err = db.ReadFind(func(e *xorm.Engine) error {
		err := e.Table("123").Where("1=1").Find(&list)
		return err
	})
	if err != nil {
		panic(err)
	}

	getData := &RoleBase{}
	has, err = db.ReadGet(func(e *xorm.Engine) (bool, error) {
		has, err := e.Table("123").Where("1=1").Get(getData)
		return has, err
	})
	if err != nil {
		panic(err)
	}

	if !has {
		panic(has)
	}

	err = db.WriteExec(nil, func(e *xorm.Engine) (int, error) {
		aff, err := e.Table("123").Where("1=1").Update(getData)
		return int(aff), err
	})
	if err != nil {
		panic(err)
	}
}
