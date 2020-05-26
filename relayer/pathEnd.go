package relayer

import (
	"strings"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	clientTypes "github.com/cosmos/cosmos-sdk/x/ibc/02-client/types"
	connTypes "github.com/cosmos/cosmos-sdk/x/ibc/03-connection/types"
	chanTypes "github.com/cosmos/cosmos-sdk/x/ibc/04-channel/types"
	tmclient "github.com/cosmos/cosmos-sdk/x/ibc/07-tendermint/types"
	xferTypes "github.com/cosmos/cosmos-sdk/x/ibc/20-transfer/types"
	commitmenttypes "github.com/cosmos/cosmos-sdk/x/ibc/23-commitment/types"
	ibctypes "github.com/cosmos/cosmos-sdk/x/ibc/types"
)

// TODO: add Order chanTypes.Order as a property and wire it up in validation
// as well as in the transaction commands

// PathEnd represents the local connection identifers for a relay path
// The path is set on the chain before performing operations
type PathEnd struct {
	ChainID      string `yaml:"chain-id,omitempty" json:"chain-id,omitempty"`
	ClientID     string `yaml:"client-id,omitempty" json:"client-id,omitempty"`
	ConnectionID string `yaml:"connection-id,omitempty" json:"connection-id,omitempty"`
	ChannelID    string `yaml:"channel-id,omitempty" json:"channel-id,omitempty"`
	PortID       string `yaml:"port-id,omitempty" json:"port-id,omitempty"`
	Order        string `yaml:"order,omitempty" json:"order,omitempty"`
}

// OrderFromString parses a string into a channel order byte
func OrderFromString(order string) ibctypes.Order {
	switch order {
	case "UNORDERED":
		return ibctypes.UNORDERED
	case "ORDERED":
		return ibctypes.ORDERED
	default:
		return ibctypes.NONE
	}
}

func (pe *PathEnd) getOrder() ibctypes.Order {
	return OrderFromString(strings.ToUpper(pe.Order))
}

// UpdateClient creates an sdk.Msg to update the client on src with data pulled from dst
func (pe *PathEnd) UpdateClient(dstHeader *tmclient.Header, signer sdk.AccAddress) sdk.Msg {
	return tmclient.NewMsgUpdateClient(
		pe.ClientID,
		*dstHeader,
		signer,
	)
}

// CreateClient creates an sdk.Msg to update the client on src with consensus state from dst
func (pe *PathEnd) CreateClient(dstHeader *tmclient.Header, trustingPeriod time.Duration,
	signer sdk.AccAddress) sdk.Msg {
	if err := dstHeader.ValidateBasic(dstHeader.ChainID); err != nil {
		panic(err)
	}
	// TODO: figure out how to dynmaically set unbonding time
	return tmclient.NewMsgCreateClient(
		pe.ClientID,
		*dstHeader,
		trustingPeriod,
		defaultUnbondingTime,
		defaultMaxClockDrift,
		signer,
	)
}

// ConnInit creates a MsgConnectionOpenInit
func (pe *PathEnd) ConnInit(dst *PathEnd, signer sdk.AccAddress) sdk.Msg {
	return connTypes.NewMsgConnectionOpenInit(
		pe.ConnectionID,
		pe.ClientID,
		dst.ConnectionID,
		dst.ClientID,
		defaultChainPrefix,
		signer,
	)
}

// ConnTry creates a MsgConnectionOpenTry
// NOTE: ADD NOTE ABOUT PROOF HEIGHT CHANGE HERE
func (pe *PathEnd) ConnTry(dst *PathEnd, dstConnState connTypes.ConnectionResponse,
	dstConsState clientTypes.ConsensusStateResponse, dstCsHeight int64, signer sdk.AccAddress) sdk.Msg {
	return connTypes.NewMsgConnectionOpenTry(
		pe.ConnectionID,
		pe.ClientID,
		dst.ConnectionID,
		dst.ClientID,
		defaultChainPrefix,
		defaultIBCVersions,
		dstConnState.Proof,
		dstConsState.Proof,
		dstConnState.ProofHeight+1,
		uint64(dstCsHeight),
		signer,
	)
}

