package core

import (
	"errors"
	"fmt"
	"reflect"
)

func checkStFieldKind(typeRecord reflect.Type) error {
	if typeRecord == nil || typeRecord.Kind() != reflect.Struct {
		return errors.New("st must be a struct")
	}

	duplicate := make(map[string]bool, 20)
	for i := 0; i < typeRecord.NumField(); i++ {
		f := typeRecord.Field(i)
		fieldTagName := f.Tag.Get("csv")
		if fieldTagName == "" {
			continue
		} else if fieldTagName == "_" {
			continue
		}
		kind := f.Type.Kind()
		switch kind {
		case reflect.String:
		case reflect.Bool:
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		case reflect.Float32, reflect.Float64:
		case reflect.Slice:
		case reflect.Struct:
		case reflect.Ptr:
		default:
			return fmt.Errorf("invalid type: %v %s", f.Name, kind)
		}

		tag := f.Tag.Get("index")
		if tag == "true" {
			switch kind {
			case reflect.Slice:
				return fmt.Errorf("count not index %s field %v %v",
					kind, i, f.Name)
			}
		}

		_, find := duplicate[fieldTagName]
		if find {
			return fmt.Errorf("表结构体csv tag名[%v]重复", fieldTagName)
		}
		duplicate[fieldTagName] = true
	}

	return nil
}

func checkFileFiedlsName(fields []string) error {
	duplicate := make(map[string]bool, 20)
	for _, v := range fields {
		_, find := duplicate[v]
		if find {
			return fmt.Errorf("表文件字段重名[%v]", v)
		}
		duplicate[v] = true
	}
	return nil
}
