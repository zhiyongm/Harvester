package utils

type CSVEntity struct {
	From        string `csv:"from"`
	To          string `csv:"to"`
	Hash        string `csv:"transactionHash"`
	Value       string `csv:"value"`
	BlockNumber string `csv:"blockNumber"`
}
