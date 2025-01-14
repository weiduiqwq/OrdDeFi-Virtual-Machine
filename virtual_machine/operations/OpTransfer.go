package operations

import (
	"OrdDeFi-Virtual-Machine/db_utils"
	"OrdDeFi-Virtual-Machine/safe_number"
	"OrdDeFi-Virtual-Machine/tx_utils"
	"OrdDeFi-Virtual-Machine/virtual_machine/instruction_set"
	"OrdDeFi-Virtual-Machine/virtual_machine/memory/memory_const"
	"OrdDeFi-Virtual-Machine/virtual_machine/memory/memory_read"
	"OrdDeFi-Virtual-Machine/virtual_machine/memory/memory_write"
	"errors"
	"fmt"
	"github.com/btcsuite/btcd/wire"
)

func performTransferBatchWriteKV(
	db *db_utils.OrdDB, coinName string,
	fromAccount string, fromSubAccount string,
	toAccount string, toSubAccount string,
	amount *safe_number.SafeNum) (map[string]string, error) {
	if amount == nil {
		return nil, errors.New("performTransferBatchWriteKV Transfer amount is nil")
	}
	if amount.IsZero() {
		return nil, errors.New("performTransferBatchWriteKV Transfer amount is 0")
	}
	var fromBalance *safe_number.SafeNum
	var toBalance *safe_number.SafeNum
	var err error
	// read from balance
	if fromSubAccount == db_utils.AvailableSubAccount {
		fromBalance, err = memory_read.AvailableBalance(db, coinName, fromAccount)
	} else if fromSubAccount == db_utils.TransferableSubAccount {
		fromBalance, err = memory_read.TransferableBalance(db, coinName, fromAccount)
	} else {
		return nil, errors.New("performTransferBatchWriteKV Sub-account error: " + fromSubAccount)
	}
	if err != nil {
		return nil, err
	}
	if fromBalance == nil {
		return nil, errors.New("performTransferBatchWriteKV failed: fromBalance is nil")
	}
	// read to balance
	if toSubAccount == db_utils.AvailableSubAccount {
		toBalance, err = memory_read.AvailableBalance(db, coinName, toAccount)
	} else if toSubAccount == db_utils.TransferableSubAccount {
		toBalance, err = memory_read.TransferableBalance(db, coinName, toAccount)
	} else {
		return nil, errors.New("performTransferBatchWriteKV Sub-account error: " + toSubAccount)
	}
	if err != nil {
		return nil, err
	}
	if toBalance == nil {
		return nil, errors.New("performTransferBatchWriteKV failed: toBalance is nil")
	}
	fromBalanceUpdated := fromBalance.Subtract(amount)
	if fromBalanceUpdated == nil {
		return nil, fmt.Errorf("performTransferBatchWriteKV from address balance error: %s - %s", fromBalance.String(), amount.String())
	}
	if fromBalanceUpdated.IsNegative() {
		return nil, fmt.Errorf("performTransferBatchWriteKV from address balance error: negative %s", fromBalanceUpdated.String())
	}
	toBalanceUpdated := toBalance.Add(amount)
	if toBalanceUpdated == nil {
		return nil, fmt.Errorf("performTransferBatchWriteKV to address balance error: %s + %s", toBalance.String(), amount.String())
	}
	if toBalanceUpdated.IsNegative() {
		return nil, fmt.Errorf("performTransferBatchWriteKV from address balance error: negative %s", toBalanceUpdated.String())
	}
	updatedSum := fromBalanceUpdated.Add(toBalanceUpdated)
	if updatedSum == nil {
		return nil, errors.New("performTransferBatchWriteKV failed: updatedSum is nil")
	}
	beforeUpdatedSum := fromBalance.Add(toBalance)
	if beforeUpdatedSum == nil {
		return nil, errors.New("performTransferBatchWriteKV failed: beforeUpdatedSum is nil")
	}
	if updatedSum.IsEqualTo(beforeUpdatedSum) == false {
		return nil, fmt.Errorf("performTransferBatchWriteKV before calculation and after are not equal")
	}
	updateFromKV := memory_write.CoinBalanceDoubleWriteKV(coinName, fromAccount, fromBalanceUpdated.String(), fromSubAccount)
	if updateFromKV == nil {
		return nil, errors.New("performTransferBatchWriteKV updateFromKV generating error")
	}
	updateToKV := memory_write.CoinBalanceDoubleWriteKV(coinName, toAccount, toBalanceUpdated.String(), toSubAccount)
	if updateToKV == nil {
		return nil, errors.New("performTransferBatchWriteKV updateToKV generating error")
	}
	for k, v := range updateToKV {
		updateFromKV[k] = v
	}
	return updateFromKV, nil
}

