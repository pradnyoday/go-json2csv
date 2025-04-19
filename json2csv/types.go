// json2csv/types.go
package json2csv

import (
	"encoding/json"
	"fmt"
	"reflect"
	"time"
)

// Transformer defines a function that transforms a value for a specific field.
// It takes the value found at the field's JSONPath (after potential flattening)
// and the entire original record (the JSON object being processed before flattening)
// as input. It returns the transformed value to be written to CSV.
// The original record is provided for context.
type Transformer func(value interface{}, originalRecord map[string]interface{}) (interface{}, error)

// Field defines a mapping from a JSON path to a CSV header and an optional transformer.
// The JSONPath can include "[*]" to indicate an array that triggers flattening.
// Example: "user_id", "address.city", "items[*].item_id"
type Field struct {
	// JSONPath is the dot-separated path to the value in the JSON object.
	// Can include "[*]" to denote an array for flattening.
	JSONPath string

	// CSVHeader is the header text for this column in the output CSV.
	CSVHeader string

	// Transformer is an optional function to modify the value before writing it to CSV.
	Transformer Transformer
}

// Options contains configuration for the JSON to CSV conversion.
type Options struct {
	// Fields defines the ordered list of columns in the output CSV.
	// Each Field specifies the JSONPath to the data, the CSV header,
	// and an optional transformation. Paths with "[*]" trigger flattening.
	// If multiple fields use "[*]", they are assumed to refer to elements
	// within the same array identified by the path segment immediately
	// preceding the first "[*]".
	Fields []Field

	// Delimiter is the character used to separate fields in the CSV output.
	// Defaults to ',' if the zero value '\0' is used.
	Delimiter rune

	// AddHeader determines whether to include a header row at the beginning
	// of the output CSV based on the CSVHeader fields. Defaults to true if
	// explicitly set to true, otherwise false (Go zero value). Note: the
	// Convert function applies a default of true if not explicitly set to false.
	AddHeader bool
}

// DefaultDelimiter is the comma character.
const DefaultDelimiter = ','

// --- Standard Transformers provided by the package ---

// BoolToYesNo is a Transformer that converts a boolean value to "Yes" or "No".
// Handles nil values by returning an empty string.
func BoolToYesNo(value interface{}, originalRecord map[string]interface{}) (interface{}, error) {
    if value == nil {
        return "", nil // Handle nil value gracefully
    }
	if b, ok := value.(bool); ok {
		if b {
			return "Yes", nil
		}
		return "No", nil
	}
	// Return original value if not a boolean, or an error depending on desired strictness.
	// Returning original value for non-bool, non-nil inputs.
	return value, nil
}

// FormatUnixTimestamp is a Transformer that converts a Unix timestamp (float64 or int)
// to a formatted date string. It handles nil values by returning an empty string.
func FormatUnixTimestamp(value interface{}, originalRecord map[string]interface{}) (interface{}, error) {
    // Handle nil value gracefully (e.g., from missing field or JSON null)
    if value == nil {
        return "", nil // Return empty string for nil timestamp, no error
    }

	var t time.Time
	switch v := value.(type) {
	case float64: // Common for numbers decoded into interface{}
		t = time.Unix(int64(v), 0)
	case int, int8, int16, int32, int64: // Handle if decoded into specific int types
        t = time.Unix(reflect.ValueOf(v).Int(), 0) // Use reflection for generic int conversion
	case json.Number: // If json.Decoder.UseNumber() is used
		i, err := v.Int64()
		if err != nil {
			return nil, fmt.Errorf("json2csv: FormatUnixTimestamp: cannot convert json.Number %q to int64: %w", v, err)
		}
		t = time.Unix(i, 0)
	default:
		// If not nil and not a recognized number type, it's an unsupported type.
		// Return an error to indicate a problem with the data type.
		return value, fmt.Errorf("json2csv: FormatUnixTimestamp: unsupported non-nil type %T for timestamp", value)
	}
	// Use a common format, can be made configurable via transformer options if needed
	return t.Format("2006-01-02 15:04:05"), nil
}

// ItemsSummaryTransformer is a Transformer that summarizes a JSON array
// by reporting its size or status (e.g., "3 Items", "Empty Array", "Null Array").
// Useful for fields in the non-flattening scenario that are arrays.
func ItemsSummaryTransformer(value interface{}, originalRecord map[string]interface{}) (interface{}, error) {
	if value == nil {
		return "Null Array", nil // Handle nil value (JSON null or missing path)
	}

	if items, ok := value.([]interface{}); ok {
		if len(items) == 0 {
			return "Empty Array", nil // Handle empty array
		}
		return fmt.Sprintf("%d Items", len(items)), nil // Report number of items
	}

	// If the value is not nil and not an array, something unexpected is here.
	// Return a string indicating the type, or an error.
	return fmt.Sprintf("Unexpected Type: %T", value), nil
}

// --- Helper function declarations (implementations in utils.go) ---

// valueToString is a helper function (defined in utils.go) to convert values to string.
// No need to define it here.
// func valueToString(value interface{}) string { ... }

// getValueByDotPath is a helper function (defined in utils.go) to retrieve values by path.
// No need to define it here.
// func getValueByDotPath(data map[string]interface{}, path string) (interface{}, error) { ... }

// getFlattenArrayPath is a helper function (defined in utils.go) to determine the array path for flattening.
// No need to define it here.
// func getFlattenArrayPath(fields []Field) (string, error) { ... }