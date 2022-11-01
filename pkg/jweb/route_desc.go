package jweb

import (
	"reflect"
)

type RouteInfo struct {
	Desc           string
	Method         string
	StructTemplate interface{}
}

type fieldDescInfo struct {
	Name      string
	FieldName string
	Type      string
	Desc      string
	Tags      reflect.StructTag
}

func (ri *RouteInfo) HasFields() bool {
	return ri.StructTemplate != nil
}

func (ri *RouteInfo) JsonStructDesc() []*fieldDescInfo {
	if ri.StructTemplate == nil {
		return []*fieldDescInfo{}
	}
	list := make([]*fieldDescInfo, 0)
	to := reflect.TypeOf(ri.StructTemplate)
	for i := 0; i < to.NumField(); i++ {
		field := to.Field(i)
		list = append(list, &fieldDescInfo{
			Name:      field.Tag.Get("json"),
			FieldName: field.Name,
			Type:      field.Type.String(),
			Desc:      field.Tag.Get("desc"),
			Tags:      field.Tag,
		})
	}
	return list
}

func (ri *RouteInfo) String() string {
	if ri.StructTemplate == nil {
		return "无参数"
	}
	desc := ""
	to := reflect.TypeOf(ri.StructTemplate)
	for i := 0; i < to.NumField(); i++ {
		field := to.Field(i)
		desc += field.Tag.Get("json") + ": " + field.Type.String() + ";"
	}
	return desc
}
