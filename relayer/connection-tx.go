package relayer

import (
	"fmt"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	conntypes "github.com/cosmos/cosmos-sdk/x/ibc/core/03-connection/types"
	tmclient "github.com/cosmos/cosmos-sdk/x/ibc/light-clients/07-tendermint/types"
)

// CreateOpenConnections runs the connection creation messages on timeout until they pass
func (c *Chain) CreateOpenConnections(dst *Chain, maxRetries uint64, to time.Duration) error {
	// client identifiers must be filled in
	if err := ValidateClientPaths(c, dst); err != nil {
		return err
	}

	ticker := time.NewTicker(to)
	failed := uint64(0)
	for ; true; <-ticker.C {
		success, lastStep, err := ExecuteConnectionStep(c, dst)
		if err != nil {
			return err
		}

		switch {
		// In the case of success and this being the last transaction
		// debug logging, log created connection and break
		case success && lastStep:
			if c.debug {
				srcH, dstH, err := GetLatestLightHeights(c, dst)
				if err != nil {
					return err
				}
				srcConn, dstConn, err := QueryConnectionPair(c, dst, srcH, dstH)
				if err != nil {
					return err
				}
				logConnectionStates(c, dst, srcConn, dstConn)
			}

			c.Log(fmt.Sprintf("★ Connection created: [%s]client{%s}conn{%s} -> [%s]client{%s}conn{%s}",
				c.ChainID, c.PathEnd.ClientID, c.PathEnd.ConnectionID,
				dst.ChainID, dst.PathEnd.ClientID, dst.PathEnd.ConnectionID))
			return nil

		// reset the failures counter
		case success:
			failed = 0
			continue

		// increment the failures counter and exit if we used all retry attempts
		case !success:
			failed++
			c.Log(fmt.Sprintf("retrying transaction..."))
			time.Sleep(5 * time.Second)

			if failed > maxRetries {
				return fmt.Errorf("! Connection failed: [%s]client{%s}conn{%s} -> [%s]client{%s}conn{%s}",
					c.ChainID, c.PathEnd.ClientID, c.PathEnd.ConnectionID,
					dst.ChainID, dst.PathEnd.ClientID, dst.PathEnd.ConnectionID)
			}
		}
	}

	return nil
}

// ExecuteConnectionStep executes the next connection step based on the
// states of two connection ends specified by the relayer configuration
// file. The booleans return indicate if the message was successfully
// executed and if this was the last handshake step.
func ExecuteConnectionStep(src, dst *Chain) (success bool, last bool, err error) {
	// update the off chain light clients to the latest header and return the header
	sh, err := NewSyncHeaders(src, dst)
	if err != nil {
		return false, false, err
	}

	// variables needed to determine the current handshake step
	var (
		srcUpdateHeader, dstUpdateHeader *tmclient.Header
		srcConn, dstConn                 *conntypes.QueryConnectionResponse
		msgs                             []sdk.Msg
	)

	// get headers to update light clients on chain
	srcUpdateHeader, dstUpdateHeader, err = sh.GetTrustedHeaders(src, dst)
	if err != nil {
		return false, false, err
	}

	// if either identifier is missing, an existing connection that matches the required fields
	// is chosen or a new connection is created.
	// This will perform either an OpenInit or OpenTry step and return
	if src.PathEnd.ConnectionID == "" || dst.PathEnd.ConnectionID == "" {
		// TODO: Query for existing identifier and fill config, if possible
		success, err := InitializeConnection(src, dst, srcUpdateHeader, dstUpdateHeader, sh)
		if err != nil {
			return false, false, err
		}

		return success, false, nil
	}

	// Query Connection data from src and dst
	srcConn, dstConn, err = QueryConnectionPair(src, dst, int64(sh.GetHeight(src.ChainID))-1, int64(sh.GetHeight(dst.ChainID))-1)
	if err != nil {
		return false, false, err
	}

	switch {

	// OpenTry on source if both connections are at INIT (crossing hellos)
	// obtain proof of counterparty in INIT state and submit to source chain to update state
	// from INIT to TRYOPEN.
	case srcConn.Connection.State == conntypes.INIT && dstConn.Connection.State == conntypes.INIT:
		if src.debug {
			logConnectionStates(src, dst, srcConn, dstConn)
		}

		openTry, err := src.PathEnd.ConnTry(dst, sh, src.MustGetAddress())
		if err != nil {
			return false, false, err
		}

		msgs = []sdk.Msg{
			src.PathEnd.UpdateClient(dstUpdateHeader, src.MustGetAddress()),
			openTry,
		}

	// OpenAck on source if dst is at TRYOPEN and src is on INIT or TRYOPEN (crossing hellos case)
	// obtain proof of counterparty in TRYOPEN state and submit to source chain to update state
	// from INIT/TRYOPEN to OPEN.
	case (srcConn.Connection.State == conntypes.INIT || srcConn.Connection.State == conntypes.TRYOPEN) && dstConn.Connection.State == conntypes.TRYOPEN:
		if src.debug {
			logConnectionStates(src, dst, srcConn, dstConn)
		}

		openAck, err := src.PathEnd.ConnAck(dst, sh, src.MustGetAddress())
		if err != nil {
			return false, false, err
		}

		msgs = []sdk.Msg{
			src.PathEnd.UpdateClient(dstUpdateHeader, src.MustGetAddress()),
			openAck,
		}

	// OpenAck on counterparty
	// obtain proof of source in TRYOPEN state and submit to counterparty chain to update state
	// from INIT to OPEN.
	case srcConn.Connection.State == conntypes.TRYOPEN && dstConn.Connection.State == conntypes.INIT:
		if dst.debug {
			logConnectionStates(dst, src, dstConn, srcConn)
		}

		openAck, err := dst.PathEnd.ConnAck(src, sh, dst.MustGetAddress())
		if err != nil {
			return false, false, err
		}

		msgs = []sdk.Msg{
			dst.PathEnd.UpdateClient(srcUpdateHeader, dst.MustGetAddress()),
			openAck,
		}

	// OpenConfirm on source
	case srcConn.Connection.State == conntypes.TRYOPEN && dstConn.Connection.State == conntypes.OPEN:
		if src.debug {
			logConnectionStates(src, dst, srcConn, dstConn)
		}
		msgs = []sdk.Msg{
			src.PathEnd.UpdateClient(dstUpdateHeader, src.MustGetAddress()),
			src.PathEnd.ConnConfirm(dstConn, src.MustGetAddress()),
		}
		last = true

	// OpenConfirm on counterparty
	case srcConn.Connection.State == conntypes.OPEN && dstConn.Connection.State == conntypes.TRYOPEN:
		if dst.debug {
			logConnectionStates(dst, src, dstConn, srcConn)
		}
		msgs = []sdk.Msg{
			dst.PathEnd.UpdateClient(srcUpdateHeader, dst.MustGetAddress()),
			dst.PathEnd.ConnConfirm(srcConn, dst.MustGetAddress()),
		}
		last = true
	}

	_, success, err = dst.SendMsgs(msgs)
	if !success {
		return false, false, err
	}

	return true, last, nil
}

