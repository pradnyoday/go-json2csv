// main.go
package main

import (
	"bytes"
	"fmt"
	"log"
	"strings"

	"github.com/pradnyoday/go-json2csv/json2csv" // Import your package
)

// ItemsSummaryTransformer is now in json2csv/types.go, remove it from here.
// func ItemsSummaryTransformer(...)


func main() {
	// Sample JSON input data (as a string for demonstration)
	// This JSON contains an array "items" within each user object, which we will flatten.
	// Note: Standard JSON does not support comments like # or // within the data itself.
	jsonInput := `
[
  {
    "user_id": 101,
    "user_name": "Alice Smith",
    "is_active": true,
    "address": {
      "city": "New York",
      "zip": "10001"
    },
    "items": [
      {"item_id": "A1", "price": 10.50, "quantity": 1, "tags": ["electronics"]},
      {"item_id": "B2", "price": 5.00, "quantity": 3, "tags": ["book", "fiction"]}
    ],
    "created_at": 1678886400
  },
  {
    "user_id": 102,
    "user_name": "Bob Johnson",
    "is_active": false,
    "address": {
      "city": "London",
      "zip": "SW1A 0AA"
    },
    "items": [
      {"item_id": "C3", "price": 2.20, "quantity": 5, "tags": ["stationery"]}
    ],
    "created_at": 1678972800
  },
  {
      "user_id": 103,
      "user_name": "Charlie Brown",
      "is_active": true,
      "address": null,
      "items": [],
      "created_at": null 
  },
    {
      "user_id": 104,
      "user_name": "David Lee",
      "is_active": true,
      "address": {"city": "Tokyo"},
      "items": null,
      "created_at": 1679145600
  }
]
`

	// Create a reader from the JSON string
	jsonReader := strings.NewReader(jsonInput)

	// Use a bytes.Buffer as the writer to capture the CSV output in memory,
	// then print it to standard output. For large files, you'd write directly to os.Stdout
	// or a file.
	var csvBuffer bytes.Buffer
	csvWriter := &csvBuffer // io.Writer interface

	// Configure the conversion options, using "[*]" in JSONPath for flattening.
	// This is now the ONLY supported mode.
	options := json2csv.Options{
		Fields: []json2csv.Field{
			// Fields from the top-level object (repeated for each flattened row)
			{JSONPath: "user_id", CSVHeader: "User ID"},
			{JSONPath: "user_name", CSVHeader: "User Name"},
			{JSONPath: "is_active", CSVHeader: "Active Status", Transformer: json2csv.BoolToYesNo},
			{JSONPath: "address.city", CSVHeader: "City"}, // Nested field from parent
			{JSONPath: "created_at", CSVHeader: "Created At", Transformer: json2csv.FormatUnixTimestamp},

			// Fields from the flattened array items, indicated by "[*]"
			{JSONPath: "items[*].item_id", CSVHeader: "Item ID"},
			{JSONPath: "items[*].price", CSVHeader: "Item Price"},
			{JSONPath: "items[*].quantity", CSVHeader: "Quantity"},
			// Custom transformer for joining an array of tags within the item.
			{JSONPath: "items[*].tags", CSVHeader: "Item Tags", Transformer: func(value interface{}, originalRecord map[string]interface{}) (interface{}, error) {
				if tags, ok := value.([]interface{}); ok {
					// Join the tags array elements into a string
					tagStrings := make([]string, len(tags))
					for i, tag := range tags {
						tagStrings[i] = fmt.Sprintf("%v", tag) // Basic conversion to string
					}
					return strings.Join(tagStrings, ";"), nil // Join with semicolon
				}
				// Return empty string or nil if tags is not an array or is null/missing
				return "", nil
			}},
            // Example of a field that might be missing in some items
            {JSONPath: "items[*].extra_field", CSVHeader: "Extra Item Field"},
             // Example of getting the item object itself (would be stringified map[...])
            // {JSONPath: "items[*]", CSVHeader: "Item Object String"},
		},
		Delimiter: ',',
		AddHeader: true, // Explicitly add header
	}

	fmt.Println("Converting JSON to CSV (Inferring Flattening from [*] - Flattening Only)...")

	// Perform the conversion
	err := json2csv.Convert(jsonReader, csvWriter, options)
	if err != nil {
		// Use log.Fatalf which prints the error and exits
		log.Fatalf("Error converting JSON to CSV: %v", err)
	}

	fmt.Println("\nCSV Output (Inferring Flattening from [*] - Flattening Only):")
	// Print the accumulated CSV from the buffer to standard output
	fmt.Println(csvBuffer.String())

    // Removed the second, non-flattening example as it's no longer supported by the package.

}