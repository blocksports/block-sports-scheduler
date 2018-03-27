package service

import (
	"bytes"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"sort"
	"time"

	"github.com/a-h/round"
	"github.com/klauspost/compress/zlib"
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

func EncodeResponse(w http.ResponseWriter, response interface{}) {
	var buf bytes.Buffer
	zipper := zlib.NewWriter(&buf)

	responseJSON, _ := json.Marshal(response)
	zipper.Write(responseJSON)
	zipper.Close()

	responseEncoded := []byte(base64.StdEncoding.EncodeToString(buf.Bytes()))
	w.Write(responseEncoded)
}

func (svc *Service) RedisGetInterface(key string, v interface{}) (err error) {
	interfaceRaw, err := svc.RedisClient.Get(key).Bytes()
	if err != nil {
		return
	}

	err = json.Unmarshal(interfaceRaw, v)
	return
}

func (svc *Service) RedisSetInterface(key string, v interface{}) (err error) {
	interfaceJSON, err := json.Marshal(v)
	if err != nil {
		return
	}

	err = svc.RedisClient.Set(key, interfaceJSON, 0).Err()
	return
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

func GetScale(key string) float64 {
	// Seed rand from match name for consistency
	hashBytes := sha1.Sum([]byte(key))
	seed := binary.BigEndian.Uint64(hashBytes[:])
	source := rand.NewSource(int64(seed))
	r := rand.New(source)
	scale := round.ToEven(r.Float64(), 3)

	return scale
}

func GetTimeScale(unixTime int64) float64 {
	fnTimeScale := makeSigmoidal([4]float64{1, 0.2851116, 96440480000, -19.12504}) // Grows to 1 as x -> 0

	timeTo := unixTime - time.Now().Unix()
	if timeTo < 0 {
		timeTo = 0
	}
	timeScale := fnTimeScale(float64(timeTo))

	return timeScale
}
