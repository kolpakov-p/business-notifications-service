package contracts

type CustomerRegisteredEvent struct {
	Pattern Subject
	Data    struct {
		Payload User `json:"payload"`
	} `json:"data"`
}
