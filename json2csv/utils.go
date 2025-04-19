// json2csv/utils.go
package json2csv

import (
	"encoding/json"
	"fmt"
	"strings"
)

// valueToString is a helper to safely get a string representation of a value.
// Used internally before writing to the CSV writer. Handles basic Go types
// from JSON decoding and treats nil as an empty string.
func valueToString(value interface{}) string {
	if value == nil {
		return "" // Treat JSON null or unresolved path as empty string in CSV
	}

	switch v := value.(type) {
	case string:
		return v
	case float64: // JSON numbers are typically decoded as float64 by default
		return fmt.Sprintf("%g", v) // Use %g to avoid trailing zeros for integers
	case bool:
		return fmt.Sprintf("%t", v)
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, complex64, complex128:
		// Handle explicit integer types if they occur
		return fmt.Sprintf("%v", v)
	case json.Number: // If json.Decoder.UseNumber() is used
		return v.String()
	default:
		// For slices, maps, or other complex types at the leaf, stringify them.
		// This might produce verbose output like map[key:value].
		// A more specific transformer is recommended for complex leaf types.
		return fmt.Sprintf("%v", v)
	}
}

// getFlattenArrayPath determines the path to the primary array for flattening
// based on the Field JSONPaths. It finds the path segment immediately preceding
// the first occurrence of "[*]" in any field's path.
// Returns the array path or an empty string if no field contains "[*]".
// Assumes if multiple fields use "[*]", they refer to the same base array path.
func getFlattenArrayPath(fields []Field) string {
	for _, field := range fields {
		starIndex := strings.Index(field.JSONPath, "[*]")
		if starIndex != -1 {
			// Found "[*]". The array path is the part before "[*]".
			// Handle cases like "[*].name" (path is empty) or "items[*].name" (path is "items").
			basePath := field.JSONPath[:starIndex]
            // Trim trailing dot if present (e.g., "items.[*].name") -> "items." -> "items"
            if strings.HasSuffix(basePath, ".") {
                basePath = basePath[:len(basePath)-1]
            }
			return basePath // Return the first one found
		}
	}
	return "" // No field contains "[*]"
}

// getValueForField retrieves the value for a given field from the original record
// or a flattened array item, correctly interpreting paths with "[*]".
// If the field's path contains "[*]", data should be the current item from the flattened array,
// and originalRecord is used for context in transformers. The path is relative to the item
// after the "[*]" part has been handled by the flattening logic.
// If the field's path does NOT contain "[*]", data should be the originalRecord,
// and the path is relative to it. originalRecord is still passed for transformers.
func getValueForField(field Field, data map[string]interface{}, originalRecord map[string]interface{}) (interface{}, error) {
	// Determine the effective path relative to the provided 'data' map.
	// If field.JSONPath has "[*]", the 'data' map is an array item,
	// and the path we need to traverse in 'data' is the part *after* "[*]".
	effectivePath := field.JSONPath
	starIndex := strings.Index(field.JSONPath, "[*]")

	if starIndex != -1 {
		// Path contains "[*]". The part before "[*]" was used to find the array.
		// The part after "[*]" is the path within the array item.
		// Example: field.JSONPath "items[*].item_id". starIndex is 5. effectivePath is "item_id".
		effectivePath = field.JSONPath[starIndex+len("[*]"):]
        // Trim leading dot if present (e.g., "items[*].item_id") -> ".item_id" -> "item_id"
        if strings.HasPrefix(effectivePath, ".") {
            effectivePath = effectivePath[1:]
        }
         // Handle case like "items[*]" pointing directly to the item itself
         if effectivePath == "" {
             // The path is just "array[*]", meaning the item itself is the value.
             // 'data' *is* the item map. Return it directly.
              return data, nil // Or return nil, nil if item itself should be nil/error
         }


		// We already determined 'data' is the item map in the calling code (Convert).
		// Now get the value from the item map using the part of the path after "[*]".
		value, err := getValueByDotPath(data, effectivePath) // Use a dot-only path getter
        if err != nil {
            // Propagate errors from dot path traversal
            return nil, fmt.Errorf("failed to get value from array item at path %q (original: %q): %w", effectivePath, field.JSONPath, err)
        }
        return value, nil // Return the value found in the item
	} else {
		// Path does NOT contain "[*]". Get the value from the original record.
		// 'data' is expected to be originalRecord in this case.
		value, err := getValueByDotPath(data, effectivePath) // Use a dot-only path getter
        if err != nil {
            // Propagate errors from dot path traversal
            return nil, fmt.Errorf("failed to get value from record at path %q: %w", field.JSONPath, err)
        }
        return value, nil // Return the value found in the original record
	}
}


// getValueByDotPath retrieves a value from a nested map[string]interface{}
// using a dot-separated path (e.g., "user.address.city").
// This is a simplified version for paths *without* "[*]".
// It handles nested maps. If a path segment is not found, or if an intermediate
// segment is nil or not a map, it returns nil and a nil error, indicating
// the path could not be fully resolved to a value (e.g., null or missing intermediate).
// It returns an error only for syntactically invalid paths (like empty segments).
func getValueByDotPath(data map[string]interface{}, path string) (interface{}, error) {
	// An empty path for getValueByDotPath after "[*]" is valid for "array[*]" case.
    // But if called with a non-empty path, empty segments are invalid.
	if path == "" {
        // If path is empty, it implies the caller wanted the data map itself (e.g., for "array[*]").
        // This case is handled in getValueForField. If this function is called with "" path,
        // it's likely an error in logic flow unless the original path was just "[*]".
        // Let's return an error if called with empty path when not expected.
        // Revisit getValueForField: if path is "array[*]", effectivePath is "".
        // If getValueByDotPath("") is called, it should return the 'data' map itself.
        // Let's adjust this helper's behavior slightly.
        return data, nil // If path is empty, return the current data object/map
	}

	keys := strings.Split(path, ".")
	currentValue := interface{}(data)

	for i, key := range keys {
		if key == "" {
             // This should ideally not happen with paths correctly derived from JSONPath,
             // but as a safeguard:
             return nil, fmt.Errorf("json2csv: invalid dot path segment (empty key) at index %d", i)
        }

		// Check if the current value is a map
		if m, ok := currentValue.(map[string]interface{}); ok {
			// Current value is a map, try to get the next key
			nextValue, exists := m[key]
			if !exists {
				// Key not found at this level. Path cannot be fully resolved.
				return nil, nil // Return nil value, nil error
			}
			currentValue = nextValue
		} else if currentValue == nil {
            // Current value is nil. Path cannot be resolved further.
            return nil, nil // Return nil value, nil error
        } else {
			// Current value is not a map, and not nil.
            // If there are more keys in the path, we can't traverse further.
            if i < len(keys)-1 {
                 // Expected a map to continue traversing, but got something else.
                 // Path cannot be fully resolved.
                 return nil, nil // Return nil value, nil error
            }
            // If it is the last segment, the currentValue is the final value.
            // This case means the path pointed directly to a non-map, non-nil value.
            // The currentValue already holds this value.
		}
	}

	// Successfully traversed the entire path
	return currentValue, nil
}