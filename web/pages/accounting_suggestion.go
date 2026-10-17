package pages

type SuggestedAccount struct {
	Side, Code, Name, Amount string
}

type AccountingReference struct {
	ID     string `json:"id"`
	Source string `json:"source"`
}
