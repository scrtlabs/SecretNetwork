package subscriber

import (
	"encoding/json"
	"log"
	"time"

	websocket "github.com/gorilla/websocket"

	types "github.com/enigmampc/SecretNetwork/rumor-go/types"
	"github.com/enigmampc/SecretNetwork/rumor-go/utils"
)

type (
	RPCSubscription struct {
		ws          *websocket.Conn
		endpoint string
		id          int
		initialized bool
		onWsError   OnWsError
		outputChannel chan types.Block // use a single channel across reconnections
	}
	Request struct {
		JSONRPC string                    `json:"jsonrpc"`
		Method  string                    `json:"method"`
		ID      int                       `json:"id"`
		Params  SubscriptionJsonRpcParams `json:"params"`
	}
	SubscriptionJsonRpcParams struct {
		Query string `json:"query"`
	}
	OnWsError func(error)
)

var (
	outputChannel = make(chan types.Block)
)

func NewRpcSubscription(
	endpoint string,
	onWsError func(err error),
) (*RPCSubscription, error) {
	log.Print("Opening websocket...")
	ws, _, err := websocket.DefaultDialer.Dial(endpoint, nil)

	// panic as this should not fail
	if err != nil {
		return nil, err
	}

	return &RPCSubscription{
		ws:          ws,
		endpoint:	endpoint,
		onWsError:   onWsError,
		id:          0,
		initialized: false,
		outputChannel: outputChannel,
	}, nil
}

func (c *RPCSubscription) Close() error {
	return c.ws.Close()
}

// Subscribe starts listening to tendermint RPC.
func (c *RPCSubscription) Subscribe(reconnect bool) chan types.Block {
	var request = &Request{
		JSONRPC: "2.0",
		Method:  "subscribe",
		ID:      c.id,
		Params: SubscriptionJsonRpcParams{
			Query: "tm.event = 'NewBlock'",
		},
	}

	log.Print("Subscribing to tendermint rpc...")

	// should not fail here
	if err := c.ws.WriteJSON(request); err != nil {
		panic(err)
	}

	// handle initial message
	// by setting c.initialized to true, we prevent message mishandling
	if c.handleInitialHandhake() != nil {
		c.initialized = true
	}

	log.Print("Subscription and the first handshake done. Receiving blocks...")

	// run event receiver
	go c.receiveBlockEvents(reconnect)

	return c.outputChannel
}

// tendermint rpc sends the "subscription ok" for the intiail response
// filter that out by only sending through channel when there is
// "data" field present
func (c *RPCSubscription) handleInitialHandhake() error {
	_, _, err := c.ws.ReadMessage()

	if err != nil {
		return err
	}

	return nil
}

// TODO: handle errors here
func (c *RPCSubscription) receiveBlockEvents(autoReconnect bool) {

	if autoReconnect {
		defer c.tryReconnect()
	}

	for {
		_, message, err := c.ws.ReadMessage()

		// if read message failed,
		// scrap the whole ws thing
		if err != nil {
			closeErr := c.Close()
			if closeErr != nil {
				log.Print("websocket close failed, but it seems the underlying websocket is already closed")
			}

			if c.onWsError != nil {
				c.onWsError(err)
				return
			} else {
				panic(err)
			}
		}

		var unmarshalErr error

		// check error
		errorMessage := new(struct {
			Error struct {
				Code int `json:"code"`
				Message string `json:"message"`
				Data string `json:"data"`
			} `json:"error"`
		})

		if unmarshalErr = json.Unmarshal(message, errorMessage); unmarshalErr != nil {
			panic(unmarshalErr)
		}

		// tendermint has sent error message,
		// close ws
		if errorMessage.Error.Code != 0 {
			log.Printf(
				"tendermint RPC error, code=%d, message=%s, data=%s",
				errorMessage.Error.Code,
				errorMessage.Error.Message,
				errorMessage.Error.Data,
			)
			c.Close()
			return
		}

		data := new(struct {
			Result struct {
				Data struct {
					Value struct {
						Block json.RawMessage
					} `json:"value"`
				} `json:"data"`
			} `json:"result"`
		})

		if unmarshalErr := json.Unmarshal(message, data); unmarshalErr != nil {
			panic(unmarshalErr)
		}

		block := utils.ConvertBlockHeaderToTMHeader(data.Result.Data.Value.Block)

		// send!
		c.outputChannel <- block
	}
}

func (c *RPCSubscription) tryReconnect() {
	log.Print("reconnecting to tendermint RPC...")
	previousEndpoint := c.endpoint
	previousWsErrorHandler := c.onWsError

	go func() {
		timer := time.NewTimer(5 * time.Second)
		select {
		case <-timer.C:
			nextRpc, connRefused := NewRpcSubscription(previousEndpoint, previousWsErrorHandler)

			if connRefused != nil {
				c.tryReconnect()
				return
			}

			nextRpc.Subscribe(true)
		}
	}()
}