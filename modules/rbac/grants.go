package rbac

import (
	"encoding/json"
	"slices"
	"strconv"
)

type grant struct {
	node      string
	valueType string
	merge     string
	value     []byte
}

// flattenGrants expects grants ordered by node, then role ID, so the first value of a string node is the lowest role ID's.
func flattenGrants(grants []grant) ([]string, error) {
	permissions := []string{}
	for i := 0; i < len(grants); {
		j := i
		for j < len(grants) && grants[j].node == grants[i].node {
			j++
		}
		group := grants[i:j]
		i = j

		node := group[0].node
		var values [][]byte
		for _, g := range group {
			if g.value != nil {
				values = append(values, g.value)
			}
		}
		if len(values) == 0 {
			permissions = append(permissions, node)
			continue
		}

		switch group[0].valueType {
		case ValueTypeInt:
			var best int64
			for k, raw := range values {
				var n int64
				if err := json.Unmarshal(raw, &n); err != nil {
					return nil, err
				}
				if k == 0 || (group[0].merge == MergeMin && n < best) || (group[0].merge != MergeMin && n > best) {
					best = n
				}
			}
			permissions = append(permissions, node+":"+strconv.FormatInt(best, 10))
		case ValueTypeString:
			var first string
			if err := json.Unmarshal(values[0], &first); err != nil {
				return nil, err
			}
			permissions = append(permissions, node+":"+first)
		case ValueTypeStringList:
			var union []string
			for _, raw := range values {
				var list []string
				if err := json.Unmarshal(raw, &list); err != nil {
					return nil, err
				}
				union = append(union, list...)
			}
			slices.Sort(union)
			union = slices.Compact(union)
			if len(union) == 0 {
				permissions = append(permissions, node)
				continue
			}
			for _, v := range union {
				permissions = append(permissions, node+":"+v)
			}
		default:
			permissions = append(permissions, node)
		}
	}
	return permissions, nil
}
