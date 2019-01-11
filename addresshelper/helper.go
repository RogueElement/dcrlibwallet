package addresshelper

import (
	"fmt"
	"strings"

	"github.com/decred/dcrd/chaincfg"
	"github.com/decred/dcrd/dcrutil"
	"github.com/decred/dcrd/txscript"
)

func PkScript(address string) ([]byte, error) {
	addr, err := dcrutil.DecodeAddress(address)
	if err != nil {
		return nil, fmt.Errorf("error decoding address '%s': %s", address, err.Error())
	}

	return txscript.PayToAddrScript(addr)
}

func DecodeForNetwork(address string, params *chaincfg.Params) (dcrutil.Address, error) {
	addr, err := dcrutil.DecodeAddress(address)
	if err != nil {
		return nil, err
	}
	if !addr.IsForNet(params) {
		return nil, fmt.Errorf("address %s is not intended for use on %s", address, params.Name)
	}
	return addr, nil
}

func AddressFromPkScript(params *chaincfg.Params, pkScript []byte) (address string, err error) {
	_, addresses, _, err := txscript.ExtractPkScriptAddrs(txscript.DefaultScriptVersion, pkScript, params)
	if err != nil {
		return
	}

	encodedAddresses := make([]string, len(addresses))
	for i, address := range addresses {
		encodedAddresses[i] = address.EncodeAddress()
	}

	return strings.Join(encodedAddresses, ", "), nil
}