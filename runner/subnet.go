package runner

import (
	"context"
	"fmt"
	"io/ioutil"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/vms/platformvm"
	"github.com/fatih/color"
)

const (
	txConfirmWait = time.Minute
	checkInterval = time.Second

	validatorWeight    = 50
	validatorStartDiff = 30 * time.Second
	validatorEndDiff   = 30 * 24 * time.Hour // 30 days

	// expected response from "CreateSubnet"
	// based on hard-coded "userPass" and "pchainFundedAddr"
	expectedSubnetTxID = "24tZhrm8j8GCJRE9PomW8FaeqbgGS4UAQjJnqqn8pq5NwYSYV1"
)

func (lc *localNetwork) createSubnet() error {
	nodeName := lc.nodeNames[0]
	color.Blue("creating subnet in %q", nodeName)
	cli := lc.apiClis[nodeName]
	subnetTxID, err := cli.PChainAPI().CreateSubnet(
		userPass,
		[]string{lc.ewoqWallet.pChainAddr}, // from
		lc.ewoqWallet.pChainAddr,           // changeAddr
		[]string{lc.ewoqWallet.pChainAddr}, // controlKeys
		1,                                  // threshold
	)
	if err != nil {
		return fmt.Errorf("failed to create subnet: %w in %q", err, nodeName)
	}
	lc.subnetTxID = subnetTxID
	if lc.subnetTxID.String() != expectedSubnetTxID {
		return fmt.Errorf("unexpected subnet tx ID %q (expected %q)", lc.subnetTxID, expectedSubnetTxID)
	}

	color.Blue("created subnet %q in %q", subnetTxID, nodeName)
	return nil
}

func (lc *localNetwork) checkSubnet(nodeName string) error {
	color.Blue("checking subnet exists %q in %q", lc.subnetTxID, nodeName)
	cli, ok := lc.apiClis[nodeName]
	if !ok {
		return fmt.Errorf("%q API client not found", nodeName)
	}
	pcli := cli.PChainAPI()

	ctx, cancel := context.WithTimeout(context.Background(), txConfirmWait)
	defer cancel()
	for ctx.Err() == nil {
		select {
		case <-lc.stopc:
			return errAborted
		case <-time.After(checkInterval):
		}

		subnets, err := pcli.GetSubnets([]ids.ID{})
		if err != nil {
			color.Yellow("failed to get subnets %v in %q", err, nodeName)
			continue
		}

		found := false
		for _, sub := range subnets {
			if sub.ID == lc.subnetTxID {
				found = true
				color.Cyan("%q returned expected subnet ID %q", nodeName, sub.ID)
				break
			}
			color.Yellow("%q returned unexpected subnet ID %q", nodeName, sub.ID)
		}
		if !found {
			color.Yellow("%q does not have subnet %q", nodeName, lc.subnetTxID)
			continue
		}

		color.Cyan("confirmed subnet exists %q in %q", lc.subnetTxID, nodeName)
		return nil
	}
	return ctx.Err()
}

func (lc *localNetwork) addSubnetValidators() error {
	color.Blue("adding subnet validator...")
	for nodeName, cli := range lc.apiClis {
		valTxID, err := cli.PChainAPI().AddSubnetValidator(
			userPass,
			[]string{lc.ewoqWallet.pChainAddr}, // from
			lc.ewoqWallet.pChainAddr,           // changeAddr
			lc.subnetTxID.String(),             // subnetID
			lc.nodeIDs[nodeName],               // nodeID
			validatorWeight,                    // stakeAmount
			uint64(time.Now().Add(validatorStartDiff).Unix()), // startTime
			uint64(time.Now().Add(validatorEndDiff).Unix()),   // endTime
		)
		if err != nil {
			return fmt.Errorf("failed to add subnet validator: %w in %q", err, nodeName)
		}
		if err := lc.checkPChainTx(nodeName, valTxID); err != nil {
			return err
		}
		color.Cyan("added subnet validator %q in %q", valTxID, nodeName)
	}
	return nil
}

