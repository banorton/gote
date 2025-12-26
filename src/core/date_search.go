package core

import (
	"fmt"
	"strconv"
	"strings"

	"gote/src/data"
)

// DateRange represents a date range for searching
type DateRange struct {
	Start string // yymmdd.hhmmss format
	End   string // yymmdd.hhmmss format
}

// ParseDateInput parses a date input string and returns its expanded range
// Supports: yy, yymm, yymmdd, yymmdd.hh, yymmdd.hhmm, yymmdd.hhmmss
func ParseDateInput(input string) (DateRange, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return DateRange{}, fmt.Errorf("empty date input")
	}

	// Validate characters (only digits and .)
	for _, c := range input {
		if c != '.' && (c < '0' || c > '9') {
			return DateRange{}, fmt.Errorf("invalid character in date: %c", c)
		}
	}

	var start, end string

	switch len(input) {
	case 2: // yy - year
		start = input + "0101.000000"
		end = input + "1231.235959"
	case 4: // yymm - month
		year := input[:2]
		month := input[2:4]
		start = input + "01.000000"
		end = input + lastDayOfMonth(year, month) + ".235959"
	case 6: // yymmdd - day
		start = input + ".000000"
		end = input + ".235959"
	case 9: // yymmdd.hh - hour
		if input[6] != '.' {
			return DateRange{}, fmt.Errorf("invalid format: expected . at position 6")
		}
		start = input + "0000"
		end = input + "5959"
	case 11: // yymmdd.hhmm - minute
		if input[6] != '.' {
			return DateRange{}, fmt.Errorf("invalid format: expected . at position 6")
		}
		start = input + "00"
		end = input + "59"
	case 13: // yymmdd.hhmmss - second
		if input[6] != '.' {
			return DateRange{}, fmt.Errorf("invalid format: expected . at position 6")
		}
		start = input
		end = input
	default:
		return DateRange{}, fmt.Errorf("invalid date length: %d (expected 2, 4, 6, 9, 11, or 13)", len(input))
	}

	return DateRange{Start: start, End: end}, nil
}

// lastDayOfMonth returns the last day of the given month
func lastDayOfMonth(year, month string) string {
	y, _ := strconv.Atoi(year)
	m, _ := strconv.Atoi(month)

	// Adjust year for 2000s
	if y < 100 {
		y += 2000
	}

	// Days in each month
	days := []int{0, 31, 28, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31}

	// Leap year check
	if m == 2 && (y%4 == 0 && (y%100 != 0 || y%400 == 0)) {
		return "29"
	}

	if m >= 1 && m <= 12 {
		return fmt.Sprintf("%02d", days[m])
	}
	return "31"
}

// ParseDateRange parses one or two date inputs into a single range
func ParseDateRange(inputs []string) (DateRange, error) {
	if len(inputs) == 0 {
		return DateRange{}, fmt.Errorf("no date inputs provided")
	}

	if len(inputs) == 1 {
		return ParseDateInput(inputs[0])
	}

	// Two inputs - use start of first and end of second
	first, err := ParseDateInput(inputs[0])
	if err != nil {
		return DateRange{}, fmt.Errorf("invalid start date: %w", err)
	}

	second, err := ParseDateInput(inputs[1])
	if err != nil {
		return DateRange{}, fmt.Errorf("invalid end date: %w", err)
	}

	return DateRange{Start: first.Start, End: second.End}, nil
}

// SearchNotesByDate searches for notes within a date range
// useCreated: true = search by creation date, false = search by modification date
func SearchNotesByDate(dateInputs []string, useCreated bool, limit int) ([]SearchResult, error) {
	dateRange, err := ParseDateRange(dateInputs)
	if err != nil {
		return nil, err
	}

	index, err := data.LoadIndex()
	if err != nil {
		return nil, err
	}
	var results []SearchResult

	for title, meta := range index {
		var dateValue string
		if useCreated {
			dateValue = meta.Created
		} else {
			dateValue = meta.Modified
		}

		// Handle old index entries that might not have Modified field
		if dateValue == "" {
			continue
		}

		// Check if date is within range (string comparison works for yymmdd.hhmmss format)
		if dateValue >= dateRange.Start && dateValue <= dateRange.End {
			results = append(results, SearchResult{
				Title:    title,
				FilePath: meta.FilePath,
				Score:    1,
			})
		}
	}

	sortResultsByTitle(results)

	if limit > 0 && limit < len(results) {
		results = results[:limit]
	}

	return results, nil
}
