package cosmos

import (
	"context"
	"fmt"
	"strconv"

	"github.com/cosmos/cosmos-sdk/client"
	sdk "github.com/cosmos/cosmos-sdk/types"
	grpctypes "github.com/cosmos/cosmos-sdk/types/grpc"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/metamask"
)

var _ client.AccountRetriever = &CosmosProvider{}

// GetAccount queries for an account given an address and a block height. An
// error is returned if the query or decoding fails.
func (cc *CosmosProvider) GetAccount(clientCtx client.Context, addr sdk.AccAddress) (client.Account, error) {
	account, _, err := cc.GetAccountWithHeight(clientCtx, addr)
	return account, err
}


// EnsureExists returns an error if no account exists for the given address else nil.
func (cc *CosmosProvider) EnsureExists(clientCtx client.Context, addr sdk.AccAddress) error {
	_, err := cc.GetAccount(clientCtx, addr)
	return err
}

// GetAccountNumberSequence returns sequence and account number for the given address.
// It returns an error if the account couldn't be retrieved from the state.
func (cc *CosmosProvider) GetAccountNumberSequence(clientCtx client.Context, addr sdk.AccAddress) (uint64, uint64, error) {
	acc, err := cc.GetAccount(clientCtx, addr)
	if err != nil {
		return 0, 0, err
	}
	return acc.GetAccountNumber(), acc.GetSequence(), nil
}
