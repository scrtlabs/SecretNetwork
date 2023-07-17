//go:build !test && !secretcli

package types

func GetApiKey() ([]byte, error) {
	apiKeyFile, err := Asset("api_key.txt") //nolint:typecheck
	if err != nil {
		return nil, err
	}

	return apiKeyFile, nil
}

func GetSpid() ([]byte, error) {
	apiKeyFile, err := Asset("spid.txt") //nolint:typecheck
	if err != nil {
		return nil, err
	}

	return apiKeyFile, nil
}
