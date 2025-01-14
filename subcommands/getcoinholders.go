package subcommands

import (
	"OrdDeFi-Virtual-Machine/db_utils"
	"OrdDeFi-Virtual-Machine/virtual_machine/memory/memory_read"
	"os"
)

func GetCoinHolders(coinName string, dataDir string) {
	db, err := db_utils.OpenDB(dataDir)
	if err != nil {
		println("open db error:", err.Error())
		os.Exit(19)
	}
	defer db_utils.CloseDB(db)

	r, err := memory_read.AllAddressBalanceForCoin(db, coinName)
	if err != nil {
		println("GetCoinHoldersParam read AllAddressBalanceForCoin error:", err.Error())
		os.Exit(20)
	}
	for k, v := range r {
		println(k, ":", v)
	}
}
