package keeper

import (
	"errors"
	// "math/big"

	// "github.com/SigmaGmbH/librustgo"
	sdk "github.com/cosmos/cosmos-sdk/types"
	// "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	// "google.golang.org/protobuf/proto"
	// compliancetypes "github.com/scrtlabs/SecretNetwork/x/compliance/types"
)

// TODO: connect our version of librustgo

// Connector allows our VM interact with existing Cosmos application.
// It is passed by pointer into SGX to make it accessible for our VM.
type Connector struct {
	// GetHashFn returns the hash corresponding to n
	GetHashFn vm.GetHashFunc
	// Keeper used to store and obtain state
	EVMKeeper *Keeper
	// Context used to make Keeper calls available
	Context sdk.Context
}

func (q Connector) Query(req []byte) ([]byte, error) {
	// Decode protobuf
	/*
		decodedRequest := &librustgo.CosmosRequest{}
		if err := proto.Unmarshal(req, decodedRequest); err != nil {
			return nil, err
		}

		switch request := decodedRequest.Req.(type) {
		// Handle request for account data such as balance and nonce
		case *librustgo.CosmosRequest_GetAccount:
			return q.GetAccount(request)
		// Handles request for updating account data
		case *librustgo.CosmosRequest_InsertAccount:
			return q.InsertAccount(request)
		// Handles request if such account exists
		case *librustgo.CosmosRequest_ContainsKey:
			return q.ContainsKey(request)
		// Handles contract code request
		case *librustgo.CosmosRequest_AccountCode:
			return q.GetAccountCode(request)
		// Handles storage cell data request
		case *librustgo.CosmosRequest_StorageCell:
			return q.GetStorageCell(request)
		// Handles inserting storage cell
		case *librustgo.CosmosRequest_InsertStorageCell:
			return q.InsertStorageCell(request)
		// Handles updating contract code
		case *librustgo.CosmosRequest_InsertAccountCode:
			return q.InsertAccountCode(request)
		// Handles remove storage cell request
		case *librustgo.CosmosRequest_RemoveStorageCell:
			return q.RemoveStorageCell(request)
		// Handles removing account storage, account record, etc.
		case *librustgo.CosmosRequest_Remove:
			return q.Remove(request)
		// Returns block hash
		case *librustgo.CosmosRequest_BlockHash:
			return q.BlockHash(request)
		case *librustgo.CosmosRequest_AddVerificationDetails:
			return q.AddVerificationDetails(request)
		case *librustgo.CosmosRequest_HasVerification:
			return q.HasVerification(request)
		case *librustgo.CosmosRequest_GetVerificationData:
			return q.GetVerificationData(request)
		}
	*/

	return nil, errors.New("wrong query received")
}

// GetAccount handles incoming protobuf-encoded request for account data such as balance and nonce.
// Returns data in protobuf-encoded format
func (q Connector) GetAccount( /*req *librustgo.CosmosRequest_GetAccount*/ ) ([]byte, error) {
	/*
		//println("Connector::Query GetAccount invoked")
		ethAddress := common.BytesToAddress(req.GetAccount.Address)
		account := q.EVMKeeper.GetAccountOrEmpty(q.Context, ethAddress)

		return proto.Marshal(&librustgo.QueryGetAccountResponse{
			Balance: account.Balance.Bytes(),
			Nonce:   account.Nonce,
		})
	*/
	return nil, errors.New("secret: not implemented")
}

// ContainsKey handles incoming protobuf-encoded request to check whether specified address exists
func (q Connector) ContainsKey( /* req *librustgo.CosmosRequest_ContainsKey */ ) ([]byte, error) {
	/* //println("Connector::Query ContainsKey invoked")
	ethAddress := common.BytesToAddress(req.ContainsKey.Key)
	account := q.EVMKeeper.GetAccountWithoutBalance(q.Context, ethAddress)
	return proto.Marshal(&librustgo.QueryContainsKeyResponse{Contains: account != nil}) */
	return nil, errors.New("secret: not implemented")
}

// InsertAccountCode handles incoming protobuf-encoded request for adding or modifying existing account code
// It will insert account code only if account exists, otherwise it returns an error
func (q Connector) InsertAccountCode( /* req *librustgo.CosmosRequest_InsertAccountCode */ ) ([]byte, error) {
	/* //println("Connector::Query InsertAccountCode invoked")
	ethAddress := common.BytesToAddress(req.InsertAccountCode.Address)
	if err := q.EVMKeeper.SetAccountCode(q.Context, ethAddress, req.InsertAccountCode.Code); err != nil {
		return nil, err
	}

	return proto.Marshal(&librustgo.QueryInsertAccountCodeResponse{}) */
	return nil, errors.New("secret: not implemented")
}

