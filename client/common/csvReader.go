package common

import (
	"encoding/csv"
	"errors"
	"io"
	"os"
	"strconv"
)

var ErrTooLargeRecord = errors.New("single record exceeds limit")

// BatchFileReader lee un CSV y arma batches on-demand, sin cargar todo en memoria.
type BatchFileReader struct {
	file      *os.File
	reader    *csv.Reader
	agencyID  int
	maxPerB   int
	limit     int // bytes, límite del payload serializado del batch
	headerLen int // overhead fijo del batch (1 byte type + 2 bytes cantidad)
	closed    bool
}

// Abre el archivo y deja listo el iterador.
func NewBatchFileReader(path string, agencyID, maxPerBatch int) (*BatchFileReader, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	r := csv.NewReader(f)
	r.TrimLeadingSpace = true

	return &BatchFileReader{
		file:      f,
		reader:    r,
		agencyID:  agencyID,
		maxPerB:   maxPerBatch,
		limit:     8 * 1024, // 8 KB
		headerLen: 1 + 2,    // [type:1] + [cantidad:2]
	}, nil
}

// Close cierra el archivo.
func (br *BatchFileReader) Close() error {
	br.closed = true
	if br.file != nil {
		return br.file.Close()
	}
	return nil
}

// FileOffset devuelve el offset actual del archivo
func (br *BatchFileReader) FileOffset() (int64, error) {
	return br.file.Seek(0, io.SeekCurrent)
}


// Next devuelve el próximo batch (slice de BetRecord) y, opcionalmente, el offset
// final luego de leer ese batch. Si no hay más datos, err == io.EOF.
func (br *BatchFileReader) Next() ([]BetRecord, int64, error) {
	if br.closed {
		return nil, 0, io.EOF
	}

	var batch []BetRecord
	currentSize := br.headerLen

	for {
		row, err := br.reader.Read()
		if err != nil {
			if errors.Is(err, io.EOF) {
				if len(batch) > 0 {
					off, _ := br.FileOffset()
					return batch, off, nil
				}
				return nil, 0, io.EOF
			}
			// línea inválida -> la saltamos y seguimos
			log.Errorf("action: csv_read | result: skip_row | error: %v", err)
			continue
		}

		// Esperamos formato: [FirstName, LastName, Document, Birthdate, Number]
		num, convErr := strconv.Atoi(row[4])
		if convErr != nil {
			log.Errorf("action: csv_parse | result: skip_row | number: %q | error: %v", row[4], convErr)
			continue
		}

		rec := BetRecord{
			Agency:    br.agencyID,
			FirstName: row[0],
			LastName:  row[1],
			Document:  row[2],
			Birthdate: row[3],
			Number:    num,
		}

		betBytes, serErr := SerializeOneBet(rec)
		if serErr != nil {
			log.Errorf("action: bet_serialize | result: skip_row | error: %v", serErr)
			continue
		}
		betSize := len(betBytes)

		
		if len(batch) >= br.maxPerB || currentSize+betSize > br.limit {
			if len(batch) > 0 {
				off, _ := br.FileOffset()
				return batch, off, nil
			}
			log.Errorf("action: bet_oversize | result: drop_record | bet_bytes: %d | limit: %d", betSize, br.limit-br.headerLen)
			continue
		}

		batch = append(batch, rec)
		currentSize += betSize

		if len(batch) >= br.maxPerB {
			off, _ := br.FileOffset()
			return batch, off, nil
		}

	}
}


// readBetsFromCSV abre el archivo y devuelve todas las apuestas
func readBetsFromCSV(path string, agencyID int) ([]BetRecord, error) {
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
		// row: [FirstName, LastName, Document, Birthdate, Number]
		num, _ := strconv.Atoi(row[4])
		bets = append(bets, BetRecord{
			Agency:    agencyID,
			FirstName: row[0],
			LastName:  row[1],
			Document:  row[2],
			Birthdate: row[3],
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
