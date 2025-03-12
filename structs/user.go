package structs

type User struct {
	Email    string  `json:"email"`
	Name     string  `json:"name"`
	GoogleID *string `json:"id"`
	Password *string `json:"password"`
}