// RemoveStorageCell handles incoming protobuf-encoded request for removing contract storage cell for given key (index)
func (q Connector) RemoveStorageCell( /* req *librustgo.CosmosRequest_RemoveStorageCell */ ) ([]byte, error) {
	/* //println("Connector::Query RemoveStorageCell invoked")
	address := common.BytesToAddress(req.RemoveStorageCell.Address)
	index := common.BytesToHash(req.RemoveStorageCell.Index)

	q.EVMKeeper.SetState(q.Context, address, index, common.Hash{}.Bytes())

	return proto.Marshal(&librustgo.QueryRemoveStorageCellResponse{}) */
	return nil, errors.New("secret: not implemented")
}

// Remove handles incoming protobuf-encoded request for removing smart contract (selfdestruct)
func (q Connector) Remove( /* req *librustgo.CosmosRequest_Remove */ ) ([]byte, error) {
	/* //println("Connector::Query Remove invoked")
	ethAddress := common.BytesToAddress(req.Remove.Address)
	if err := q.EVMKeeper.DeleteAccount(q.Context, ethAddress); err != nil {
		return nil, err
	}

	return proto.Marshal(&librustgo.QueryRemoveResponse{}) */
	return nil, errors.New("secret: not implemented")
}

// BlockHash handles incoming protobuf-encoded request for getting block hash
func (q Connector) BlockHash( /* req *librustgo.CosmosRequest_BlockHash */ ) ([]byte, error) {
	/* //println("Connector::Query BlockHash invoked")

	blockNumber := &big.Int{}
	blockNumber.SetBytes(req.BlockHash.Number)
	blockHash := q.GetHashFn(blockNumber.Uint64())

	return proto.Marshal(&librustgo.QueryBlockHashResponse{Hash: blockHash.Bytes()}) */
	return nil, errors.New("secret: not implemented")
}

// InsertStorageCell handles incoming protobuf-encoded request for updating state of storage cell
func (q Connector) InsertStorageCell( /* req *librustgo.CosmosRequest_InsertStorageCell */ ) ([]byte, error) {
	/* ethAddress := common.BytesToAddress(req.InsertStorageCell.Address)
	index := common.BytesToHash(req.InsertStorageCell.Index)

	q.EVMKeeper.SetState(q.Context, ethAddress, index, req.InsertStorageCell.Value)
	return proto.Marshal(&librustgo.QueryInsertStorageCellResponse{}) */
	return nil, errors.New("secret: not implemented")
}

// GetStorageCell handles incoming protobuf-encoded request of storage cell value
func (q Connector) GetStorageCell( /* req *librustgo.CosmosRequest_StorageCell */ ) ([]byte, error) {
	/* //println("Connector::Query Request value of storage cell")
	ethAddress := common.BytesToAddress(req.StorageCell.Address)
	index := common.BytesToHash(req.StorageCell.Index)
	value := q.EVMKeeper.GetState(q.Context, ethAddress, index)

	return proto.Marshal(&librustgo.QueryGetAccountStorageCellResponse{Value: value}) */
	return nil, errors.New("secret: not implemented")
}

// GetAccountCode handles incoming protobuf-encoded request and returns bytecode associated
// with given account. If account does not exist, it returns empty response
func (q Connector) GetAccountCode( /* req *librustgo.CosmosRequest_AccountCode */ ) ([]byte, error) {
	/* //println("Connector::Query Request account code")
	ethAddress := common.BytesToAddress(req.AccountCode.Address)
	account := q.EVMKeeper.GetAccountWithoutBalance(q.Context, ethAddress)
	if account == nil {
		return proto.Marshal(&librustgo.QueryGetAccountCodeResponse{
			Code: nil,
		})
	}

	code := q.EVMKeeper.GetCode(q.Context, common.BytesToHash(account.CodeHash))
	return proto.Marshal(&librustgo.QueryGetAccountCodeResponse{
		Code: code,
	}) */
	return nil, errors.New("secret: not implemented")
}

// InsertAccount handles incoming protobuf-encoded request for inserting new account data
// such as balance and nonce. If there is deployed contract behind given address, its bytecode
// or code hash won't be changed
func (q Connector) InsertAccount( /* req *librustgo.CosmosRequest_InsertAccount */ ) ([]byte, error) {
	/* //println("Connector::Query Request to insert account code")
	ethAddress := common.BytesToAddress(req.InsertAccount.Address)

	balance := &big.Int{}
	balance.SetBytes(req.InsertAccount.Balance)
	nonce := req.InsertAccount.Nonce

	account := q.EVMKeeper.GetAccountOrEmpty(q.Context, ethAddress)
	if err := q.EVMKeeper.SetBalance(q.Context, ethAddress, balance); err != nil {
		return nil, err
	}

	account.Balance = balance
	account.Nonce = nonce
	if err := q.EVMKeeper.SetAccount(q.Context, ethAddress, account); err != nil {
		return nil, err
	}

	return proto.Marshal(&librustgo.QueryInsertAccountResponse{}) */
	return nil, errors.New("secret: not implemented")
}

