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

// Next construye y devuelve el próximo batch que cumpla con los límites de
// cantidad y tamaño. Si no hay más datos, retorna io.EOF. No mantiene estado
// adicional salvo el del propio csv.Reader.
func (br *BatchFileReader) Next() ([]BetRecord, int64, error) {
	if br.closed {
		return nil, 0, io.EOF
	}
	batch := make([]BetRecord, 0, br.maxPerB)
	curSize := br.headerLen
	for {
		row, eof, err := br.readRow()
		if eof {
			return br.flushIfAny(batch)
		}
		if err != nil {
			continue
		}
		rec, ok := br.parseRow(row)
		if !ok {
			continue
		}
		ok, newSize := br.tryFit(&batch, curSize, rec)
		if ok {
			curSize = newSize
			if len(batch) >= br.maxPerB {
				return br.ret(batch)
			}
			continue
		}
		if len(batch) > 0 {
			return br.ret(batch)
		}
		log.Errorf("action: bet_oversize | result: drop_record")
	}
}

// readRow lee una fila del CSV y normaliza los casos de error y EOF.
// Devuelve la fila, un booleano indicando EOF, y un error si corresponde.
func (br *BatchFileReader) readRow() ([]string, bool, error) {
	row, err := br.reader.Read()
	if err != nil {
		if errors.Is(err, io.EOF) {
			return nil, true, nil
		}
		log.Errorf("action: csv_read | result: skip_row | error: %v", err)
		return nil, false, err
	}
	return row, false, nil
}

// parseRow valida y transforma la fila leída en un BetRecord. Devuelve el
// registro y si la conversión fue exitosa (para poder descartar filas inválidas).
func (br *BatchFileReader) parseRow(row []string) (BetRecord, bool) {
	if len(row) < 5 {
		log.Errorf("action: csv_parse | result: skip_row | reason: columns")
		return BetRecord{}, false
	}
	num, err := strconv.Atoi(row[4])
	if err != nil {
		log.Errorf("action: csv_parse | result: skip_row | number: %q | error: %v", row[4], err)
		return BetRecord{}, false
	}
	return BetRecord{
		Agency:    br.agencyID,
		FirstName: row[0],
		LastName:  row[1],
		Document:  row[2],
		Birthdate: row[3],
		Number:    num,
	}, true
}

// tryFit estima el tamaño serializado del registro y decide si entra en el batch
// según el límite de bytes y el máximo de elementos. Si entra, lo agrega.
func (br *BatchFileReader) tryFit(batch *[]BetRecord, curSize int, rec BetRecord) (bool, int) {
	b, err := SerializeOneBet(rec)
	if err != nil {
		log.Errorf("action: bet_serialize | result: skip_row | error: %v", err)
		return false, curSize
	}
	if len(*batch) >= br.maxPerB || curSize+len(b) > br.limit {
		return false, curSize
	}
	*batch = append(*batch, rec)
	return true, curSize + len(b)
}

// ret empaqueta el batch actual junto con el offset de archivo para logging.
func (br *BatchFileReader) ret(batch []BetRecord) ([]BetRecord, int64, error) {
	off, _ := br.FileOffset()
	return batch, off, nil
}

// flushIfAny devuelve el batch si contiene elementos; de lo contrario, io.EOF.
// Se usa al alcanzar el final del archivo.
func (br *BatchFileReader) flushIfAny(batch []BetRecord) ([]BetRecord, int64, error) {
	if len(batch) > 0 {
		return br.ret(batch)
	}
	return nil, 0, io.EOF
}