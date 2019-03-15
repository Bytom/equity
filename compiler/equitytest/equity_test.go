package equitytest

import (
	"bufio"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/equity/compiler"
)

func TestCompileContract(t *testing.T) {
	cases := []struct {
		pathFile string
		want     string
	}{
		{
			"./LockPosition",
			"cd9f697b7bae7cac6900c3c251547ac1",
		},
		{
			"./RepayCollateral",
			"557a641f0000007bcda069007b7b51547ac16951c3c251547ac1632a0000007bcd9f6900c3c251567ac1",
		},
		{
			"./LoanCollateral",
			"567a64650000007bcda06900c3537ac2547a5100597989587a89577a89557a89537a8901747e2a557a641f0000007bcda069007b7b51547ac16951c3c251547ac1632a0000007bcd9f6900c3c251567ac189008901c07ec16951c3c251547ac163700000007bcd9f6900c3c251577ac1",
		},
		{
			"./FixedLimitCollect",
			"597a642f0200005479cda069c35b797ca153795579a19a695a790400e1f5059653790400e1f505967c00a07c00a09a69c35b797c9f9161644d010000005b79c2547951005e79895d79895c79895b7989597989587989537a894caa587a649e0000005479cd9f6959790400e1f5059653790400e1f505967800a07800a09a5c7956799f9a6955797b957c96c37800a052797ba19a69c3787c9f91616487000000005b795479515b79c1695178c2515d79c16952c3527994c251005d79895c79895b79895a79895979895879895779895679890274787e008901c07ec1696399000000005b795479515b79c16951c3c2515d79c16963aa000000557acd9f69577a577aae7cac890274787e008901c07ec169515b79c2515d79c16952c35c7994c251005d79895c79895b79895a79895979895879895779895679895579890274787e008901c07ec169632a020000005b79c2547951005e79895d79895c79895b7989597989587989537a894caa587a649e0000005479cd9f6959790400e1f5059653790400e1f505967800a07800a09a5c7956799f9a6955797b957c96c37800a052797ba19a69c3787c9f91616487000000005b795479515b79c1695178c2515d79c16952c3527994c251005d79895c79895b79895a79895979895879895779895679890274787e008901c07ec1696399000000005b795479515b79c16951c3c2515d79c16963aa000000557acd9f69577a577aae7cac890274787e008901c07ec16951c3c2515d79c169633b020000547acd9f69587a587aae7cac",
		},
		{
			"./FixedLimitProfit",
			"587a649e0000005479cd9f6959790400e1f5059653790400e1f505967800a07800a09a5c7956799f9a6955797b957c96c37800a052797ba19a69c3787c9f91616487000000005b795479515b79c1695178c2515d79c16952c3527994c251005d79895c79895b79895a79895979895879895779895679890274787e008901c07ec1696399000000005b795479515b79c16951c3c2515d79c16963aa000000557acd9f69577a577aae7cac",
		},
	}

	for _, c := range cases {
		contractName := filepath.Base(c.pathFile)
		t.Run(contractName, func(t *testing.T) {
			absPathFile, err := filepath.Abs(c.pathFile)
			if err != nil {
				t.Fatal(err)
			}

			if _, err := os.Stat(absPathFile); err != nil {
				t.Fatal(err)
			}

			inputFile, err := os.Open(absPathFile)
			if err != nil {
				t.Fatal(err)
			}
			defer inputFile.Close()

			inputReader := bufio.NewReader(inputFile)
			contracts, err := compiler.Compile(inputReader)
			if err != nil {
				t.Fatal(err)
			}

			contract := contracts[len(contracts)-1]
			got := hex.EncodeToString(contract.Body)
			if got != c.want {
				t.Errorf("%s got %s\nwant %s", contractName, got, c.want)
			}
		})
	}
}
