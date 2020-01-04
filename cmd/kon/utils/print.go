package utils

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/olekukonko/tablewriter"
)

var (
	lineSeparator = strings.Repeat("-", 80)
)

func FormatTable(table *tablewriter.Table) {
	table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
	table.SetCenterSeparator("-")
	table.SetColumnSeparator(" ")
}

func PrintImportant(message string, header string) {
	if header == "" {
		header = "IMPORTANT"
	}
	fmt.Println(lineSeparator)
	fmt.Println(header)
	fmt.Println(lineSeparator)
	fmt.Println(message)
	fmt.Println(lineSeparator)
	fmt.Println("")
}

func PrintJSON(val interface{}) {
	data, _ := json.MarshalIndent(val, "", "  ")
	fmt.Println(string(data))
}

type descPair struct {
	Desc string
	Val  interface{}
}

func PrintDescStruct(val interface{}) {
	t := reflect.TypeOf(val)
	if t.Kind() != reflect.Struct {
		fmt.Printf("not a struct: %T, %v", val, val)
		return
	}
	items := make([]descPair, 0, t.NumField())
	items = appendDescItems(items, val)

	// figure out max length
	maxLen := 0
	for _, item := range items {
		itemLen := len(item.Desc)
		if itemLen > maxLen {
			maxLen = itemLen
		}
	}
	fmtStr := fmt.Sprintf("%%%dv: %%v\n", maxLen) // generate %10v: %v
	for _, item := range items {
		fmt.Printf(fmtStr, item.Desc, item.Val)
	}
}

func appendDescItems(items []descPair, val interface{}) []descPair {
	v := reflect.ValueOf(val)
	t := reflect.TypeOf(val)
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		// flatten
		if v.Field(i).Kind() == reflect.Struct {
			items = appendDescItems(items, v.Field(i).Interface)
			continue
		}
		// find its tag
		desc := f.Tag.Get("desc")
		if desc == "" {
			continue
		}
		items = append(items, descPair{
			Desc: desc,
			Val:  v.Field(i).Interface(),
		})
	}
	return items
}
