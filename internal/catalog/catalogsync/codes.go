package catalogsync

// sv42Fuel maps KBA fuel/energy-source codes (SV 4.2 column 9) to torq's fuel
// types. Only codes with a certain meaning are listed; any other code becomes
// "other" and is listed in the sync report so the table can be extended.
var sv42Fuel = map[string]string{
	"0001": "petrol",
	"0002": "diesel",
	"0004": "electric",
	"0005": "lpg",
	"0009": "cng",
}
