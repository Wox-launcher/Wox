package modules

import (
	"testing"
	"wox/util"

	"github.com/stretchr/testify/assert"
)

func TestFetchCryptoPrices(t *testing.T) {
	ctx := util.NewTraceContext()
	err := util.GetLocation().Init()
	if err != nil {
		panic(err)
	}

	module := &CryptoModule{}
	prices, err := module.fetchCryptoPrices(ctx)
	assert.NoError(t, err)
	assert.NotEmpty(t, prices)
}
