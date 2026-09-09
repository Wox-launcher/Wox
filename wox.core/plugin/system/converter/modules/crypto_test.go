package modules

import (
	"os"
	"testing"
	"wox/util"

	"github.com/stretchr/testify/assert"
)

func TestFetchCryptoPrices(t *testing.T) {
	if os.Getenv("WOX_TEST_ENABLE_NETWORK") == "false" {
		t.Skip("external price service disabled by WOX_TEST_ENABLE_NETWORK")
	}
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
