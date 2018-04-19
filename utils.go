package service

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"sort"

	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

// Advanced Unicode normalization and filtering,
// see http://blog.golang.org/normalization and
// http://godoc.org/golang.org/x/text/unicode/norm for more
// details.
func stripCtlAndExtFromUnicode(str string) string {
	isOk := func(r rune) bool {
		return r < 32 || r >= 127
	}
	// The isOk filter is such that there is no need to chain to norm.NFC
	t := transform.Chain(norm.NFKD, transform.RemoveFunc(isOk))
	// This Transformer could also trivially be applied as an io.Reader
	// or io.Writer filter to automatically do such filtering when reading
	// or writing data anywhere.
	str, _, _ = transform.String(t, str)
	return str
}

func TruncateMatches(matches []Match, amount int) []Match {
	truncatedMatches := make([]Match, 0)
	for index, match := range matches {
		if index >= amount {
			break
		}

		truncatedMatches = append(truncatedMatches, match)
	}

	return truncatedMatches
}

func GetFPMatches(matchMap map[string][]Match, keys []SportKey, order string) []Match {
	matches := make([]Match, 0)

	for _, key := range keys {
		temp := matchMap[key.Sport]
		if order == "date" {
			sort.Sort(ByDate(temp))
		} else if order == "popular" {
			sort.Sort(ByPopular(temp))
		}

		for index, match := range temp {
			if index >= 3 {
				break
			}

			matches = append(matches, match)
		}
	}

	return matches
}

func aggregateErrors(msg string, errors []error) error {
	if len(errors) > 0 {
		var aggregateError string
		for _, err := range errors {
			aggregateError += err.Error() + "\n"
		}
		return fmt.Errorf("%s:\n %v", msg, aggregateError)
	}
	return nil
}

func WriteDataToJSONFile(filename string, data interface{}) (err error) {
	dataJSON, _ := json.Marshal(data)
	err = ioutil.WriteFile(filename+".json", dataJSON, 0644)
	return
}