func executeImmediateTransfer(instruction instruction_set.OpTransferInstruction, db *db_utils.OrdDB) error {
	amountSafeNum := safe_number.SafeNumFromString(instruction.Amt)
	if amountSafeNum == nil {
		return nil
	}
	// remove from current address available, add to "to" address available
	batchKV, err := performTransferBatchWriteKV(
		db, instruction.Tick,
		instruction.TxOutAddr, db_utils.AvailableSubAccount,
		instruction.To, db_utils.AvailableSubAccount,
		amountSafeNum,
	)
	if err != nil {
		return err
	}
	err = db.StoreKeyValues(batchKV)
	return err
}

func executeUTXOTransfer(instruction instruction_set.OpTransferInstruction, db *db_utils.OrdDB) error {
	amountSafeNum := safe_number.SafeNumFromString(instruction.Amt)
	if amountSafeNum == nil {
		return nil
	}
	// remove from current address available, add to current address transferable
	batchKV, err := performTransferBatchWriteKV(
		db, instruction.Tick,
		instruction.TxOutAddr, db_utils.AvailableSubAccount,
		instruction.TxOutAddr, db_utils.TransferableSubAccount,
		amountSafeNum,
	)
	if err != nil {
		return err
	}
	// save a record on UTXOCarryingBalance:txId:0:coinName, content: amountString
	utxoCarryingBalancePath := memory_const.UTXOCarryingBalancePath(instruction.TxId)
	batchKV[utxoCarryingBalancePath] = instruction.TxOutAddr + ":" + instruction.Tick + ":" + amountSafeNum.String()
	err = db.StoreKeyValues(batchKV)
	return err
}

func ExecuteTransfer(instruction instruction_set.OpTransferInstruction, db *db_utils.OrdDB) error {
	if instruction.To != "" {
		return executeImmediateTransfer(instruction, db)
	} else {
		return executeUTXOTransfer(instruction, db)
	}
}

type outputLocationMap struct {
	satLocation int64
	address     string
}

func txOutputSatMap(tx *wire.MsgTx) ([]outputLocationMap, error) {
	// int64 range [-9223372036854775808, 9223372036854775807] covers 2100000000000000
	var result []outputLocationMap
	var currentSat int64
	currentSat = 0
	for _, output := range tx.TxOut {
		address, err := tx_utils.ParseOutputAddress(output)
		if err != nil {
			return nil, err
		}
		if address == nil {
			// output is OpReturn
			continue
		}
		mapObject := new(outputLocationMap)
		mapObject.satLocation = currentSat
		mapObject.address = *address
		result = append(result, *mapObject)
		currentSat = currentSat + output.Value
	}
	// Appending tail info, for calculating transaction fee(gas) burning sats
	mapObject := new(outputLocationMap)
	mapObject.satLocation = currentSat
	mapObject.address = ""
	result = append(result, *mapObject)
	return result, nil
}

func containsTransferUTXOInTxIn(db *db_utils.OrdDB, tx *wire.MsgTx) (bool, error) {
	// tx nil protection
	if tx == nil {
		return false, errors.New("tx is nil")
	}
	// If one TxIn contains coins, returns true. Otherwise returns false.
	contains := false
	for _, input := range tx.TxIn {
		// previousOutputIndex != 0 cannot be a UTXO which contains coins.
		previousOutputIndex := input.PreviousOutPoint.Index
		if previousOutputIndex != 0 {
			// All UTXOs `carrying transferable tokens` are created at TxOut[0].
			// So if the `previousOutputIndex` is not 0, the input UTXO could not be a `carrying transferable tokens` UTXO.
			continue
		}
		// Query coin info from DB:
		previousTxId := input.PreviousOutPoint.Hash.String()
		address, tick, amount, err := memory_read.UTXOCarryingBalance(db, previousTxId)
		// Return error if DB returns error
		if err != nil {
			return false, err
		}
		if address == nil && tick == nil && amount == nil {
			// This UTXO contains nothing, let's seek the next one
			continue
		} else if address != nil && tick != nil && amount != nil {
			// Found coin in one UTXO, then the whole tx contains UTXO
			contains = true
			break
		} else {
			// Shouldn't been there, something wrong at DB writing.
			return false, errors.New("containsTransferUTXOInTxIn error: DB interrupted")
		}
	}
	return contains, nil
}

