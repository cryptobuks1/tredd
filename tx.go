package tredd

import (
	"context"
	"fmt"
	"io"
	"math/big"
	"strings"
	"time"

	"github.com/bobg/merkle"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/pkg/errors"
)

// ProposePayment publishes a new instance of the Tredd contract instantiated with the given parameters.
func ProposePayment(
	ctx context.Context,
	client *ethclient.Client, // see ethclient.Dial
	buyer *bind.TransactOpts, // see bind.NewTransactor
	seller common.Address,
	amount int64,
	tokenType []byte, // TODO: how to specify the token type?
	collateral int64,
	clearRoot, cipherRoot [32]byte,
	revealDeadline, refundDeadline time.Time,
) (*types.Receipt, error) {
	parsed, err := abi.JSON(strings.NewReader(TreddABI))
	if err != nil {
		return nil, errors.Wrap(err, "parsing contract JSON to ABI")
	}

	_, tx, _, err := bind.DeployContract(buyer, parsed, common.FromHex(TreddBin), client)
	if err != nil {
		return nil, errors.Wrap(err, "deploying contract")
	}

	// Wait for tx to be mined on-chain.
	receipt, err := bind.WaitMined(ctx, client, tx)
	if err != nil {
		return nil, errors.Wrap(err, "awaiting contract-deployment receipt")
	}

	// TODO: store contractAddr

	return receipt, nil
}

// RevealKey updates a Tredd contract on-chain by adding the decryption key.
// TODO: Must also supply collateral.
func RevealKey(
	ctx context.Context,
	client *ethclient.Client, // see ethclient.Dial
	seller *bind.TransactOpts, // see bind.NewTransactor
	contractAddr common.Address,
	key [32]byte,
	wantClearRoot, wantCipherRoot [32]byte,
	wantRevealDeadline, wantRefundDeadline time.Time,
) (*types.Receipt, error) {
	// TODO: read values from the on-chain contract, verify they match the "want" parameters
	con, err := NewTredd(contractAddr, client)
	if err != nil {
		return nil, errors.Wrap(err, "instantiating deployed contract")
	}
	tx, err := con.Reveal(seller, key)
	if err != nil {
		return nil, errors.Wrap(err, "invoking ClaimPayment")
	}
	return bind.WaitMined(ctx, client, tx)
}

// ClaimPayment constructs a seller-claims-payment transaction,
// rehydrating and invoking a Tredd contract from the utxo state (identified by the information in r).
func ClaimPayment(
	ctx context.Context,
	client *ethclient.Client,
	seller *bind.TransactOpts,
	contractAddr common.Address,
) (*types.Receipt, error) {
	con, err := NewTredd(contractAddr, client)
	if err != nil {
		return nil, errors.Wrap(err, "instantiating deployed contract")
	}
	tx, err := con.ClaimPayment(seller)
	if err != nil {
		return nil, errors.Wrap(err, "invoking ClaimPayment")
	}
	return bind.WaitMined(ctx, client, tx)
}

// ClaimRefund constructs a buyer-claims-refund transaction,
// rehydrating a Tredd contract from the utxo state (identified by the information in r)
// and calling it with the necessary proofs and other information.
func ClaimRefund(
	ctx context.Context,
	client *ethclient.Client,
	buyer *bind.TransactOpts,
	contractAddr common.Address,
	index int64,
	cipherChunk []byte,
	clearHash [32]byte,
	cipherProof, clearProof []byte, // TODO: determine the right representation for merkle proofs in Solidity
) (*types.Receipt, error) {
	con, err := NewTredd(contractAddr, client)
	if err != nil {
		return nil, errors.Wrap(err, "instantiating deployed contract")
	}

	bigIndex := big.NewInt(index)

	tx, err := con.Refund(buyer, bigIndex, cipherChunk, clearHash, cipherProof, clearProof)
	if err != nil {
		return nil, errors.Wrap(err, "invoking Refund")
	}
	return bind.WaitMined(ctx, client, tx)
}

func renderProof(w io.Writer, proof merkle.Proof) {
	fmt.Fprint(w, "{")
	for i := len(proof) - 1; i >= 0; i-- {
		if i < len(proof)-1 {
			fmt.Fprint(w, ", ")
		}
		var isLeft int64
		if proof[i].Left {
			isLeft = 1
		}
		fmt.Fprintf(w, "x'%x', %d", proof[i].H, isLeft)
	}
	fmt.Fprintln(w, "}")
}
