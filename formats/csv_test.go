package formats

import (
	"reflect"
	"testing"
)

func Test_flatten(t *testing.T) {
	tests := []struct {
		name     string
		document map[string]interface{}
		want     map[string]interface{}
	}{
		{
			"simple",
			map[string]interface{}{
				"string": "1",
				"int":    2,
				"float":  3.0,
				"bool":   true,
				"slice":  []string{"a", "b", "c"},
			},
			map[string]interface{}{
				"string": "1",
				"int":    2,
				"float":  3.0,
				"bool":   true,
				"slice":  []string{"a", "b", "c"},
			},
		},
		{
			"nested",
			map[string]interface{}{
				"string": "value1",
				"map": map[string]interface{}{
					"string": "value2",
					"int":    2,
					"float":  3.0,
					"bool":   true,
					"slice":  []string{"a", "b", "c"},
				},
			},
			map[string]interface{}{
				"string": "value1",
				"map": map[string]interface{}{
					"string": "value2",
					"int":    2,
					"float":  3.0,
					"bool":   true,
					"slice":  []string{"a", "b", "c"},
				},
				"map.string": "value2",
				"map.int":    2,
				"map.float":  3.0,
				"map.bool":   true,
				"map.slice":  []string{"a", "b", "c"},
			},
		},
		{
			"very nested",
			map[string]interface{}{
				"string": "value1",
				"map": map[string]interface{}{
					"string": "value2",
					"int":    2,
					"float":  3.0,
					"bool":   true,
					"slice":  []string{"a", "b", "c"},
					"map": map[string]interface{}{
						"string": "value3",
						"int":    2,
						"float":  3.0,
						"bool":   true,
						"slice":  []string{"a", "b", "c"},
					},
				},
			},
			map[string]interface{}{
				"string": "value1",
				"map": map[string]interface{}{
					"string": "value2",
					"int":    2,
					"float":  3.0,
					"bool":   true,
					"slice":  []string{"a", "b", "c"},
					"map": map[string]interface{}{
						"string": "value3",
						"int":    2,
						"float":  3.0,
						"bool":   true,
						"slice":  []string{"a", "b", "c"},
					},
				},
				"map.string": "value2",
				"map.int":    2,
				"map.float":  3.0,
				"map.bool":   true,
				"map.slice":  []string{"a", "b", "c"},
				"map.map": map[string]interface{}{
					"string": "value3",
					"int":    2,
					"float":  3.0,
					"bool":   true,
					"slice":  []string{"a", "b", "c"},
				},
				"map.map.string": "value3",
				"map.map.int":    2,
				"map.map.float":  3.0,
				"map.map.bool":   true,
				"map.map.slice":  []string{"a", "b", "c"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := flatten(tt.document); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("flatten() = %v, want %v", got, tt.want)
			}
		})
	}
}
