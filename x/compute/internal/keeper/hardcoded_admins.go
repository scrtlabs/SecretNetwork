package keeper

// This map enables these gov-proposed contracts to have admin functionality even though they
// were created before the contract upgrade feature existed
var hardcodedContractAdmins = map[string]string{
	"secret1mfk7n6mc2cg6lznujmeckdh4x0a5ezf6hx6y8q": "secret1ap26qrlp8mcq2pg6r47w43l0y8zkqm8a450s03",
}
