package common

import (
	"encoding/json"
	"os"
	"strconv"
)

// BetMessage representa la apuesta que manda el cliente
type BetMessage struct {
	Type      string `json:"type"`
	Agency    string `json:"agency"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Document  string `json:"document"`
	Birthdate string `json:"birthdate"`
	Number    int    `json:"number"`
}

// NewBetFromEnv construye un BetMessage a partir de las variables de entorno
func NewBetFromEnv(agency string) (*BetMessage, error) {
	numStr := os.Getenv("NUMERO")
	number, err := strconv.Atoi(numStr)
	if err != nil {
		return nil, err
	}
	return &BetMessage{
		Type:      "bet",
		Agency:    agency,
		FirstName: os.Getenv("NOMBRE"),
		LastName:  os.Getenv("APELLIDO"),
		Document:  os.Getenv("DOCUMENTO"),
		Birthdate: os.Getenv("NACIMIENTO"),
		Number:    number,
	}, nil
}

// Serialize serializa la apuesta como JSON terminado en '\n'
func (b *BetMessage) Serialize() (string, error) {
	data, err := json.Marshal(b)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
