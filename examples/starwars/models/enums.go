package models

type EnumIota int

const (
	EnumIota1 EnumIota = iota + 5
	EnumIota2
	EnumIota3
	enumIotaPrivate
)

type EnumString string

const (
	EnumString1       EnumString = "Option1"
	EnumString2       EnumString = "Option2"
	EnumString3       EnumString = "Option3"
	enumStringPrivate EnumString = "privateOption"
)
