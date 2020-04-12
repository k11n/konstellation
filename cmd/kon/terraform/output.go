package terraform

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cast"
)

type OutputValue struct {
	Type  interface{} `json:"type"`
	Value interface{} `json:"value"`
}

type OutputContainer map[string]*OutputValue

func ParseOutput(data []byte) (oc *OutputContainer, err error) {
	oc = &OutputContainer{}
	err = json.Unmarshal(data, oc)
	return
}

func (oc OutputContainer) GetString(key string) string {
	return cast.ToString(oc[key].Value)
}

func (oc OutputContainer) ParseField(key string, target interface{}) error {
	v := oc[key]
	if v == nil {
		return fmt.Errorf("key %s doesn't exist", key)
	}

	// serialize v to json, and deserialize
	data, err := json.Marshal(v.Value)
	if err != nil {
		return err
	}

	return json.Unmarshal(data, target)
}
