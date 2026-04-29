package output

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/rosaiju/whaletrack/internal/edgar"
)

// WriteJSON exports filings to a JSON file.
func WriteJSON(path string, filings []*edgar.Filing) error {
	data, err := json.MarshalIndent(filings, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal JSON: %w", err)
	}
	return os.WriteFile(path, data, 0644)
}
