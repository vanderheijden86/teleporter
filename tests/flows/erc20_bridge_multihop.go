package flows

import (
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"math/big"
	"strings"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/subnet-evm/accounts/abi/bind"
	"github.com/ava-labs/subnet-evm/core/types"
	bridgetoken "github.com/ava-labs/teleporter/abi-bindings/go/CrossChainApplications/examples/ERC20Bridge/BridgeToken"
	erc20bridge "github.com/ava-labs/teleporter/abi-bindings/go/CrossChainApplications/examples/ERC20Bridge/ERC20Bridge"
	teleportermessenger "github.com/ava-labs/teleporter/abi-bindings/go/Teleporter/TeleporterMessenger"
	"github.com/ava-labs/teleporter/tests/interfaces"
	"github.com/ava-labs/teleporter/tests/utils"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
	. "github.com/onsi/gomega"
)

func ERC20BridgeMultihop(network interfaces.Network) {
	CChainInfo := network.GetPrimaryNetworkInfo()
	subnetAInfo, subnetBInfo := utils.GetTwoSubnets(network)
	fundedAddress, fundedKey := network.GetFundedAccountInfo()
	ctx := context.Background()

	// log the CChainInfo
	log.Info("CChainInfo", "NodeURIs", strings.Join(CChainInfo.NodeURIs, ", "))
	log.Info("CChainInfo", "SubnetId", CChainInfo.SubnetID.String())
	log.Info("CChainInfo", "BlockchainId", CChainInfo.BlockchainID.String())
	log.Info("CChainInfo", "EVMChainId", CChainInfo.EVMChainID.String())
	log.Info("CChainInfo", "TeleporterRegistryAddress", CChainInfo.TeleporterRegistryAddress.String())

	log.Info("subnetAInfo", "NodeURIs", strings.Join(subnetAInfo.NodeURIs, ", "))
	log.Info("subnetAInfo", "SubnetId", subnetAInfo.SubnetID.String())
	log.Info("subnetAInfo", "BlockchainId", subnetAInfo.BlockchainID.String())
	log.Info("subnetAInfo", "EVMChainId", subnetAInfo.EVMChainID.String())
	log.Info("subnetAInfo", "TeleporterRegistryAddress", CChainInfo.TeleporterRegistryAddress.String())

	log.Info("subnetBInfo", "NodeURIs", strings.Join(subnetBInfo.NodeURIs, ", "))
	log.Info("subnetBInfo", "SubnetId", subnetBInfo.SubnetID.String())
	log.Info("subnetBInfo", "BlockchainId", subnetBInfo.BlockchainID.String())
	log.Info("subnetBInfo", "EVMChainId", subnetBInfo.EVMChainID.String())
	log.Info("subnetBInfo", "TeleporterRegistryAddress", CChainInfo.TeleporterRegistryAddress.String())

	// Deploy an ERC20 to C Chain
	nativeERC20Address, nativeERC20 := utils.DeployExampleERC20(
		context.Background(),
		fundedKey,
		CChainInfo,
	)

	// Deploy the ERC20 bridge to C Chain
	erc20BridgeAddressCChain, erc20BridgeCChain := utils.DeployERC20Bridge(
		ctx,
		fundedKey,
		fundedAddress,
		CChainInfo,
	)
	// Deploy the ERC20 bridge to subnet A
	erc20BridgeAddressA, erc20BridgeA := utils.DeployERC20Bridge(
		ctx,
		fundedKey,
		fundedAddress,
		subnetAInfo,
	)
	// Deploy the ERC20 bridge to subnet B
	erc20BridgeAddressB, erc20BridgeB := utils.DeployERC20Bridge(
		ctx,
		fundedKey,
		fundedAddress,
		subnetBInfo,
	)

	amount := big.NewInt(0).Mul(big.NewInt(1e18), big.NewInt(10000000000000))
	utils.ERC20Approve(
		ctx,
		nativeERC20,
		erc20BridgeAddressCChain,
		amount,
		CChainInfo,
		fundedKey,
	)

	// Send a transaction on C Chain to add support for the the ERC20 token to the bridge on Subnet A
	receipt, messageID := submitCreateBridgeToken(
		ctx,
		CChainInfo,
		subnetAInfo.BlockchainID,
		erc20BridgeAddressA,
		nativeERC20Address,
		nativeERC20Address,
		big.NewInt(0),
		fundedAddress,
		fundedKey,
		erc20BridgeCChain,
		CChainInfo.TeleporterMessenger,
	)

	// Relay message
	network.RelayMessage(ctx, receipt, CChainInfo, subnetAInfo, true)

	// Check Teleporter message received on the destination
	delivered, err := subnetAInfo.TeleporterMessenger.MessageReceived(
		&bind.CallOpts{},
		messageID,
	)
	Expect(err).Should(BeNil())
	Expect(delivered).Should(BeTrue())

	// Check the bridge token was added on Subnet A
	bridgeTokenSubnetAAddress, err := erc20BridgeA.NativeToWrappedTokens(
		&bind.CallOpts{},
		CChainInfo.BlockchainID,
		erc20BridgeAddressCChain,
		nativeERC20Address,
	)
	// log out bridgeTokenSubnetAAddress
	log.Info("bridgeTokenSubnetA", "bridgeTokenSubnetAAddress", bridgeTokenSubnetAAddress.String())

	Expect(err).Should(BeNil())
	Expect(bridgeTokenSubnetAAddress).ShouldNot(Equal(common.Address{}))
	bridgeTokenA, err := bridgetoken.NewBridgeToken(bridgeTokenSubnetAAddress, subnetAInfo.RPCClient)
	Expect(err).Should(BeNil())

	// Check all the settings of the new bridge token are correct.
	actualNativeChainID, err := bridgeTokenA.NativeBlockchainID(&bind.CallOpts{})
	Expect(err).Should(BeNil())
	Expect(actualNativeChainID[:]).Should(Equal(CChainInfo.BlockchainID[:]))

	actualNativeBridgeAddress, err := bridgeTokenA.NativeBridge(&bind.CallOpts{})
	Expect(err).Should(BeNil())
	Expect(actualNativeBridgeAddress).Should(Equal(erc20BridgeAddressCChain))

	actualNativeAssetAddress, err := bridgeTokenA.NativeAsset(&bind.CallOpts{})
	Expect(err).Should(BeNil())
	Expect(actualNativeAssetAddress).Should(Equal(nativeERC20Address))

	actualName, err := bridgeTokenA.Name(&bind.CallOpts{})
	Expect(err).Should(BeNil())
	Expect(actualName).Should(Equal("Mock Token"))

	actualSymbol, err := bridgeTokenA.Symbol(&bind.CallOpts{})
	Expect(err).Should(BeNil())
	Expect(actualSymbol).Should(Equal("EXMP"))

	actualDecimals, err := bridgeTokenA.Decimals(&bind.CallOpts{})
	Expect(err).Should(BeNil())
	Expect(actualDecimals).Should(Equal(uint8(18)))

	// Send a transaction on Subnet A to add support for the the ERC20 token to the bridge on Subnet B
	receipt, messageID = submitCreateBridgeToken(
		ctx,
		CChainInfo,
		subnetBInfo.BlockchainID,
		erc20BridgeAddressB,
		nativeERC20Address,
		nativeERC20Address,
		big.NewInt(0),
		fundedAddress,
		fundedKey,
		erc20BridgeCChain,
		CChainInfo.TeleporterMessenger,
	)

	// Relay message
	network.RelayMessage(ctx, receipt, CChainInfo, subnetBInfo, true)

	// Check Teleporter message received on the destination
	delivered, err =
		subnetBInfo.TeleporterMessenger.MessageReceived(
			&bind.CallOpts{},
			messageID,
		)
	Expect(err).Should(BeNil())
	Expect(delivered).Should(BeTrue())

	// Check the bridge token was added on Subnet B
	bridgeTokenSubnetBAddress, err := erc20BridgeB.NativeToWrappedTokens(
		&bind.CallOpts{},
		CChainInfo.BlockchainID,
		erc20BridgeAddressCChain,
		nativeERC20Address,
	)
	Expect(err).Should(BeNil())
	Expect(bridgeTokenSubnetBAddress).ShouldNot(Equal(common.Address{}))
	log.Info("bridgeTokenSubnetB", "bridgeTokenSubnetBAddress", bridgeTokenSubnetBAddress.String())

	bridgeTokenB, err := bridgetoken.NewBridgeToken(bridgeTokenSubnetBAddress, subnetBInfo.RPCClient)
	Expect(err).Should(BeNil())

	// Send a bridge transfer for the newly added token from C Chain to subnet A
	totalAmount := big.NewInt(0).Mul(big.NewInt(1e18), big.NewInt(13))
	primaryFeeAmount := big.NewInt(1e18)
	receipt, messageID = bridgeToken(
		ctx,
		CChainInfo,
		subnetAInfo.BlockchainID,
		erc20BridgeAddressA,
		nativeERC20Address,
		fundedAddress,
		totalAmount,
		primaryFeeAmount,
		big.NewInt(0),
		fundedAddress,
		fundedKey,
		erc20BridgeCChain,
		true,
		CChainInfo.BlockchainID,
		CChainInfo.TeleporterMessenger,
	)

	// Relay message
	deliveryReceipt := network.RelayMessage(ctx, receipt, CChainInfo, subnetAInfo, true)
	receiveEvent, err := utils.GetEventFromLogs(
		deliveryReceipt.Logs,
		subnetAInfo.TeleporterMessenger.ParseReceiveCrossChainMessage)
	Expect(err).Should(BeNil())

	// Check Teleporter message received on the destination
	delivered, err =
		subnetAInfo.TeleporterMessenger.MessageReceived(&bind.CallOpts{}, messageID)
	Expect(err).Should(BeNil())
	Expect(delivered).Should(BeTrue())

	// Check the recipient balance of the new bridge token.
	actualRecipientBalance, err := bridgeTokenA.BalanceOf(&bind.CallOpts{}, fundedAddress)
	Expect(err).Should(BeNil())
	Expect(actualRecipientBalance).Should(Equal(totalAmount.Sub(totalAmount, primaryFeeAmount)))

	// Approve the bridge contract on subnet A to spend the wrapped tokens in the user account.
	approveBridgeToken(
		ctx,
		subnetAInfo,
		bridgeTokenSubnetAAddress,
		bridgeTokenA,
		amount,
		erc20BridgeAddressA,
		fundedAddress,
		fundedKey,
	)

	// Check the initial relayer reward amount on SubnetA.
	currentRewardAmount, err := CChainInfo.TeleporterMessenger.CheckRelayerRewardAmount(
		&bind.CallOpts{},
		receiveEvent.RewardRedeemer,
		nativeERC20Address)
	Expect(err).Should(BeNil())

	// Unwrap bridged tokens back to C Chain, then wrap tokens to final destination on subnet B
	totalAmount = big.NewInt(0).Mul(big.NewInt(1e18), big.NewInt(11))
	secondaryFeeAmount := big.NewInt(1e18)
	receipt, messageID = bridgeToken(
		ctx,
		subnetAInfo,
		subnetBInfo.BlockchainID,
		erc20BridgeAddressB,
		bridgeTokenSubnetAAddress,
		fundedAddress,
		totalAmount,
		primaryFeeAmount,
		secondaryFeeAmount,
		fundedAddress,
		fundedKey,
		erc20BridgeA,
		false,
		CChainInfo.BlockchainID,
		subnetAInfo.TeleporterMessenger,
	)

	// Relay message from SubnetB to SubnetA
	// The receipt of transaction that delivers the message will also have the "second hop"
	// message sent from C Chain to subnet B.
	receipt = network.RelayMessage(ctx, receipt, subnetAInfo, CChainInfo, true)

	// Check Teleporter message received on the destination
	delivered, err =
		CChainInfo.TeleporterMessenger.MessageReceived(
			&bind.CallOpts{},
			messageID,
		)
	Expect(err).Should(BeNil())
	Expect(delivered).Should(BeTrue())

	// Get the sendCrossChainMessage event from SubnetA to SubnetC, which should be present in
	// the receipt of the transaction that delivered the first message from SubnetB to SubnetA.
	event, err := utils.GetEventFromLogs(receipt.Logs,
		CChainInfo.TeleporterMessenger.ParseSendCrossChainMessage)
	Expect(err).Should(BeNil())
	Expect(event.DestinationBlockchainID[:]).Should(Equal(subnetBInfo.BlockchainID[:]))
	messageID = event.MessageID

	// Check the redeemable reward balance of the relayer if the relayer address was set.
	// If this is an external network, skip this check since it depends on the initial state of the receipt
	// queue prior to the test run.
	if !network.IsExternalNetwork() {
		updatedRewardAmount, err :=
			CChainInfo.TeleporterMessenger.CheckRelayerRewardAmount(
				&bind.CallOpts{},
				receiveEvent.RewardRedeemer,
				nativeERC20Address,
			)
		Expect(err).Should(BeNil())
		Expect(updatedRewardAmount).Should(Equal(new(big.Int).Add(currentRewardAmount, primaryFeeAmount)))
	}

	// Relay message from SubnetA to SubnetC
	deliveryReceipt = network.RelayMessage(ctx, receipt, CChainInfo, subnetBInfo, true)
	receiveEvent, err = utils.GetEventFromLogs(
		deliveryReceipt.Logs,
		subnetBInfo.TeleporterMessenger.ParseReceiveCrossChainMessage)
	Expect(err).Should(BeNil())

	// Check Teleporter message received on the destination
	delivered, err =
		subnetBInfo.TeleporterMessenger.MessageReceived(&bind.CallOpts{}, messageID)
	Expect(err).Should(BeNil())
	Expect(delivered).Should(BeTrue())

	actualRecipientBalance, err = bridgeTokenB.BalanceOf(&bind.CallOpts{}, fundedAddress)
	Expect(err).Should(BeNil())
	expectedAmount := totalAmount.Sub(totalAmount, primaryFeeAmount).Sub(totalAmount, secondaryFeeAmount)
	Expect(actualRecipientBalance).Should(Equal(expectedAmount))

	// Approve the bridge contract on Subnet B to spend the bridge tokens from the user account
	approveBridgeToken(
		ctx,
		subnetBInfo,
		bridgeTokenSubnetBAddress,
		bridgeTokenB,
		amount,
		erc20BridgeAddressB,
		fundedAddress,
		fundedKey)

	// Get the current relayer reward amount on SubnetA.
	currentRewardAmount, err = CChainInfo.TeleporterMessenger.CheckRelayerRewardAmount(
		&bind.CallOpts{},
		receiveEvent.RewardRedeemer,
		nativeERC20Address)
	Expect(err).Should(BeNil())

	// Send a transaction to unwrap tokens from Subnet B back to Subnet A
	totalAmount = big.NewInt(0).Mul(big.NewInt(1e18), big.NewInt(8))
	receipt, messageID = bridgeToken(
		ctx,
		subnetBInfo,
		CChainInfo.BlockchainID,
		erc20BridgeAddressCChain,
		bridgeTokenSubnetBAddress,
		fundedAddress,
		totalAmount,
		primaryFeeAmount,
		big.NewInt(0),
		fundedAddress,
		fundedKey,
		erc20BridgeB,
		false,
		CChainInfo.BlockchainID,
		subnetBInfo.TeleporterMessenger,
	)

	// Relay message from SubnetC to SubnetA
	network.RelayMessage(ctx, receipt, subnetBInfo, CChainInfo, true)

	// Check Teleporter message received on the destination
	delivered, err =
		CChainInfo.TeleporterMessenger.MessageReceived(&bind.CallOpts{}, messageID)
	Expect(err).Should(BeNil())
	Expect(delivered).Should(BeTrue())

	// Check the balance of the native token after the unwrap
	actualNativeTokenDefaultAccountBalance, err := nativeERC20.BalanceOf(&bind.CallOpts{}, fundedAddress)
	Expect(err).Should(BeNil())
	expectedAmount = big.NewInt(0).Mul(big.NewInt(1e18), big.NewInt(9999999994))
	Expect(actualNativeTokenDefaultAccountBalance).Should(Equal(expectedAmount))

	// Check the balance of the native token for the relayer, which should have received the fee rewards
	// If this is an external network, skip this check since it depends on the initial state of the receipt
	// queue prior to the test run.
	if !network.IsExternalNetwork() {
		updatedRewardAmount, err :=
			CChainInfo.TeleporterMessenger.CheckRelayerRewardAmount(
				&bind.CallOpts{},
				receiveEvent.RewardRedeemer,
				nativeERC20Address,
			)
		Expect(err).Should(BeNil())
		Expect(updatedRewardAmount).Should(Equal(new(big.Int).Add(currentRewardAmount, secondaryFeeAmount)))
	}
}

