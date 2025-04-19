// json2csv/convert.go
package json2csv

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

// Convert reads JSON objects from r, converts them to CSV rows based on options.
// It requires at least one Field's JSONPath to contain "[*]" to trigger flattening.
// Extreme streaming and memory optimization are maintained.
func Convert(r io.Reader, w io.Writer, options Options) error {
	// Set default delimiter if not provided
	if options.Delimiter == ',' {
		options.Delimiter = DefaultDelimiter
	}

	csvWriter := csv.NewWriter(w)
	csvWriter.Comma = options.Delimiter
	defer csvWriter.Flush() // Ensure any buffered data is written at the end

	// Handle default for AddHeader. If not explicitly set to false, default to true.
	addHeader := true
	if options.AddHeader == false {
		addHeader = false
	}

	if addHeader {
		headerRow := make([]string, len(options.Fields))
		for i, field := range options.Fields {
			headerRow[i] = field.CSVHeader
		}
		if err := csvWriter.Write(headerRow); err != nil {
			return fmt.Errorf("json2csv: failed to write header: %w", err)
		}
	}

	decoder := json.NewDecoder(r)
	decoder.UseNumber() // Keep numbers as json.Number for precision

	// Expect the input to be a JSON array of objects.
	token, err := decoder.Token()
	if err != nil {
        if err == io.EOF { return nil } // Handle empty input
		return fmt.Errorf("json2csv: failed to read initial token: %w", err)
	}
	if delim, ok := token.(json.Delim); !ok || delim.String() != "[" {
		return fmt.Errorf(`json2csv: expected start of json array "[", but got %v (%T)`, token, token)
	}

	// Determine the path to the array that will trigger flattening.
	flattenArrayPath := getFlattenArrayPath(options.Fields)
    if err != nil {
        return fmt.Errorf("json2csv: failed to determine flattening array path: %w", err)
    }
    isFlattening := flattenArrayPath != ""

    // --- New Check: Require flattening ---
    if !isFlattening {
        return errors.New("json2csv: flattening is the only supported mode. At least one Field JSONPath must contain '[*]'")
    }
    // --- End New Check ---


	// Process each JSON object in the array
	for decoder.More() {
		var originalRecord map[string]interface{}
		err := decoder.Decode(&originalRecord)
		if err != nil {
			return fmt.Errorf("json2csv: failed to decode json object: %w", err)
		}

		var itemsToProcess []map[string]interface{} // Will hold the array items

        // Get the array value from the original record using the determined path
		arrayValue, getArrErr := getValueByDotPath(originalRecord, flattenArrayPath)
		if getArrErr != nil {
			// Error getting the array itself (e.g., path segment not a map)
			return fmt.Errorf("json2csv: failed to get array for flattening at path %q: %w", flattenArrayPath, getArrErr)
		}

        // Handle null or non-array values at the flattening path
		if arrayValue == nil {
			// Value is null. Treat as empty array, skip this record.
			continue
		}

		arr, ok := arrayValue.([]interface{})
		if !ok {
			// Value is not an array (and not null). Return error.
			return fmt.Errorf("json2csv: value at flatten path %q is not an array or null, but %T", flattenArrayPath, arrayValue)
		}

        // Convert array items to map[string]interface{} slice
        for i, item := range arr {
            if itemMap, itemIsMap := item.(map[string]interface{}); itemIsMap {
                itemsToProcess = append(itemsToProcess, itemMap)
            } else if item == nil {
                 // Handle null items within the array by skipping them.
                 continue
            } else {
                // Handle array elements that are not objects. Error out.
                return fmt.Errorf("json2csv: array element at path %q index %d is not a JSON object, but %T", flattenArrayPath, i, item)
            }
        }

        // If after processing, itemsToProcess is empty (original array was empty or contained only null/non-objects)
        if len(itemsToProcess) == 0 {
             continue // Skip this record
        }


		// --- Process Items (the flattened array items) ---
		for _, itemData := range itemsToProcess { // itemData is a flattened array item map
			csvRow := make([]string, len(options.Fields))

			for i, field := range options.Fields {
				var value interface{}
				var getValErr error

                // Determine the data source and effective path based on whether the field has "[*]".
                starIndex := strings.Index(field.JSONPath, "[*]")

                if starIndex != -1 {
                    // Field has "[*]". Get value from the current itemData (the array item map).
                    pathAfterStar := field.JSONPath[starIndex+len("[*]"):]
                     if strings.HasPrefix(pathAfterStar, ".") {
                        pathAfterStar = pathAfterStar[1:]
                     }
                     // Handle "array[*]" case (path after star is empty) implicitly handled by getValueByDotPath

                     value, getValErr = getValueByDotPath(itemData, pathAfterStar) // Get value from the item map
                     if getValErr != nil {
                        return fmt.Errorf("json2csv: failed to get value from array item for field %q (path after [*]: %q): %w", field.JSONPath, pathAfterStar, getValErr)
                     }

                } else {
                    // Field does NOT have "[*]". Get value from the original record.
                     value, getValErr = getValueByDotPath(originalRecord, field.JSONPath) // Get value from original record
                     if getValErr != nil {
                        return fmt.Errorf("json2csv: failed to get value from record for field %q: %w", field.JSONPath, getValErr)
                     }
                }

                // Note: If getValueByDotPath successfully returns nil, nil, 'value' will be nil, valueToString handles as "".


				// Apply transformation if a transformer is provided
				transformedValue := value
				var transformErr error
				if field.Transformer != nil {
					transformedValue, transformErr = field.Transformer(value, originalRecord) // Pass originalRecord for context
					if transformErr != nil {
						// Handle transformation error: propagate it.
						return fmt.Errorf("json2csv: failed to transform field %q: %w", field.JSONPath, transformErr)
					}
				}

				// Convert the transformed value to a string for CSV
				csvRow[i] = valueToString(transformedValue)
			}

			// Write the CSV row
			if err := csvWriter.Write(csvRow); err != nil {
				return fmt.Errorf("json2csv: failed to write csv row: %w", err)
			}
		}
	}

	// Read the closing bracket ']'
	token, err = decoder.Token()
	if err != nil {
        if err == io.EOF {
             return fmt.Errorf("json2csv: unexpected EOF while expecting end of array ']'")
        }
		return fmt.Errorf("json2csv: failed to read final token: %w", err)
	}
	if delim, ok := token.(json.Delim); !ok || delim.String() != "]" {
		return fmt.Errorf(`json2csv: expected end of json array "]", but got %v (%T)`, token, token)
	}

	// Flush any remaining buffered CSV data
	csvWriter.Flush()
	if err := csvWriter.Error(); err != nil {
		return fmt.Errorf("json2csv: error flushing csv writer: %w", err)
	}

	return nil // Success
}