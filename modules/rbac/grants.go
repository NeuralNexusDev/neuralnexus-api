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
		entries, err := flattenNode(grants[i:j])
		if err != nil {
			return nil, err
		}
		permissions = append(permissions, entries...)
		i = j
	}
	return permissions, nil
}

func flattenNode(grants []grant) ([]string, error) {
	node := grants[0].node
	var values [][]byte
	for _, g := range grants {
		if g.value != nil {
			values = append(values, g.value)
		}
	}
	if len(values) == 0 {
		return []string{node}, nil
	}

	switch grants[0].valueType {
	case ValueTypeInt:
		var best int64
		for i, raw := range values {
			var n int64
			if err := json.Unmarshal(raw, &n); err != nil {
				return nil, err
			}
			if i == 0 || (grants[0].merge == MergeMin && n < best) || (grants[0].merge != MergeMin && n > best) {
				best = n
			}
		}
		return []string{node + ":" + strconv.FormatInt(best, 10)}, nil
	case ValueTypeString:
		var first string
		if err := json.Unmarshal(values[0], &first); err != nil {
			return nil, err
		}
		return []string{node + ":" + first}, nil
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
			return []string{node}, nil
		}
		entries := make([]string, len(union))
		for i, v := range union {
			entries[i] = node + ":" + v
		}
		return entries, nil
	}
	return []string{node}, nil
}
