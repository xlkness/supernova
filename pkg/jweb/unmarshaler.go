package jweb

import (
	"encoding/json"
	"fmt"
	"io"
	"joynova.com/library/supernova/pkg/jlog"
	"reflect"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

var structuredUnmarshaler = func(c *gin.Context, structTemplate interface{}) (interface{}, string, string, error) {
	toReq := reflect.TypeOf(structTemplate)
	receiver := reflect.New(toReq).Interface()
	value := reflect.ValueOf(receiver).Elem()
	// 给每个字段赋值
	to := value.Type()
	buf, err := io.ReadAll(c.Request.Body)
	if err != nil {
		jlog.Errorf("read io error:%v", err)
		return nil, "", "", err
	}
	if len(buf) == 0 { // 如果body没有参数，则参数来自url
		for i := 0; i < to.NumField(); i++ {
			f := to.Field(i)
			fieldTagName := f.Tag.Get("json")
			if fieldTagName == "" {
				fieldTagName = f.Name
			}

			field := value.Field(i)
			if !field.CanSet() {
				continue
			}
			fieldStr := c.Query(fieldTagName)
			err := setValue(field, fieldStr)
			if err != nil {
				// c.ResponseFailWithDefaultCode(fmt.Sprintf("query param(%v) error:%v", fieldTagName, err))
				return nil, field.String(), fieldStr, fmt.Errorf("query param(%v) error:%v", fieldTagName, err)
			}
		}
	} else { // 如果body有参数，则用body的参数json反序列化
		err := json.Unmarshal(buf, receiver)
		if err != nil {
			return nil, "", "", err
		}
	}
	return receiver, "", "", nil
}

// setValue 设置结构体一个字段的值
func setValue(field reflect.Value, value string) error {
	if field.Kind() == reflect.Ptr {
		if value == "" {
			return nil
		}

		if field.IsNil() {
			field.Set(reflect.New(field.Type().Elem()))
		}
		field = field.Elem()
	}
	switch field.Kind() {
	case reflect.String:
		field.SetString(value)
	case reflect.Bool:
		if value == "" {
			field.SetBool(false)
		} else {
			b, err := strconv.ParseBool(value)
			if err != nil {
				return err
			}
			field.SetBool(b)
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if value == "" {
			field.SetInt(0)
		} else {
			i, err := strconv.ParseInt(value, 0, field.Type().Bits())
			if err != nil {
				return err
			}
			field.SetInt(i)
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if value == "" {
			field.SetUint(0)
		}
		ui, err := strconv.ParseUint(value, 0, field.Type().Bits())
		if err != nil {
			return err
		}
		field.SetUint(ui)
	case reflect.Float32, reflect.Float64:
		if value == "" {
			field.SetFloat(0)
			break
		}
		f, err := strconv.ParseFloat(value, field.Type().Bits())
		if err != nil {
			return err
		}
		field.SetFloat(f)
	case reflect.Struct:
		return fmt.Errorf("unsupport struct field:%v", field.Type())
	case reflect.Slice:
		values := strings.Split(value, ",")
		if len(values) == 1 && values[0] == "" {
			values = []string{}
		}
		field.Set(reflect.MakeSlice(field.Type(), len(values), len(values)))
		for i := 0; i < len(values); i++ {
			err := setValue(field.Index(i), values[i])
			if err != nil {
				return err
			}
		}

	default:
		return fmt.Errorf("no support type %s", field.Type())
	}
	return nil
}
