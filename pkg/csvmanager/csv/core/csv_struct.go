package core

import (
	"reflect"
)

type templateTypeRecord struct {
	to reflect.Type
}

func (r *templateTypeRecord) getKeyColIndex() int {
	for i := 0; i < r.to.NumField(); i++ {
		field := r.to.Field(i)
		indexTagFlag := field.Tag.Get("index")
		if indexTagFlag == "true" {
			return i
		}
	}

	return -1
}

func (r *templateTypeRecord) getGroupKeyColsIndex() (cols []int) {
	for i := 0; i < r.to.NumField(); i++ {
		field := r.to.Field(i)
		groupTag := field.Tag.Get("group")
		if groupTag == "true" {
			cols = append(cols, i)
		}
	}
	return
}

func (r *templateTypeRecord) getUnionGroupKeyColsIndex() (cols map[string][]int) {
	cols = make(map[string][]int)
	for i := 0; i < r.to.NumField(); i++ {
		field := r.to.Field(i)
		groupTag := field.Tag.Get("union")
		if groupTag != "" {
			list, find := cols[groupTag]
			if !find {
				list = make([]int, 0, 1)
			}
			list = append(list, i)
			cols[groupTag] = list
		}
	}
	return
}

func (r *templateTypeRecord) getUnionUniqueGroupKeyColsIndex() (cols map[string][]int) {
	cols = make(map[string][]int)
	for i := 0; i < r.to.NumField(); i++ {
		field := r.to.Field(i)
		groupTag := field.Tag.Get("unionu")
		if groupTag != "" {
			list, find := cols[groupTag]
			if !find {
				list = make([]int, 0, 1)
			}
			list = append(list, i)
			cols[groupTag] = list
		}
	}
	return
}
