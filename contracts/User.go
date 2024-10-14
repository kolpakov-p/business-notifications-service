package contracts

type User struct {
	Id        string `json:"id"`
	TenantId  string `json:"tenantId"`
	Phone     string `json:"phone,omitempty"`
	Email     string `json:"email,omitempty"`
	Firstname string `json:"firstName,omitempty"`
	Lastname  string `json:"lastName,omitempty"`
	Country   string `json:"country"`
	Language  string `json:"language"`
}