func (lc *localNetwork) createBlockchain() error {
	vmGenesis, err := ioutil.ReadFile(lc.vmGenesisPath)
	if err != nil {
		return fmt.Errorf("failed to read genesis file (%s): %w", lc.vmGenesisPath, err)
	}

	color.Blue("creating blockchain with vm name %q and ID %q...", lc.vmName, lc.vmID)
	for name, cli := range lc.apiClis {
		blkChainTxID, err := cli.PChainAPI().CreateBlockchain(
			userPass,
			[]string{lc.ewoqWallet.pChainAddr}, // from
			lc.ewoqWallet.pChainAddr,           // changeAddr
			lc.subnetTxID,                      // subnetID
			lc.vmID,                            // vmID
			[]string{},                         // fxIDs
			lc.vmName,                          // name
			vmGenesis,                          // genesisData
		)
		if err != nil {
			return fmt.Errorf("failed to create blockchain: %w in %q", err, name)
		}
		lc.blkChainTxID = blkChainTxID
		color.Blue("created blockchain %q in %q", blkChainTxID, name)
		break
	}
	return nil
}

func (lc *localNetwork) checkBlockchain(nodeName string) error {
	color.Blue("checking blockchain exists %q in %q", lc.blkChainTxID, nodeName)
	cli, ok := lc.apiClis[nodeName]
	if !ok {
		return fmt.Errorf("%q API client not found", nodeName)
	}
	pcli := cli.PChainAPI()

	ctx, cancel := context.WithTimeout(context.Background(), txConfirmWait)
	defer cancel()
	for ctx.Err() == nil {
		select {
		case <-lc.stopc:
			return errAborted
		case <-time.After(checkInterval):
		}

		blockchains, err := pcli.GetBlockchains()
		if err != nil {
			color.Yellow("failed to get blockchains %v in %q", err, nodeName)
			continue
		}
		blockchainID := ids.Empty
		for _, blockchain := range blockchains {
			if blockchain.SubnetID == lc.subnetTxID {
				blockchainID = blockchain.ID
				break
			}
		}
		if blockchainID == ids.Empty {
			color.Yellow("failed to get blockchain ID in %q", nodeName)
			continue
		}
		if lc.blkChainTxID != blockchainID {
			color.Yellow("unexpected blockchain ID %q in %q (expected %q)", nodeName, lc.blkChainTxID)
			continue
		}

		status, err := pcli.GetBlockchainStatus(blockchainID.String())
		if err != nil {
			color.Yellow("failed to get blockchain status %v in %q", err, nodeName)
			continue
		}
		if status != platformvm.Validating {
			color.Yellow("blockchain status %q in %q, retrying", status, nodeName)
			continue
		}

		color.Cyan("confirmed blockchain exists and status %q in %q", status, nodeName)
		return nil
	}
	return ctx.Err()
}

func (lc *localNetwork) checkBlockchainBootstrapped(nodeName string) error {
	color.Blue("checking blockchain bootstrapped %q in %q", lc.blkChainTxID, nodeName)
	cli, ok := lc.apiClis[nodeName]
	if !ok {
		return fmt.Errorf("%q API client not found", nodeName)
	}
	icli := cli.InfoAPI()

	ctx, cancel := context.WithTimeout(context.Background(), txConfirmWait)
	defer cancel()
	for ctx.Err() == nil {
		select {
		case <-lc.stopc:
			return errAborted
		case <-time.After(checkInterval):
		}

		bootstrapped, err := icli.IsBootstrapped(lc.blkChainTxID.String())
		if err != nil {
			color.Yellow("failed to check blockchain bootstrapped %v in %q", err, nodeName)
			continue
		}
		if !bootstrapped {
			color.Yellow("blockchain %q in %q not bootstrapped yet", lc.blkChainTxID, nodeName)
			continue
		}

		color.Cyan("confirmed blockchain bootstrapped %q in %q", lc.blkChainTxID, nodeName)
		return nil
	}
	return ctx.Err()
}
