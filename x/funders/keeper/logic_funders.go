package keeper

import (
	"cosmossdk.io/errors"
	"github.com/KYVENetwork/chain/util"
	"github.com/KYVENetwork/chain/x/funders/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	errorsTypes "github.com/cosmos/cosmos-sdk/types/errors"
)

func (k Keeper) CreateFundingState(ctx sdk.Context, poolId uint64) {
	fundingState := types.FundingState{
		PoolId:                poolId,
		ActiveFunderAddresses: []string{},
		TotalAmount:           0,
	}
	k.SetFundingState(ctx, &fundingState)
}

func (k Keeper) DefundFundingState(ctx sdk.Context, poolId uint64) (err error) {
	// Get funding state for pool
	fundingState, found := k.GetFundingState(ctx, poolId)
	if !found {
		return errors.Wrapf(errorsTypes.ErrNotFound, types.ErrFundingStateDoesNotExist.Error(), poolId)
	}

	// If there are no active fundings we immediately return
	activeFundings := k.GetActiveFundings(ctx, fundingState)
	if len(activeFundings) == 0 {
		return nil
	}

	// Transfer tokens back to funders
	for _, funding := range activeFundings {
		if err := util.TransferFromModuleToAddress(k.bankKeeper, ctx, types.ModuleName, funding.FunderAddress, funding.Amount); err != nil {
			return err
		}
		k.SetFunding(ctx, &funding)
	}

	// Save funding state
	fundingState.TotalAmount = 0
	fundingState.ActiveFunderAddresses = []string{}
	k.SetFundingState(ctx, &fundingState)
	return nil
}

// ChargeFundersOfPool equally splits the amount between all funders and removes
// the appropriate amount from each funder.
// All funders who can't afford the amount, are kicked out.
// Their remaining amount is transferred to the Treasury.
// This method does not transfer any funds. The bundles-module
// is responsible for transferring the rewards out of the module.
// TODO: update text
func (k Keeper) ChargeFundersOfPool(ctx sdk.Context, poolId uint64) (payout uint64, err error) {
	// Get funding state for pool
	fundingState, found := k.GetFundingState(ctx, poolId)
	if !found {
		return 0, errors.Wrapf(errorsTypes.ErrNotFound, types.ErrFundingStateDoesNotExist.Error(), poolId)
	}

	// If there are no active fundings we immediately return
	activeFundings := k.GetActiveFundings(ctx, fundingState)
	if len(activeFundings) == 0 {
		return 0, nil
	}

	// This is the amount every funding will be charged
	payout = 0
	for _, funding := range activeFundings {
		payout += funding.ChargeOneBundle()
		if funding.Amount == 0 {
			fundingState.SetInactive(&funding)
		}
		k.SetFunding(ctx, &funding)
	}

	// Save funding state
	k.SetFundingState(ctx, &fundingState)

	// Emit a pool out of funds event if there are no more active funders
	if len(fundingState.ActiveFunderAddresses) == 0 {
		_ = ctx.EventManager().EmitTypedEvent(&types.EventPoolOutOfFunds{
			PoolId: poolId,
		})
	}
	return payout, nil
}
