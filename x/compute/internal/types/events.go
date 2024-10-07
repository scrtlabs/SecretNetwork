package types

const (
	// WasmModuleEventType is stored with any contract TX that returns non empty EventAttributes
	CustomEventType = "wasm"
	// CustomContractEventPrefix contracts can create custom events. To not mix them with other system events they got the `wasm-` prefix.
	CustomContractEventPrefix = "wasm-"

	EventTypeStoreCode             = "store_code"
	EventTypeInstantiate           = "instantiate"
	EventTypeExecute               = "execute"
	EventTypeMigrate               = "migrate"
	EventTypePinCode               = "pin_code"
	EventTypeUnpinCode             = "unpin_code"
	EventTypeSudo                  = "sudo"
	EventTypeReply                 = "reply"
	EventTypeUpdateContractAdmin   = "update_contract_admin"
	EventTypeUpgradeProposalPassed = "upgrade_proposal_passed"
)

// event attributes returned from contract execution
const (
	AttributeReservedPrefix = "_"

	AttributeKeyContractAddr = "contract_address"
	AttributeKeyCodeID       = "code_id"
	AttributeKeySigner       = "signer"
	AttributeKeyNewAdmin     = "new_admin_address"
)