func calculatingAddress(outputMap []outputLocationMap, satIndex int64, selfTransferAddress string) string {
	for _, object := range outputMap {
		if object.satLocation < satIndex {
			continue
		} else {
			address := object.address
			return address
		}
	}
	// If a transfer UTXO was burnt as fee, regard the burning tx as a self-transfer tx.
	return selfTransferAddress
}

func ApplyUTXOTransfer(db *db_utils.OrdDB, tx *wire.MsgTx) (bool, error) {
	if tx == nil {
		return false, errors.New("tx is nil")
	}
	anyInputCarryingCoins, err := containsTransferUTXOInTxIn(db, tx)
	if err != nil {
		return false, err
	}
	if anyInputCarryingCoins == false {
		return false, nil
	}
	// 0. Generate output index map
	outputSatMap, err := txOutputSatMap(tx)
	if err != nil {
		return false, err
	}
	// perform transfer coins
	// 1. Keeps the current sat index in `currentInputSatIndex` and `nextInputSatIndex`
	var currentInputSatIndex int64
	var nextInputSatIndex int64
	currentInputSatIndex = 0
	nextInputSatIndex = 0
	for _, input := range tx.TxIn {
		// 2. Shift sat index from last loop `nextInputSatIndex` into `currentInputSatIndex`
		currentInputSatIndex = nextInputSatIndex
		// 3. Parse input, save input value for calculating UTXO (which carrying coins) routing info
		inputAddress, inputValue, err := tx_utils.ParseInputAddressAndValue(input)
		if err != nil {
			return false, err
		}
		if inputAddress == nil || inputValue == nil {
			// This is a coinbase tx, which doesn't contain input.
			// So there won't be any witness script or UTXO moving instruction.
			return false, nil
		}
		// 3.1 Record input sat index for next loop
		nextInputSatIndex = nextInputSatIndex + *inputValue

		// 4. Check UTXO routing
		// 4.1 previousOutputIndex != 0 cannot be an UTXO which contains coins. Because the UTXO cannot receive OrdDeFi instruction.
		previousOutputIndex := input.PreviousOutPoint.Index
		if previousOutputIndex != 0 {
			// All UTXOs `carrying transferable tokens` are created at TxOut[0].
			// So if the `previousOutputIndex` is not 0, the input UTXO could not be a `carrying transferable tokens` UTXO.
			continue
		}
		// 4.2 Query coin info from DB:
		previousTxId := input.PreviousOutPoint.Hash.String()
		address, tick, amount, err := memory_read.UTXOCarryingBalance(db, previousTxId)
		// 4.3 Return error if DB returns error
		if err != nil {
			return false, err
		}
		if address == nil && tick == nil && amount == nil {
			// 4.4 This UTXO contains nothing, let's seek the next one
			continue
		} else if address != nil && tick != nil && amount != nil {
			// 5 Update DB
			// 5.1 Calculate with address should this UTXO run into
			toAddress := calculatingAddress(outputSatMap, currentInputSatIndex, *inputAddress)
			if toAddress == "" {
				return false, errors.New("calculatingAddress error, toAddress is nil")
			}
			// 5.2 Generate updating value KV
			batchKV, err := performTransferBatchWriteKV(
				db, *tick,
				*inputAddress, db_utils.TransferableSubAccount,
				toAddress, db_utils.AvailableSubAccount,
				amount,
			)
			if err != nil {
				return false, err
			}
			// 5.3 Remove UTXO carrying coins info
			batchKV[previousTxId] = ""
			err = db.StoreKeyValues(batchKV)
			if err != nil {
				return false, err
			}
			continue
		} else {
			// 4.5 Shouldn't been there, something wrong at DB writing.
			return false, errors.New("containsTransferUTXOInTxIn error: DB interrupted")
		}
	}
	return true, nil
}
