package dbdiff

import (
	"fmt"
	"testing"
)

func TestDiff(t *testing.T) {
	differ, err := NewMysqlDiffer(
		"dev:dev123@tcp(192.168.1.22:3306)/cof_new?charset=utf8&parseTime=True&loc=Local",
		"dev:dev123@tcp(192.168.1.22:3306)/cof_last?charset=utf8&parseTime=True&loc=Local",
	)
	if err != nil {
		panic(err)
	}

	result, err := differ.Diff("")
	if err != nil {
		panic(err)
	}

	strs, strs1 := differ.Generate(result, map[string]int{
		"role_base": 10,
	})

	for k, v := range strs {
		if k != "account_role_base" {
			continue
		}
		fmt.Printf("table[%v], generate sql diff for shard num:%v\n", k, len(v))
		for _, v1 := range v {
			fmt.Printf("%+v\n", v1)
		}
	}
	fmt.Printf("-----------------------\n")
	for k, v := range strs1 {
		if k != "account_role_base" {
			continue
		}
		fmt.Printf("table[%v], generate sql diff for shard num:%v\n", k, len(v))
		for _, v1 := range v {
			fmt.Printf("%+v\n", v1)
		}
	}
}
