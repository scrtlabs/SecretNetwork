package keeper

import (
	"fmt"
	"slices"

	"cosmossdk.io/errors"
	"cosmossdk.io/store/prefix"
	storetypes "cosmossdk.io/store/types"
	"github.com/cometbft/cometbft/libs/log"
	sdk "github.com/cosmos/cosmos-sdk/types"
	paramtypes "github.com/cosmos/cosmos-sdk/x/params/types"
	"github.com/cosmos/gogoproto/proto"
	"github.com/ethereum/go-ethereum/crypto"

	"github.com/scrtlabs/SecretNetwork/x/compliance/types"
)

type (
	Keeper struct {
		storeKey   storetypes.StoreKey
		memKey     storetypes.StoreKey
		paramstore paramtypes.Subspace
	}
)

func NewKeeper(
	storeKey,
	memKey storetypes.StoreKey,
	ps paramtypes.Subspace,
) *Keeper {
	if !ps.HasKeyTable() {
		ps = ps.WithKeyTable(types.ParamKeyTable())
	}

	return &Keeper{
		storeKey:   storeKey,
		memKey:     memKey,
		paramstore: ps,
	}
}

func (k Keeper) Logger(ctx sdk.Context) log.Logger {
	return ctx.Logger().With("module", fmt.Sprintf("x/%s", types.ModuleName))
}

// SetIssuerDetails sets details for provided issuer address
func (k Keeper) SetIssuerDetails(ctx sdk.Context, issuerAddress sdk.AccAddress, details *types.IssuerDetails) error {
	store := prefix.NewStore(ctx.KVStore(k.storeKey), types.KeyPrefixIssuerDetails)

	detailsBytes, err := details.Marshal()
	if err != nil {
		return err
	}

	store.Set(issuerAddress.Bytes(), detailsBytes)

	return nil
}

// RemoveIssuer removes provided issuer
func (k Keeper) RemoveIssuer(ctx sdk.Context, issuerAddress sdk.AccAddress) {
	store := prefix.NewStore(ctx.KVStore(k.storeKey), types.KeyPrefixIssuerDetails)
	store.Delete(issuerAddress.Bytes())
	// NOTE, all the verification data verified by removed issuer must be deleted from store
	// But for now, let's keep those verifications.
	// They will be filtered out at the time when call `GetAddressDetails` or `GetVerificationDetails`

	// Remove address details for issuer
	k.RemoveAddressDetails(ctx, issuerAddress)
}

// GetIssuerDetails returns details of provided issuer address
func (k Keeper) GetIssuerDetails(ctx sdk.Context, issuerAddress sdk.AccAddress) (*types.IssuerDetails, error) {
	store := prefix.NewStore(ctx.KVStore(k.storeKey), types.KeyPrefixIssuerDetails)

	detailsBytes := store.Get(issuerAddress.Bytes())
	if detailsBytes == nil {
		return &types.IssuerDetails{}, nil
	}

	var issuerDetails types.IssuerDetails
	if err := proto.Unmarshal(detailsBytes, &issuerDetails); err != nil {
		return nil, err
	}

	return &issuerDetails, nil
}

// IssuerExists checks if issuer exists by checking operator address
func (k Keeper) IssuerExists(ctx sdk.Context, issuerAddress sdk.AccAddress) (bool, error) {
	res, err := k.GetIssuerDetails(ctx, issuerAddress)
	if err != nil {
		return false, err
	}
	return len(res.Name) > 0, nil
}

// GetAddressDetails returns address details
func (k Keeper) GetAddressDetails(ctx sdk.Context, address sdk.AccAddress) (*types.AddressDetails, error) {
	store := prefix.NewStore(ctx.KVStore(k.storeKey), types.KeyPrefixAddressDetails)

	addressDetailsBytes := store.Get(address.Bytes())
	if addressDetailsBytes == nil {
		return &types.AddressDetails{}, nil
	}

	var addressDetails types.AddressDetails
	if err := proto.Unmarshal(addressDetailsBytes, &addressDetails); err != nil {
		return nil, err
	}

	// Filter verification details by issuer's existance
	var newVerifications []*types.Verification
	for _, verification := range addressDetails.Verifications {
		issuerAddress, err := sdk.AccAddressFromBech32(verification.IssuerAddress)
		if err != nil {
			return nil, err
		}
		exists, err := k.IssuerExists(ctx, issuerAddress)
		if err != nil {
			return nil, err
		}
		if exists {
			newVerifications = append(newVerifications, verification)
		}
	}
	addressDetails.Verifications = newVerifications

	return &addressDetails, nil
}

