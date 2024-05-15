package v1_5

import (
	"context"
	"fmt"
	"github.com/KYVENetwork/chain/app/upgrades/v1_5/v1_4_types/bundles"
	"github.com/KYVENetwork/chain/app/upgrades/v1_5/v1_4_types/funders"
	fundersKeeper "github.com/KYVENetwork/chain/x/funders/keeper"
	"github.com/KYVENetwork/chain/x/funders/types"
	fundersTypes "github.com/KYVENetwork/chain/x/funders/types"
	globalTypes "github.com/KYVENetwork/chain/x/global/types"

	"cosmossdk.io/math"

	storetypes "cosmossdk.io/store/types"
	upgradetypes "cosmossdk.io/x/upgrade/types"

	"github.com/KYVENetwork/chain/x/bundles/keeper"
	bundlestypes "github.com/KYVENetwork/chain/x/bundles/types"
	poolkeeper "github.com/KYVENetwork/chain/x/pool/keeper"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
)

const (
	UpgradeName = "v1.5.0"
)

func CreateUpgradeHandler(mm *module.Manager, configurator module.Configurator, cdc codec.Codec, storeKeys []storetypes.StoreKey, bundlesKeeper keeper.Keeper, poolKeeper *poolkeeper.Keeper, fundersKeeper fundersKeeper.Keeper) upgradetypes.UpgradeHandler {
	return func(ctx context.Context, plan upgradetypes.Plan, fromVM module.VersionMap) (module.VersionMap, error) {
		sdkCtx := sdk.UnwrapSDKContext(ctx)
		logger := sdkCtx.Logger().With("upgrade", UpgradeName)
		logger.Info(fmt.Sprintf("performing upgrade %v", UpgradeName))

		if err := migrateStorageCosts(sdkCtx, cdc, storeKeys, bundlesKeeper); err != nil {
			return nil, err
		}

		// TODO: migrate gov params

		// migrate fundings
		if storeKey, err := getStoreKey(storeKeys, fundersTypes.StoreKey); err == nil {
			migrateFundersModule(sdkCtx, cdc, storeKey, fundersKeeper)
		} else {
			return nil, err
		}

		// TODO: migrate delegation outstanding rewards

		// TODO: migrate network fee and whitelist weights

		return mm.RunMigrations(ctx, configurator, fromVM)
	}
}

func getStoreKey(storeKeys []storetypes.StoreKey, storeName string) (storetypes.StoreKey, error) {
	for _, k := range storeKeys {
		if k.Name() == storeName {
			return k, nil
		}
	}

	return nil, fmt.Errorf("store key not found: %s", storeName)
}

func migrateFundersModule(sdkCtx sdk.Context, cdc codec.Codec, storeKey storetypes.StoreKey, fundersKeeper fundersKeeper.Keeper) {
	// migrate params
	// TODO: define final prices and initial whitelisted coins
	oldParams := funders.GetParams(sdkCtx, storeKey, cdc)
	fundersKeeper.SetParams(sdkCtx, fundersTypes.Params{
		CoinWhitelist: []*fundersTypes.WhitelistCoinEntry{
			{
				CoinDenom:                 globalTypes.Denom,
				MinFundingAmount:          oldParams.MinFundingAmount,
				MinFundingAmountPerBundle: oldParams.MinFundingAmountPerBundle,
				CoinWeight:                math.LegacyMustNewDecFromStr("0.06"),
			},
		},
		MinFundingMultiple: oldParams.MinFundingMultiple,
	})

	// migrate fundings
	oldFundings := funders.GetAllFundings(sdkCtx, storeKey, cdc)
	for _, funding := range oldFundings {
		fundersKeeper.SetFunding(sdkCtx, &types.Funding{
			FunderAddress:    funding.FunderAddress,
			PoolId:           funding.PoolId,
			Amounts:          sdk.NewCoins(sdk.NewInt64Coin(globalTypes.Denom, int64(funding.Amount))),
			AmountsPerBundle: sdk.NewCoins(sdk.NewInt64Coin(globalTypes.Denom, int64(funding.AmountPerBundle))),
			TotalFunded:      sdk.NewCoins(sdk.NewInt64Coin(globalTypes.Denom, int64(funding.TotalFunded))),
		})
	}
}

func migrateStorageCosts(sdkCtx sdk.Context, cdc codec.Codec, storeKeys []storetypes.StoreKey, bundlesKeeper keeper.Keeper) error {
	var bundlesStoreKey storetypes.StoreKey
	for _, k := range storeKeys {
		if k.Name() == "bundles" {
			bundlesStoreKey = k
			break
		}
	}
	if bundlesStoreKey == nil {
		return fmt.Errorf("store key not found: bundles")
	}

	// Copy storage cost from old params to new params
	// The storage cost of all storage providers will be the same after this migration
	oldParams := bundles.GetParams(sdkCtx, bundlesStoreKey, cdc)
	newParams := bundlestypes.Params{
		UploadTimeout: oldParams.UploadTimeout,
		StorageCosts: []bundlestypes.StorageCost{
			// TODO: define value for storage provider id 1 and 2
			{StorageProviderId: 1, Cost: math.LegacyMustNewDecFromStr("0.00")},
			{StorageProviderId: 2, Cost: math.LegacyMustNewDecFromStr("0.00")},
		},
		NetworkFee: oldParams.NetworkFee,
		MaxPoints:  oldParams.MaxPoints,
	}

	bundlesKeeper.SetParams(sdkCtx, newParams)
	return nil
}