func submitCreateBridgeToken(
	ctx context.Context,
	source interfaces.SubnetTestInfo,
	destinationBlockchainID ids.ID,
	destinationBridgeAddress common.Address,
	nativeToken common.Address,
	messageFeeAsset common.Address,
	messageFeeAmount *big.Int,
	fundedAddress common.Address,
	fundedKey *ecdsa.PrivateKey,
	transactor *erc20bridge.ERC20Bridge,
	teleporterMessenger *teleportermessenger.TeleporterMessenger,
) (*types.Receipt, ids.ID) {
	opts, err := bind.NewKeyedTransactorWithChainID(fundedKey, source.EVMChainID)
	Expect(err).Should(BeNil())

	tx, err := transactor.SubmitCreateBridgeToken(
		opts,
		destinationBlockchainID,
		destinationBridgeAddress,
		nativeToken,
		messageFeeAsset,
		messageFeeAmount,
	)
	Expect(err).Should(BeNil())

	// Wait for the transaction to be mined
	receipt := utils.WaitForTransactionSuccess(ctx, source, tx.Hash())

	event, err := utils.GetEventFromLogs(receipt.Logs, teleporterMessenger.ParseSendCrossChainMessage)
	Expect(err).Should(BeNil())
	Expect(event.DestinationBlockchainID[:]).Should(Equal(destinationBlockchainID[:]))

	log.Info("Successfully SubmitCreateBridgeToken",
		"txHash", tx.Hash().Hex(),
		"messageID", hex.EncodeToString(event.MessageID[:]))

	return receipt, event.MessageID
}

