package elastic

import "sort"

func ExtractFieldsFromMapping(mapping map[string]interface{}) []string {
	fields := make(map[string]struct{})
	for _, indexMapping := range mapping {
		indexMap, ok := indexMapping.(map[string]interface{})
		if !ok {
			continue
		}

		props := extractProperties(indexMap)
		if len(props) == 0 {
			continue
		}
		collectFields("", props, fields)
	}

	names := make([]string, 0, len(fields))
	for name := range fields {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func extractProperties(indexMap map[string]interface{}) map[string]interface{} {
	if props, ok := indexMap["properties"].(map[string]interface{}); ok {
		return props
	}

	mappings, ok := indexMap["mappings"].(map[string]interface{})
	if !ok {
		return nil
	}

	if props, ok := mappings["properties"].(map[string]interface{}); ok {
		return props
	}

	for _, typed := range mappings {
		typedMap, ok := typed.(map[string]interface{})
		if !ok {
			continue
		}
		if props, ok := typedMap["properties"].(map[string]interface{}); ok {
			return props
		}
	}

	return nil
}

func collectFields(prefix string, props map[string]interface{}, fields map[string]struct{}) {
	for name, raw := range props {
		fieldName := name
		if prefix != "" {
			fieldName = prefix + "." + name
		}
		fields[fieldName] = struct{}{}

		propMap, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}

		if nested, ok := propMap["properties"].(map[string]interface{}); ok {
			collectFields(fieldName, nested, fields)
		}
		if multi, ok := propMap["fields"].(map[string]interface{}); ok {
			collectFields(fieldName, multi, fields)
		}
	}
}
