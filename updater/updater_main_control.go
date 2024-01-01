package updater

import (
	"OrdDeFi-Virtual-Machine/bitcoin_cli_channel"
	"OrdDeFi-Virtual-Machine/db_utils"
	"errors"
)

const controlDBPath = "./OrdDeFi_control"
const genesisBlockNumber = 829832

func UpdateIndex(dataDir string, logDir string, verbose bool) error {
	println("The Times 03/Jan/2009 Chancellor on brink of second bailout for banks.")
	println("OrdDeFi indexer start to work.")

	// check bitcoin-cli requirements
	reachedMinRequirement, err := bitcoin_cli_channel.VersionGreaterThanMinRequirement()
	if err != nil {
		return err
	}
	if *reachedMinRequirement == false {
		return errors.New("bitcoin-cli version lower than 24.0.1")
	}

	// open db
	controlDB, err := db_utils.OpenDB(controlDBPath)
	if err != nil {
		return err
	}
	defer db_utils.CloseDB(controlDB)

	// check current block number
	currentBlockNumber := bitcoin_cli_channel.GetBlockCount()
	if currentBlockNumber == 0 {
		err := errors.New("updateIndex error: bitcoin-cli getblockcount failed")
		return err
	}
	for indexingBlockNumber := genesisBlockNumber; indexingBlockNumber <= currentBlockNumber; indexingBlockNumber++ {
		// get block hash and all txIds in block
		println("indexing block", indexingBlockNumber)
		blockHash := bitcoin_cli_channel.GetBlockHash(indexingBlockNumber)
		if blockHash == nil {
			return errors.New("UpdateBlockNumber GetBlockHash failed")
		}
		err = UpdateBlockNumber(indexingBlockNumber, blockHash, dataDir, logDir, verbose)
		if err != nil {
			return err
		}
	}
	return nil
}