// SetAddressDetails writes address details to the storage
func (k Keeper) SetAddressDetails(ctx sdk.Context, address sdk.AccAddress, details *types.AddressDetails) error {
	store := prefix.NewStore(ctx.KVStore(k.storeKey), types.KeyPrefixAddressDetails)
	detailsBytes, err := details.Marshal()
	if err != nil {
		return err
	}
	store.Set(address.Bytes(), detailsBytes)
	return nil
}

// RemoveAddressDetails deletes address details from store
func (k Keeper) RemoveAddressDetails(ctx sdk.Context, address sdk.AccAddress) {
	store := prefix.NewStore(ctx.KVStore(k.storeKey), types.KeyPrefixAddressDetails)
	store.Delete(address.Bytes())
}

// IsAddressVerified returns information if address is verified.
func (k Keeper) IsAddressVerified(ctx sdk.Context, address sdk.AccAddress) (bool, error) {
	addressDetails, err := k.GetAddressDetails(ctx, address)
	if err != nil {
		return false, err
	}

	// If address is banned, its verification is suspended
	return addressDetails.IsVerified, nil
}

// SetAddressVerificationStatus marks provided address as verified or not verified.
func (k Keeper) SetAddressVerificationStatus(ctx sdk.Context, address sdk.AccAddress, isVerifiedStatus bool) error {
	addressDetails, err := k.GetAddressDetails(ctx, address)
	if err != nil {
		return err
	}

	// Skip if address already has provided status
	if addressDetails.IsVerified == isVerifiedStatus {
		return nil
	}

	addressDetails.IsVerified = isVerifiedStatus
	if err := k.SetAddressDetails(ctx, address, addressDetails); err != nil {
		return err
	}

	return nil
}

// AddVerificationDetails writes details of passed verification by provided address.
func (k Keeper) AddVerificationDetails(ctx sdk.Context, userAddress sdk.AccAddress, verificationType types.VerificationType, details *types.VerificationDetails) ([]byte, error) {
	// Check if issuer is verified and not banned
	issuerAddress, err := sdk.AccAddressFromBech32(details.IssuerAddress)
	if err != nil {
		return nil, err
	}

	isAddressVerified, err := k.IsAddressVerified(ctx, issuerAddress)
	if err != nil {
		return nil, err
	}

	if !isAddressVerified {
		return nil, errors.Wrap(types.ErrInvalidIssuer, "issuer not verified")
	}

	if verificationType <= types.VerificationType_VT_UNSPECIFIED || verificationType > types.VerificationType_VT_CREDIT_SCORE {
		return nil, errors.Wrap(types.ErrInvalidParam, "invalid verification type")
	}
	details.Type = verificationType
	if details.IssuanceTimestamp < 1 || (details.ExpirationTimestamp > 0 && details.IssuanceTimestamp >= details.ExpirationTimestamp) {
		return nil, errors.Wrap(types.ErrInvalidParam, "invalid issuance timestamp")
	}
	if len(details.OriginalData) < 1 {
		return nil, errors.Wrap(types.ErrInvalidParam, "empty proof data")
	}

	detailsBytes, err := details.Marshal()
	if err != nil {
		return nil, err
	}

	// Check if there is no such verification details in storage yet
	verificationDetailsID := crypto.Keccak256(userAddress.Bytes(), verificationType.ToBytes(), detailsBytes)
	verificationDetailsStore := prefix.NewStore(ctx.KVStore(k.storeKey), types.KeyPrefixVerificationDetails)

	if verificationDetailsStore.Has(verificationDetailsID) {
		return nil, errors.Wrap(types.ErrInvalidParam, "provided verification details already in storage")
	}

	// If there is no such verification details associated with provided address, write them to the table
	verificationDetailsStore.Set(verificationDetailsID, detailsBytes)

	// Associate provided verification details with user address
	verification := &types.Verification{
		Type:           verificationType,
		VerificationId: verificationDetailsID,
		IssuerAddress:  issuerAddress.String(),
	}
	userAddressDetails, err := k.GetAddressDetails(ctx, userAddress)
	if err != nil {
		return nil, err
	}

	if slices.Contains(userAddressDetails.Verifications, verification) {
		return nil, errors.Wrap(types.ErrInvalidParam, "such verification already associated with user address")
	}

	userAddressDetails.Verifications = append(userAddressDetails.Verifications, verification)
	if err := k.SetAddressDetails(ctx, userAddress, userAddressDetails); err != nil {
		return nil, err
	}

	return verificationDetailsID, nil
}