// InitializeConnection creates a new connection on either the source or destination chain .
// The identifiers set in the PathEnd's are used to determine which connection ends need to be
// initialized. The PathEnds are updated upon a successful transaction.
// NOTE: This function may need to be called twice if neither connection exists.
func InitializeConnection(src, dst *Chain, srcUpdateHeader, dstUpdateHeader *tmclient.Header, sh *SyncHeaders) (bool, error) {
	switch {

	// OpenInit on source
	// Neither connection has been initialized
	case src.PathEnd.ConnectionID == "" && dst.PathEnd.ConnectionID == "":
		if src.debug {
			// TODO: log that we are attempting to create new connection ends
		}

		// cosntruct OpenInit message to be submitted on source chain
		msgs := []sdk.Msg{
			src.PathEnd.UpdateClient(dstUpdateHeader, src.MustGetAddress()),
			src.PathEnd.ConnInit(dst.PathEnd, src.MustGetAddress()),
		}

		res, success, err := src.SendMsgs(msgs)
		if !success {
			return false, err
		}

		// update connection identifier in PathEnd
		// use index 1, connection open init is the second message in the transaction
		connectionID, err := ParseConnectionIDFromEvents(res.Logs[1].Events)
		if err != nil {
			return false, err
		}
		src.PathEnd.ConnectionID = connectionID

		return true, nil

	// OpenTry on source
	// source connection does not exist, but counterparty connection exists
	case src.PathEnd.ConnectionID == "" && dst.PathEnd.ConnectionID != "":
		if src.debug {
			// TODO: update logging
		}

		openTry, err := src.PathEnd.ConnTry(dst, sh, src.MustGetAddress())
		if err != nil {
			return false, err
		}

		msgs := []sdk.Msg{
			src.PathEnd.UpdateClient(dstUpdateHeader, src.MustGetAddress()),
			openTry,
		}
		res, success, err := src.SendMsgs(msgs)
		if !success {
			return false, err
		}

		// update connection identifier in PathEnd
		// use index 1, connection open try is the second message in the transaction
		connectionID, err := ParseConnectionIDFromEvents(res.Logs[1].Events)
		if err != nil {
			return false, err
		}
		src.PathEnd.ConnectionID = connectionID

		return true, nil

	// OpenTry on counterparty
	// source connection exists, but counterparty connection does not exist
	case src.PathEnd.ConnectionID != "" && dst.PathEnd.ConnectionID == "":
		if dst.debug {
			// TODO: update logging
		}

		openTry, err := dst.PathEnd.ConnTry(src, sh, dst.MustGetAddress())
		if err != nil {
			return false, err
		}

		msgs := []sdk.Msg{
			dst.PathEnd.UpdateClient(srcUpdateHeader, dst.MustGetAddress()),
			openTry,
		}
		res, success, err := dst.SendMsgs(msgs)
		if !success {
			return false, err
		}

		// update connection identifier in PathEnd
		// use index 1, connection open try is the second message in the transaction
		connectionID, err := ParseConnectionIDFromEvents(res.Logs[1].Events)
		if err != nil {
			return false, err
		}
		dst.PathEnd.ConnectionID = connectionID

		return true, nil

	default:
		return false, fmt.Errorf("connection ends already created")
	}
}