func bridgeToken(
	ctx context.Context,
	source interfaces.SubnetTestInfo,
	destinationBlockchainID ids.ID,
	destinationBridgeAddress common.Address,
	token common.Address,
	recipient common.Address,
	totalAmount *big.Int,
	primaryFeeAmount *big.Int,
	secondaryFeeAmount *big.Int,
	fundedAddress common.Address,
	fundedKey *ecdsa.PrivateKey,
	transactor *erc20bridge.ERC20Bridge,
	isNative bool,
	nativeTokenChainID ids.ID,
	teleporterMessenger *teleportermessenger.TeleporterMessenger,
) (*types.Receipt, ids.ID) {
	opts, err := bind.NewKeyedTransactorWithChainID(fundedKey, source.EVMChainID)
	Expect(err).Should(BeNil())

	tx, err := transactor.BridgeTokens(
		opts,
		destinationBlockchainID,
		destinationBridgeAddress,
		token,
		recipient,
		totalAmount,
		primaryFeeAmount,
		secondaryFeeAmount,
	)
	Expect(err).Should(BeNil())

	// Wait for the transaction to be mined
	receipt := utils.WaitForTransactionSuccess(ctx, source, tx.Hash())

	event, err := utils.GetEventFromLogs(receipt.Logs, teleporterMessenger.ParseSendCrossChainMessage)
	Expect(err).Should(BeNil())
	if isNative {
		Expect(event.DestinationBlockchainID[:]).Should(Equal(destinationBlockchainID[:]))
	} else {
		Expect(event.DestinationBlockchainID[:]).Should(Equal(nativeTokenChainID[:]))
	}

	return receipt, event.MessageID
}

func approveBridgeToken(
	ctx context.Context,
	source interfaces.SubnetTestInfo,
	bridgeTokenAddress common.Address,
	transactor *bridgetoken.BridgeToken,
	amount *big.Int,
	spender common.Address,
	fundedAddress common.Address,
	fundedKey *ecdsa.PrivateKey,
) {
	opts, err := bind.NewKeyedTransactorWithChainID(fundedKey, source.EVMChainID)
	Expect(err).Should(BeNil())

	tx, err := transactor.Approve(opts, spender, amount)
	Expect(err).Should(BeNil())

	utils.WaitForTransactionSuccess(ctx, source, tx.Hash())
}
