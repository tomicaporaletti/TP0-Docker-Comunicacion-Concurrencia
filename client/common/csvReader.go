package common

import (
	"encoding/csv"
	"os"
	"strconv"
)

// readBetsFromCSV abre el archivo y devuelve todas las apuestas
func readBetsFromCSV(path string) ([]BetRecord, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	reader := csv.NewReader(f)
	reader.TrimLeadingSpace = true

	var bets []BetRecord
	for {
		row, err := reader.Read()
		if err != nil {
			break // EOF
		}
		num, _ 		:= strconv.Atoi(row[5])
		agency, _ 	:= strconv.Atoi(row[0])
		bets = append(bets, BetRecord{
			Agency:    agency,
			FirstName: row[1],
			LastName:  row[2],
			Document:  row[3],
			Birthdate: row[4],
			Number:    num,
		})
	}
	return bets, nil
}

// splitIntoBatches corta una lista de bets en sublistas que cumplen:
//  - como maximo "max" bets
//  - el tamaño serializado total no supera los 8KB
func splitIntoBatches(bets []BetRecord, max int) [][]BetRecord {
    const limit = 8 * 1024 // 8 KB
    var batches [][]BetRecord
    var current []BetRecord
    currentSize := 1 + 2 // [1 byte type] + [2 bytes cantidad]

    for _, bet := range bets {
        betBytes, err := SerializeOneBet(bet)
        if err != nil {
			log.Errorf("action: Batch Serialization | result: fail | error: %v",err)
            continue
        }
        betSize := len(betBytes)

        // Si agregar esta bet supera límite de 8KB o max cantidad → flush
        if len(current) >= max || currentSize+betSize > limit {
            if len(current) > 0 {
                batches = append(batches, current)
            }
            current = []BetRecord{}
            currentSize = 1 + 2
        }

        current = append(current, bet)
        currentSize += betSize
    }

    if len(current) > 0 {
        batches = append(batches, current)
    }

    return batches
}