// SetVerificationDetails writes verification details
func (k Keeper) SetVerificationDetails(ctx sdk.Context, verificationDetailsId []byte, details *types.VerificationDetails) error {
	verificationDetailsStore := prefix.NewStore(ctx.KVStore(k.storeKey), types.KeyPrefixVerificationDetails)
	if verificationDetailsStore.Has(verificationDetailsId) {
		return errors.Wrap(types.ErrInvalidParam, "provided verification details already in storage")
	}

	detailsBytes, err := details.Marshal()
	if err != nil {
		return err
	}

	// If there is no such verification details associated with provided address, write them to the table
	verificationDetailsStore.Set(verificationDetailsId, detailsBytes)
	return nil
}

// RemoveVerificationDetails removes verification details for provided ID
func (k Keeper) RemoveVerificationDetails(ctx sdk.Context, verificationDetailsId []byte) {
	if verificationDetailsId == nil {
		return
	}
	verificationDetailsStore := prefix.NewStore(ctx.KVStore(k.storeKey), types.KeyPrefixVerificationDetails)
	verificationDetailsStore.Delete(verificationDetailsId)
}

// GetVerificationDetails returns verification details for provided ID
func (k Keeper) GetVerificationDetails(ctx sdk.Context, verificationDetailsId []byte) (*types.VerificationDetails, error) {
	verificationDetailsStore := prefix.NewStore(ctx.KVStore(k.storeKey), types.KeyPrefixVerificationDetails)
	verificationDetailsBytes := verificationDetailsStore.Get(verificationDetailsId)
	if verificationDetailsBytes == nil {
		return &types.VerificationDetails{}, nil
	}

	var verificationDetails types.VerificationDetails
	if err := proto.Unmarshal(verificationDetailsBytes, &verificationDetails); err != nil {
		return nil, err
	}

	// Check if issuer exists. If removed, delete verification data from store
	issuerAddress, err := sdk.AccAddressFromBech32(verificationDetails.IssuerAddress)
	if err != nil {
		return nil, err
	}
	exists, err := k.IssuerExists(ctx, issuerAddress)
	if err != nil {
		return nil, err
	}
	if !exists {
		return &types.VerificationDetails{}, nil
	}

	return &verificationDetails, nil
}

func (k Keeper) GetVerificationDetailsByIssuer(ctx sdk.Context, userAddress sdk.AccAddress, issuerAddress sdk.AccAddress) ([]*types.Verification, []*types.VerificationDetails, error) {
	addressDetails, err := k.GetAddressDetails(ctx, userAddress)
	if err != nil {
		return nil, nil, err
	}

	var (
		filteredVerifications       []*types.Verification
		filteredVerificationDetails []*types.VerificationDetails
	)
	for _, verification := range addressDetails.Verifications {
		if verification.IssuerAddress != issuerAddress.String() {
			continue
		}
		verificationDetails, err := k.GetVerificationDetails(ctx, verification.VerificationId)
		if err != nil {
			return nil, nil, err
		}
		filteredVerifications = append(filteredVerifications, verification)
		filteredVerificationDetails = append(filteredVerificationDetails, verificationDetails)
	}
	return filteredVerifications, filteredVerificationDetails, nil
}

// HasVerificationOfType checks if user has verifications of specific type (for example, passed KYC) from provided issuers.
// If there is no provided expected issuers, this function will check if user has any verification of appropriate type.
func (k Keeper) HasVerificationOfType(ctx sdk.Context, userAddress sdk.AccAddress, expectedType types.VerificationType, expirationTimestamp uint32, expectedIssuers []sdk.AccAddress) (bool, error) {
	// Obtain user address details
	userAddressDetails, err := k.GetAddressDetails(ctx, userAddress)
	if err != nil {
		return false, err
	}

	// If expiration is 0, it means infinite period
	if expirationTimestamp < 1 {
		expirationTimestamp = ^uint32(0)
	}

	for _, verification := range userAddressDetails.Verifications {
		if verification.Type == expectedType {
			// If not found matched issuer, do not get details to check expiration
			found := false
			for _, expectedIssuer := range expectedIssuers {
				if verification.IssuerAddress == expectedIssuer.String() {
					found = true
					break
				}
			}
			if len(expectedIssuers) > 0 && !found {
				continue
			}

			verificationDetails, err := k.GetVerificationDetails(ctx, verification.VerificationId)
			if err != nil {
				continue
			}
			// Check if verification is valid by given expiration timestamp
			if verificationDetails.ExpirationTimestamp > 0 && expirationTimestamp > verificationDetails.ExpirationTimestamp {
				continue
			}
			return true, nil
		}
	}

	return false, nil
}

