package memory_read

import (
	"OrdDeFi-Virtual-Machine/db_utils"
	"OrdDeFi-Virtual-Machine/safe_number"
	"OrdDeFi-Virtual-Machine/virtual_machine/memory/memory_const"
)

/*
For a single version.
*/

func Balance(db *db_utils.OrdDB, coinName string, address string, version string) (*safe_number.SafeNum, error) {
	balanceKey := memory_const.CoinBalanceTable + ":v" + version + ":" + coinName + ":" + address
	balanceString, err := db.Read(balanceKey)
	if err != nil {
		if err.Error() == "leveldb: not found" {
			value := "0"
			balanceString = &value
		} else {
			return nil, err
		}
	}
	num := safe_number.SafeNumFromString(*balanceString)
	return num, nil
}

func AllCoinBalanceForAddress(db *db_utils.OrdDB, address string, version string) (map[string]string, error) {
	if version == "" {
		version = "1"
	}
	prefix := memory_const.AddressBalanceTable + ":v" + version + ":" + address + ":"
	result, err := db.ReadAllPrefix(prefix)
	return result, err
}

func AllAddressBalanceForCoin(db *db_utils.OrdDB, coinName string, version string) (map[string]string, error) {
	if version == "" {
		version = "1"
	}
	prefix := memory_const.CoinBalanceTable + ":v" + version + ":" + coinName + ":"
	result, err := db.ReadAllPrefix(prefix)
	return result, err
}
