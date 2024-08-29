package client

import (
	govclient "github.com/cosmos/cosmos-sdk/x/gov/client"

	"github.com/scrtlabs/SecretNetwork/x/compliance/client/cli"
)

var (
	VerifyIssuerProposalHandler = govclient.NewProposalHandler(cli.CmdVerifyIssuerProposal)
)
