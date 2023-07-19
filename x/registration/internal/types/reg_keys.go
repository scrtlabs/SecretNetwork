//go:build !test && !secretcli

package types

func GetApiKey() ([]byte, error) {
	apiKeyFile, err := Asset("api_key.txt") //nolint:all
	if err != nil {
		return nil, err
	}

	return apiKeyFile, nil
}

func GetSpid() ([]byte, error) {
	apiKeyFile, err := Asset("spid.txt") //nolint:all
	if err != nil {
		return nil, err
	}

	return apiKeyFile, nil
}
