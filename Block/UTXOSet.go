package Block

import (
	"log"
	"github.com/boltdb/bolt"
	"encoding/hex"
)
const utxoBucket = "chainstate"

type UTXOSet struct{
	Blockchain *BlockChain
}

func (u UTXOSet) Reindex(){
	db := u.Blockchain.DB
	bucketName := []byte(utxoBucket)
	err := db.Update(func(tx *bolt.Tx) error{
		err := tx.DeleteBucket(bucketName)
		if err != nil && err != bolt.ErrBucketNotFound{
			log.Panic(err)
		}

		_,err =tx.CreateBucket(bucketName)
		if err != nil{
			log.Panic(err)
		}
		return nil
	})

	if err != nil {
		log.Panic(err)
	}

	UTXO := u.Blockchain.FindUTXO()
	err = db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketName)
		for txID,outs := range UTXO {
			key ,err := hex.DecodeString(string(txID))
			if err != nil {
				log.Panic(err)
			}
			err = b.Put(key,outs.Serialize())
			if err != nil {
				log.Panic(err)
			}
		}
		return nil
	})
}

func (u UTXOSet) FindSpendableOutputs(pubKeyHash []byte , amount int) (int , map[string][]int){
	unspentOutputs := make(map[string][]int)
	accumulate := 0
	db := u.Blockchain.DB

	err := db.View(func(tx *bolt.Tx) error{
		b := tx.Bucket([]byte(utxoBucket))
		c := b.Cursor()

		for k,v := c.First();k != nil;k,v = c.Next(){
			txID := hex.EncodeToString(k)
			outs := DeserializeOutputs(v)

			for outIdx,out := range outs.Outputs {
				if out.IsLockedWithKey(pubKeyHash) && accumulate < amount {
					accumulate += out.Value
					unspentOutputs[txID] = append(unspentOutputs[txID],outIdx)
				}
			}
		}
		return nil
	})
	if err != nil {
		log.Panic(err)
	}
	return accumulate,unspentOutputs
}

func (u UTXOSet) FindUTXO (pubKeyHash []byte) []TXOutput {
	var UTXOs []TXOutput
	db := u.Blockchain.DB

	err := db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(utxoBucket))
		c := b.Cursor()

		for k,v := c.First();k != nil;k,v = c.Next(){
			outs := DeserializeOutputs(v)

			for _,out := range outs.Outputs {
				if out.IsLockedWithKey(pubKeyHash) {
					UTXOs = append(UTXOs,out)
				}
			}
		}
		return nil
	})
	if err != nil {
		log.Panic(err)
	}
	return UTXOs
}


func (u UTXOSet) Update(block *Block){
	db := u.Blockchain.DB
	err := db.Update(func(tx *bolt.Tx) error{
		b := tx.Bucket([]byte(utxoBucket))

		for _,tx := range block.Transactions{
			if tx.IsCoinbase() == false {
				for _,vin := range tx.Vin{
					updatedOuts := TXOutputs{}
					outsBytes := b.Get(vin.Txid)
					outs := DeserializeOutputs(outsBytes)

					for outIdx,out := range outs.Outputs{
						if outIdx != vin.Vout{
							updatedOuts.Outputs = append(updatedOuts.Outputs,out)
						}
					}

					if len(updatedOuts.Outputs) == 0{
						err := b.Delete(vin.Txid)
						if err != nil {
							log.Panic(err)
						}
					}else{
						err := b.Put(vin.Txid,updatedOuts.Serialize())
						if err != nil {
							log.Panic(err)
						}
					}
				}
			}

			newOutputs := TXOutputs{}

			for _, out := range tx.Vout{
				newOutputs.Outputs = append(newOutputs.Outputs,out)
			}

			err := b.Put(tx.ID,newOutputs.Serialize())
			if err != nil {
				log.Panic(err)
			}
		}
		return nil
	})
	if err != nil {
		log.Panic(err)
	}

}