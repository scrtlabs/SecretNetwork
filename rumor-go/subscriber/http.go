package subscriber

import (
	"encoding/json"
	"io/ioutil"
	"net/http"

	"github.com/enigmampc/SecretNetwork/rumor-go/types"
	"github.com/enigmampc/SecretNetwork/rumor-go/utils"
)

type BlockGetter func(height interface{}) (*types.Block, error)

func GetBlock(endpoint string) (*types.Block, error) {
	res, err := http.Get(endpoint)
	if err != nil {
		return nil, err
	}

	resbytes, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	data := new(struct {
		Result struct {
			Block json.RawMessage
		} `json:"result"`
	})

	if unmarshalErr := json.Unmarshal(resbytes, data); unmarshalErr != nil {
		panic(unmarshalErr)
	}

	block := utils.ConvertBlockHeaderToTMHeader(data.Result.Block)

	return &block, nil
}
