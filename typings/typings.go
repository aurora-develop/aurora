package typings

type GenericResponseLine struct {
	Line  string `json:"line"`
	Error string `json:"error"`
}

type StringStruct struct {
	Text string `json:"text"`
}
