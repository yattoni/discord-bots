package usgs

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestFetchQuakes(t *testing.T) {
	queryResults := NewClient().FetchQuakes(time.Now().UTC(), time.Now().UTC())

	for _, feature := range queryResults.Features {
		fmt.Println(feature)
	}

	assert.Equal(t, 1, 2)
}
