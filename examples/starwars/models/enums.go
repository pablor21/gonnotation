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

// Test various iota expressions
type TestEnum1 int

const (
	Test1_A TestEnum1 = iota * 2 // 0 * 2 = 0
	Test1_B                      // 1 * 2 = 2
	Test1_C                      // 2 * 2 = 4
)

type TestEnum2 int

const (
	Test2_A TestEnum2 = iota - 1 // 0 - 1 = -1
	Test2_B                      // 1 - 1 = 0
	Test2_C                      // 2 - 1 = 1
)

type TestEnum3 int

const (
	Test3_A TestEnum3 = iota + 100 // 0 + 100 = 100
	Test3_B                        // 1 + 100 = 101
	Test3_C                        // 2 + 100 = 102
)
