package modules

import (
	"strings"
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
	if err != nil {
		if strings.HasPrefix(err.Error(), "failed to fetch prices:") {
			t.Skipf("skip CoinGecko integration check because the external API is unreachable: %v", err)
		}
		t.Fatalf("fetch crypto prices: %v", err)
	}
	assert.NotEmpty(t, prices)
}