// ConnAck creates a MsgConnectionOpenAck
// NOTE: ADD NOTE ABOUT PROOF HEIGHT CHANGE HERE
func (pe *PathEnd) ConnAck(dstConnState connTypes.ConnectionResponse, dstConsState clientTypes.ConsensusStateResponse,
	dstCsHeight int64, signer sdk.AccAddress) sdk.Msg {
	return connTypes.NewMsgConnectionOpenAck(
		pe.ConnectionID,
		dstConnState.Proof,
		dstConsState.Proof,
		dstConnState.ProofHeight+1,
		uint64(dstCsHeight),
		defaultIBCVersion,
		signer,
	)
}

// ConnConfirm creates a MsgConnectionOpenAck
// NOTE: ADD NOTE ABOUT PROOF HEIGHT CHANGE HERE
func (pe *PathEnd) ConnConfirm(dstConnState connTypes.ConnectionResponse, signer sdk.AccAddress) sdk.Msg {
	return connTypes.NewMsgConnectionOpenConfirm(
		pe.ConnectionID,
		dstConnState.Proof,
		dstConnState.ProofHeight+1,
		signer,
	)
}

// ChanInit creates a MsgChannelOpenInit
func (pe *PathEnd) ChanInit(dst *PathEnd, signer sdk.AccAddress) sdk.Msg {
	return chanTypes.NewMsgChannelOpenInit(
		pe.PortID,
		pe.ChannelID,
		defaultTransferVersion,
		pe.getOrder(),
		[]string{pe.ConnectionID},
		dst.PortID,
		dst.ChannelID,
		signer,
	)
}

// ChanTry creates a MsgChannelOpenTry
func (pe *PathEnd) ChanTry(dst *PathEnd, dstChanState chanTypes.ChannelResponse, signer sdk.AccAddress) sdk.Msg {
	return chanTypes.NewMsgChannelOpenTry(
		pe.PortID,
		pe.ChannelID,
		defaultTransferVersion,
		dstChanState.Channel.Ordering,
		[]string{pe.ConnectionID},
		dst.PortID,
		dst.ChannelID,
		dstChanState.Channel.Version,
		dstChanState.Proof,
		dstChanState.ProofHeight+1,
		signer,
	)
}

// ChanAck creates a MsgChannelOpenAck
func (pe *PathEnd) ChanAck(dstChanState chanTypes.ChannelResponse, signer sdk.AccAddress) sdk.Msg {
	return chanTypes.NewMsgChannelOpenAck(
		pe.PortID,
		pe.ChannelID,
		dstChanState.Channel.Version,
		dstChanState.Proof,
		dstChanState.ProofHeight+1,
		signer,
	)
}

// ChanConfirm creates a MsgChannelOpenConfirm
func (pe *PathEnd) ChanConfirm(dstChanState chanTypes.ChannelResponse, signer sdk.AccAddress) sdk.Msg {
	return chanTypes.NewMsgChannelOpenConfirm(
		pe.PortID,
		pe.ChannelID,
		dstChanState.Proof,
		dstChanState.ProofHeight+1,
		signer,
	)
}

// ChanCloseInit creates a MsgChannelCloseInit
func (pe *PathEnd) ChanCloseInit(signer sdk.AccAddress) sdk.Msg {
	return chanTypes.NewMsgChannelCloseInit(
		pe.PortID,
		pe.ChannelID,
		signer,
	)
}

// ChanCloseConfirm creates a MsgChannelCloseConfirm
func (pe *PathEnd) ChanCloseConfirm(dstChanState chanTypes.ChannelResponse, signer sdk.AccAddress) sdk.Msg {
	return chanTypes.NewMsgChannelCloseConfirm(
		pe.PortID,
		pe.ChannelID,
		dstChanState.Proof,
		dstChanState.ProofHeight+1,
		signer,
	)
}

