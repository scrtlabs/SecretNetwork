package types

const (
	EventTypeAddOperator    = "add_operator"
	EventTypeRemoveOperator = "remove_operator"
	EventTypeAddIssuer      = "add_issuer"
	EventTypeUpdateIssuer   = "update_issuer"
	EventTypeRemoveIssuer   = "remove_issuer"
	EventTypeVerifyIssuer   = "verify_issuer"

	AttributeKeyOperator           = "operator"
	AttributeKeyIssuerCreator      = "creator"
	AttributeKeyIssuer             = "issuer"
	AttributeKeyIssuerDetails      = "issuer_details"
	AttributeKeyVerificationStatus = "verification_status"
)
