package agent_dto

import (
	"encoding/json"
	"fmt"
	"strings"
)

type LinkedObjectSource string

const (
	LinkedObjectSourceTrigger LinkedObjectSource = "trigger object"
	LinkedObjectSourcePivot   LinkedObjectSource = "pivot"
)

type LinkedObject struct {
	SourceEntity LinkedObjectSource
	LinkName     string
	TableName    string
	Data         map[string]any
}

type LinkedObjects []LinkedObject

func (l LinkedObjects) PrintForAgent() (string, error) {
	var sb strings.Builder

	for _, obj := range l {
		fmt.Fprintf(&sb, "### %s (%s, linked from %s)\n\n", obj.LinkName, obj.TableName, obj.SourceEntity)
		fmt.Fprintf(&sb, "<LinkedEntity link=\"%s\" table=\"%s\" source=\"%s\">\n",
			obj.LinkName, obj.TableName, obj.SourceEntity)
		dataBytes, err := json.Marshal(obj.Data)
		if err != nil {
			return "", err
		}
		sb.Write(dataBytes)
		sb.WriteByte('\n')
		fmt.Fprintf(&sb, "</LinkedEntity>\n\n")
	}

	return sb.String(), nil
}