// MsgRecvPacket creates a MsgPacket
func (pe *PathEnd) MsgRecvPacket(dst *PathEnd, sequence, timeoutHeight, timeoutStamp uint64,
	packetData []byte, proof commitmenttypes.MerkleProof, proofHeight uint64, signer sdk.AccAddress) sdk.Msg {
	return chanTypes.NewMsgPacket(
		dst.NewPacket(
			pe,
			sequence,
			packetData,
			timeoutHeight,
			timeoutStamp,
		),
		proof,
		proofHeight+1,
		signer,
	)
}

// MsgTimeout creates MsgTimeout
func (pe *PathEnd) MsgTimeout(dst *PathEnd, packetData []byte, seq, timeout, timeoutStamp uint64,
	proof commitmenttypes.MerkleProof, proofHeight uint64, signer sdk.AccAddress) sdk.Msg {
	return chanTypes.NewMsgTimeout(
		pe.NewPacket(
			dst,
			seq,
			packetData,
			timeout,
			timeoutStamp,
		),
		seq,
		proof,
		proofHeight+1,
		signer,
	)
}

// MsgAck creates MsgAck
func (pe *PathEnd) MsgAck(dst *PathEnd, sequence, timeoutHeight, timeoutStamp uint64, ack, packetData []byte,
	proof commitmenttypes.MerkleProof, proofHeight uint64, signer sdk.AccAddress) sdk.Msg {
	return chanTypes.NewMsgAcknowledgement(
		pe.NewPacket(
			dst,
			sequence,
			packetData,
			timeoutHeight,
			timeoutStamp,
		),
		ack,
		proof,
		proofHeight+1,
		signer,
	)
}

// MsgTransfer creates a new transfer message
func (pe *PathEnd) MsgTransfer(dst *PathEnd, dstHeight uint64, amount sdk.Coins, dstAddr string,
	signer sdk.AccAddress) sdk.Msg {
	return xferTypes.NewMsgTransfer(
		pe.PortID,
		pe.ChannelID,
		dstHeight,
		amount,
		signer,
		dstAddr,
	)
}

// MsgSendPacket creates a new arbitrary packet message
func (pe *PathEnd) MsgSendPacket(dst *PathEnd, packetData []byte, relativeTimeout, timeoutStamp uint64,
	signer sdk.AccAddress) sdk.Msg {
	// NOTE: Use this just to pass the packet integrity checks.
	fakeSequence := uint64(1)
	packet := chanTypes.NewPacket(packetData, fakeSequence, pe.PortID, pe.ChannelID, dst.PortID,
		dst.ChannelID, relativeTimeout, timeoutStamp)
	return NewMsgSendPacket(packet, signer)
}

// NewPacket returns a new packet from src to dist w
func (pe *PathEnd) NewPacket(dst *PathEnd, sequence uint64, packetData []byte,
	timeoutHeight, timeoutStamp uint64) chanTypes.Packet {
	return chanTypes.NewPacket(
		packetData,
		sequence,
		pe.PortID,
		pe.ChannelID,
		dst.PortID,
		dst.ChannelID,
		timeoutHeight,
		timeoutStamp,
	)
}

// XferPacket creates a new transfer packet
func (pe *PathEnd) XferPacket(amount sdk.Coins, sender, receiver string) []byte {
	return xferTypes.NewFungibleTokenPacketData(
		amount,
		sender,
		receiver,
	).GetBytes()
}

// PacketMsg returns a new MsgPacket for forwarding packets from one chain to another
func (c *Chain) PacketMsg(dst *Chain, xferPacket []byte, timeout, timeoutStamp uint64,
	seq int64, dstCommitRes CommitmentResponse) sdk.Msg {
	return c.PathEnd.MsgRecvPacket(
		dst.PathEnd,
		uint64(seq),
		timeout,
		timeoutStamp,
		xferPacket,
		dstCommitRes.Proof,
		dstCommitRes.ProofHeight,
		c.MustGetAddress(),
	)
}
