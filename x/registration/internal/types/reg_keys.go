//go:build !test && !secretcli

package types

func GetApiKey() ([]byte, error) {
	return nil, nil
	//apiKeyFile, err := Asset("api_key.txt")
	//if err != nil {
	//	return nil, err
	//}
	//
	//return apiKeyFile, nil
}

func GetSpid() ([]byte, error) {
	return nil, nil
	//apiKeyFile, err := Asset("spid.txt")
	//if err != nil {
	//	return nil, err
	//}
	//
	//return apiKeyFile, nil
}
