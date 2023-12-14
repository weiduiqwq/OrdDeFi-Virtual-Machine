package updater

import (
	"OrdDefi-Virtual-Machine/bitcoin_cli_channel"
	"OrdDefi-Virtual-Machine/inscription_parser"
	"OrdDefi-Virtual-Machine/tx_utils"
	"OrdDefi-Virtual-Machine/virtual_machine"
	"errors"
)

func UpdateBlockNumber(blockNumber int) {
	var err error
	blockHash := bitcoin_cli_channel.GetBlockHash(blockNumber)
	if blockHash == nil {
		err = errors.New("UpdateBlockNumber GetBlockHash failed")
		return
	}
	block := bitcoin_cli_channel.GetBlock(*blockHash)
	for _, txId := range block.Tx {
		rawTx := bitcoin_cli_channel.GetRawTransaction(txId)
		if rawTx == nil {
			err = errors.New("GetRawTransaction Failed")
			break
		}
		tx := bitcoin_cli_channel.DecodeRawTransaction(*rawTx)
		if tx == nil {
			err = errors.New("ParseRawTransaction -> DecodeRawTransaction Failed")
			break
		}
		contentType, content, err := inscription_parser.ParseTransactionToInscription(*tx)
		if err != nil {
			break
		}
		if contentType != nil && content != nil {
			virtual_machine.CompileInstructions(*contentType, content, tx, txId)
			println("txId", txId)
			firstInputAddress, err := tx_utils.ParseFirstInputAddress(tx)
			if err != nil || firstInputAddress == nil {
				break
			}
			println("input", *firstInputAddress)
			firstOutputAddress, err := tx_utils.ParseFirstOutputAddress(tx)
			if err != nil || firstOutputAddress == nil {
				break
			}
			println("output", *firstOutputAddress)
			println(*contentType, len(content))
			println(string(content))
		}
	}
	if err != nil {
		println("Updating block got error:", err) // failing
		println("Aborting update blocks...")
	}
}