// TODO: REMOVE
// AddVerificationDetails writes provided verification details to x/compliance module
/*
func (q Connector) AddVerificationDetails(req *librustgo.CosmosRequest_AddVerificationDetails) ([]byte, error) {
	userAddress := sdk.AccAddress(req.AddVerificationDetails.UserAddress)
	issuerAddress := sdk.AccAddress(req.AddVerificationDetails.IssuerAddress).String()
	verificationType := compliancetypes.VerificationType(req.AddVerificationDetails.VerificationType)

	// Addresses in keeper are Cosmos Addresses
	verificationDetails := &compliancetypes.VerificationDetails{
		IssuerAddress:        issuerAddress,
		OriginChain:          req.AddVerificationDetails.OriginChain,
		IssuanceTimestamp:    req.AddVerificationDetails.IssuanceTimestamp,
		ExpirationTimestamp:  req.AddVerificationDetails.ExpirationTimestamp,
		OriginalData:         req.AddVerificationDetails.ProofData,
		Schema:               string(req.AddVerificationDetails.Schema),
		IssuerVerificationId: string(req.AddVerificationDetails.IssuerVerificationId),
		Version:              req.AddVerificationDetails.Version,
	}

	verificationID, err := q.EVMKeeper.ComplianceKeeper.AddVerificationDetails(q.Context, userAddress, verificationType, verificationDetails)
	if err != nil {
		return nil, err
	}

	return proto.Marshal(&librustgo.QueryAddVerificationDetailsResponse{
		VerificationId: verificationID,
	})
}

// HasVerification returns if user has verification of provided type from x/compliance module
func (q Connector) HasVerification(req *librustgo.CosmosRequest_HasVerification) ([]byte, error) {
	userAddress := sdk.AccAddress(req.HasVerification.UserAddress)
	verificationType := compliancetypes.VerificationType(req.HasVerification.VerificationType)
	expirationTimestamp := req.HasVerification.ExpirationTimestamp

	var allowedIssuers []sdk.AccAddress
	for _, issuer := range req.HasVerification.AllowedIssuers {
		allowedIssuers = append(allowedIssuers, sdk.AccAddress(issuer))
	}

	hasVerification, err := q.EVMKeeper.ComplianceKeeper.HasVerificationOfType(q.Context, userAddress, verificationType, expirationTimestamp, allowedIssuers)
	if err != nil {
		return nil, err
	}

	return proto.Marshal(&librustgo.QueryHasVerificationResponse{
		HasVerification: hasVerification,
	})
}

func (q Connector) GetVerificationData(req *librustgo.CosmosRequest_GetVerificationData) ([]byte, error) {
	userAddress := sdk.AccAddress(req.GetVerificationData.UserAddress)
	issuerAddress := sdk.AccAddress(req.GetVerificationData.IssuerAddress)

	verifications, verificationsDetails, err := q.EVMKeeper.ComplianceKeeper.GetVerificationDetailsByIssuer(q.Context, userAddress, issuerAddress)
	if err != nil {
		return nil, err
	}
	if len(verifications) != len(verificationsDetails) {
		return nil, errors.New("invalid verification details")
	}

	var resData []*librustgo.VerificationDetails
	for i, v := range verifications {
		issuerAccount, err := sdk.AccAddressFromBech32(v.IssuerAddress)
		if err != nil {
			return nil, err
		}
		details := verificationsDetails[i]
		// Addresses from Query requests are Ethereum Addresses
		resData = append(resData, &librustgo.VerificationDetails{
			VerificationType:     uint32(v.Type),
			VerificationID:       v.VerificationId,
			IssuerAddress:        common.Address(issuerAccount.Bytes()).Bytes(),
			OriginChain:          details.OriginChain,
			IssuanceTimestamp:    details.IssuanceTimestamp,
			ExpirationTimestamp:  details.ExpirationTimestamp,
			OriginalData:         details.OriginalData,
			Schema:               details.Schema,
			IssuerVerificationId: details.IssuerVerificationId,
			Version:              details.Version,
		})
	}
	return proto.Marshal(&librustgo.QueryGetVerificationDataResponse{
		Data: resData,
	})
}
*/