func (k Keeper) GetVerificationsOfType(ctx sdk.Context, userAddress sdk.AccAddress, expectedType types.VerificationType, expectedIssuers ...sdk.AccAddress) ([]*types.VerificationDetails, error) {
	// Obtain user address details
	userAddressDetails, err := k.GetAddressDetails(ctx, userAddress)
	if err != nil {
		return nil, err
	}

	// Filter verifications with expected type
	var appropriateTypeVerifications []*types.Verification
	for _, verification := range userAddressDetails.Verifications {
		if verification.Type == expectedType {
			appropriateTypeVerifications = append(appropriateTypeVerifications, verification)
		}
	}

	if len(appropriateTypeVerifications) == 0 {
		return nil, nil
	}

	// Extract verification data
	var verifications []*types.VerificationDetails
	for _, verification := range appropriateTypeVerifications {
		// Filter verifications by expected issuer
		if expectedIssuers != nil && slices.ContainsFunc(expectedIssuers, func(expectedIssuer sdk.AccAddress) bool {
			if expectedIssuer.String() == verification.IssuerAddress {
				return true
			}
			return false
		}) == false {
			continue
		}

		verificationDetails, err := k.GetVerificationDetails(ctx, verification.VerificationId)
		if err != nil {
			return nil, err
		}
		verifications = append(verifications, verificationDetails)
	}

	return verifications, nil
}

// GetOperatorDetails returns the operator details
func (k Keeper) GetOperatorDetails(ctx sdk.Context, operator sdk.AccAddress) (*types.OperatorDetails, error) {
	store := prefix.NewStore(ctx.KVStore(k.storeKey), types.KeyPrefixOperatorDetails)

	detailsBytes := store.Get(operator.Bytes())
	if detailsBytes == nil {
		return &types.OperatorDetails{}, nil
	}

	var operatorDetails types.OperatorDetails
	if err := proto.Unmarshal(detailsBytes, &operatorDetails); err != nil {
		return nil, err
	}

	return &operatorDetails, nil
}

// AddOperator adds initial/regular operator.
// Initial operator can not be removed
func (k Keeper) AddOperator(ctx sdk.Context, operator sdk.AccAddress, operatorType types.OperatorType) error {
	store := prefix.NewStore(ctx.KVStore(k.storeKey), types.KeyPrefixOperatorDetails)

	if operatorType <= types.OperatorType_OT_UNSPECIFIED || operatorType > types.OperatorType_OT_REGULAR {
		return errors.Wrap(types.ErrInvalidParam, "invalid operator type")
	}

	details := &types.OperatorDetails{
		Operator:     operator.String(),
		OperatorType: operatorType,
	}
	detailsBytes, err := details.Marshal()
	if err != nil {
		return err
	}

	store.Set(operator.Bytes(), detailsBytes)
	return nil
}

// RemoveRegularOperator removes regular operator
func (k Keeper) RemoveRegularOperator(ctx sdk.Context, operator sdk.AccAddress) error {
	operatorDetails, err := k.GetOperatorDetails(ctx, operator)
	if err != nil || operatorDetails == nil {
		return errors.Wrapf(types.ErrInvalidOperator, "operator not exists")
	}

	if operatorDetails.OperatorType != types.OperatorType_OT_REGULAR {
		return errors.Wrapf(types.ErrNotAuthorized, "operator not a regular type")
	}

	store := prefix.NewStore(ctx.KVStore(k.storeKey), types.KeyPrefixOperatorDetails)
	store.Delete(operator.Bytes())
	return nil
}

// OperatorExists checks if operator exists
func (k Keeper) OperatorExists(ctx sdk.Context, operator sdk.AccAddress) (bool, error) {
	res, err := k.GetOperatorDetails(ctx, operator)
	if err != nil || res == nil {
		return false, err
	}
	return len(res.Operator) > 0, nil
}

func (k Keeper) IterateOperatorDetails(ctx sdk.Context, callback func(address sdk.AccAddress) (continue_ bool)) {
	latestVersionIterator := sdk.KVStorePrefixIterator(ctx.KVStore(k.storeKey), types.KeyPrefixOperatorDetails)
	defer closeIteratorOrPanic(latestVersionIterator)

	for ; latestVersionIterator.Valid(); latestVersionIterator.Next() {
		key := latestVersionIterator.Key()
		address := types.AccAddressFromKey(key)
		if !callback(address) {
			break
		}
	}
}

func (k Keeper) IterateVerificationDetails(ctx sdk.Context, callback func(id []byte) (continue_ bool)) {
	latestVersionIterator := sdk.KVStorePrefixIterator(ctx.KVStore(k.storeKey), types.KeyPrefixVerificationDetails)
	defer closeIteratorOrPanic(latestVersionIterator)

	for ; latestVersionIterator.Valid(); latestVersionIterator.Next() {
		key := latestVersionIterator.Key()
		id := types.VerificationIdFromKey(key)
		if !callback(id) {
			break
		}
	}
}

func (k Keeper) IterateAddressDetails(ctx sdk.Context, callback func(address sdk.AccAddress) (continue_ bool)) {
	latestVersionIterator := sdk.KVStorePrefixIterator(ctx.KVStore(k.storeKey), types.KeyPrefixAddressDetails)
	defer closeIteratorOrPanic(latestVersionIterator)

	for ; latestVersionIterator.Valid(); latestVersionIterator.Next() {
		key := latestVersionIterator.Key()
		address := types.AccAddressFromKey(key)
		if !callback(address) {
			break
		}
	}
}

func (k Keeper) IterateIssuerDetails(ctx sdk.Context, callback func(address sdk.AccAddress) (continue_ bool)) {
	latestVersionIterator := sdk.KVStorePrefixIterator(ctx.KVStore(k.storeKey), types.KeyPrefixIssuerDetails)
	defer closeIteratorOrPanic(latestVersionIterator)

	for ; latestVersionIterator.Valid(); latestVersionIterator.Next() {
		key := latestVersionIterator.Key()
		address := types.AccAddressFromKey(key)
		if !callback(address) {
			break
		}
	}
}

func (k Keeper) ExportOperators(ctx sdk.Context) ([]*types.OperatorDetails, error) {
	var (
		allDetails []*types.OperatorDetails
		details    *types.OperatorDetails
		err        error
	)

	k.IterateOperatorDetails(ctx, func(address sdk.AccAddress) (continue_ bool) {
		details, err = k.GetOperatorDetails(ctx, address)
		if err != nil {
			return false
		}
		allDetails = append(allDetails, details)
		return true
	})
	if err != nil {
		return nil, err
	}

	return allDetails, nil
}

func (k Keeper) ExportVerificationDetails(ctx sdk.Context) ([]*types.GenesisVerificationDetails, error) {
	var (
		allVerificationDetails []*types.GenesisVerificationDetails
		details                *types.VerificationDetails
		err                    error
	)

	k.IterateVerificationDetails(ctx, func(id []byte) bool {
		details, err = k.GetVerificationDetails(ctx, id)
		if err != nil {
			return false
		}
		allVerificationDetails = append(allVerificationDetails, &types.GenesisVerificationDetails{Id: id, Details: details})
		return true
	})
	if err != nil {
		return nil, err
	}

	return allVerificationDetails, nil
}

func (k Keeper) ExportAddressDetails(ctx sdk.Context) ([]*types.GenesisAddressDetails, error) {
	var (
		allAddressDetails []*types.GenesisAddressDetails
		details           *types.AddressDetails
		err               error
	)

	k.IterateAddressDetails(ctx, func(address sdk.AccAddress) bool {
		details, err = k.GetAddressDetails(ctx, address)
		if err != nil {
			return false
		}
		allAddressDetails = append(allAddressDetails, &types.GenesisAddressDetails{Address: address.String(), Details: details})
		return true
	})
	if err != nil {
		return nil, err
	}

	return allAddressDetails, nil
}

func (k Keeper) ExportIssuerDetails(ctx sdk.Context) ([]*types.GenesisIssuerDetails, error) {
	var (
		issuerDetails []*types.GenesisIssuerDetails
		details       *types.IssuerDetails
		err           error
	)

	k.IterateIssuerDetails(ctx, func(address sdk.AccAddress) bool {
		details, err = k.GetIssuerDetails(ctx, address)
		if err != nil {
			return false
		}
		issuerDetails = append(issuerDetails, &types.GenesisIssuerDetails{
			Address: address.String(),
			Details: details,
		})
		return true
	})
	if err != nil {
		return nil, err
	}

	return issuerDetails, nil
}

func closeIteratorOrPanic(iterator sdk.Iterator) {
	err := iterator.Close()
	if err != nil {
		panic(err.Error())
	}
}

// TODO: Create fn to obtain all verified issuers with their aliases
