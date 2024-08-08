package pushtxman

import (
	"context"
	"errors"
	"fmt"
	"github.com/ethereum/go-ethereum"
	"strconv"
	"strings"
	"time"

	"github.com/0xPolygonHermez/zkevm-bridge-service/etherman"
	"github.com/0xPolygonHermez/zkevm-bridge-service/log"
	"github.com/0xPolygonHermez/zkevm-bridge-service/utils"
	"github.com/ethereum/go-ethereum/common"
)

type PushTxManager struct {
	ctx         context.Context
	cancel      context.CancelFunc
	index       int
	l2Name      string
	l2Node      *utils.Client
	l2NetworkID uint
	depositMgr  *DepositManager
	cfg         Config
	storage     storageInterface
	chPush      map[uint]chan *etherman.Deposit
	chClaim     map[uint]chan *etherman.Claim
}

// NewPushTxManager creates a new push transaction manager.
func NewPushTxManager(cfg Config, idx int, chPush map[uint]chan *etherman.Deposit, chClaim map[uint]chan *etherman.Claim,
	depositMgr *DepositManager, storage interface{}) (*PushTxManager, error) {
	ctx := context.Background()
	client, err := utils.NewClient(ctx, cfg.NodeRpcs[idx].Url, common.HexToAddress(""))
	if err != nil {
		return nil, err
	}
	chainID := convertChainID(cfg.NodeRpcs[idx].ChainID)
	chPush[chainID] = make(chan *etherman.Deposit, 10000)
	chClaim[chainID] = make(chan *etherman.Claim, 10000)
	ctx, cancel := context.WithCancel(ctx)
	log.Infof("Init %s pushTx manager success", cfg.NodeRpcs[idx].Name)
	return &PushTxManager{
		ctx:         ctx,
		cancel:      cancel,
		index:       idx,
		l2Name:      cfg.NodeRpcs[idx].Name,
		l2Node:      client,
		l2NetworkID: chainID,
		depositMgr:  depositMgr,
		cfg:         cfg,
		storage:     storage.(storageInterface),
		chPush:      chPush,
		chClaim:     chClaim,
	}, err
}

func convertChainID(chainID uint) uint {
	if chainID == etherman.VIZING_TESTNET || chainID == etherman.VIZING_MAINNET {
		return 1
	}
	return chainID
}

// Start will start the tx management, reading txs from storage,
// send then to the blockchain and keep monitoring them until they
// get mined
func (tm *PushTxManager) Start() {
	ticker := time.NewTicker(tm.cfg.FrequencyToMonitorTxs.Duration)
	for {
		select {
		case <-tm.ctx.Done():
			log.Warnf("pushtxmanager %s exit", tm.l2Name)
			return
		case deposit := <-tm.chPush[tm.l2NetworkID]:
			err := tm.depositTxs(context.Background(), deposit)
			if err != nil {
				log.Errorf("failed to %s deposit txs: %v", tm.l2Name, err)
			}
		case <-ticker.C:
			err := tm.scanDepositTxs(context.Background())
			if err != nil {
				log.Errorf("failed to %s deposit txs: %v", tm.l2Name, err)
			}
			err = tm.scanClaimTxs(context.Background())
			if err != nil {
				log.Errorf("failed to %s claim txs: %v", tm.l2Name, err)
			}
		case claim := <-tm.chClaim[tm.l2NetworkID]:
			err := tm.claimTxs(context.Background(), claim)
			if err != nil {
				log.Errorf("failed to %s claim txs: %v", tm.l2Name, err)
			}
		}
	}
}

func (tm *PushTxManager) depositTxs(ctx context.Context, mTx *etherman.Deposit) error {
	tx, err := tm.l2Node.TransactionReceipt(ctx, mTx.TxHash)
	if err != nil {
		if errors.Is(err, ethereum.NotFound) {
			tm.depositMgr.AddInvalidDeposit(ctx, mTx.Id)
		}
		log.Errorf("[pushTxManager %s] error getting depositTxByHash %s. Error: %v", tm.l2Name, mTx.TxHash, err)
		return err
	}
	if tx == nil {
		tm.depositMgr.AddInvalidDeposit(ctx, mTx.Id)
		return nil
	}
	if tx.BlockNumber == nil {
		tm.depositMgr.AddInvalidDeposit(ctx, mTx.Id)
		log.Infof("[pushTxManager %s] depositTx: %s not mined yet", tm.l2Name, mTx.TxHash)
		return nil
	}
	tm.depositMgr.DelInvalidDeposit(ctx, mTx.Id)
	dbTx, err := tm.storage.BeginDBTransaction(ctx)
	if err != nil {
		return err
	}
	block := &etherman.Block{
		BlockNumber: tx.BlockNumber.Uint64(),
		BlockHash:   tx.BlockHash,
		ParentHash:  tx.BlockHash,
		NetworkID:   convertChainID(tm.l2NetworkID),
		ReceivedAt:  time.Now(),
	}
	blockID, err := tm.storage.AddBlock(ctx, block, dbTx)
	if err != nil {
		log.Infof("[pushTxManager %s] depositTx: %s, add block err: %v", tm.l2Name, mTx.TxHash, err)
		rollbackErr := tm.storage.Rollback(ctx, dbTx)
		if rollbackErr != nil {
			log.Errorf("[pushTxManager %s] error rolling back state. RollbackErr: %s, err: %v", tm.l2Name, rollbackErr.Error(), err)
		}
		return err
	}

	err = tm.storage.UpdatePushDepositsBlock(ctx, mTx.Id, blockID, dbTx)
	if err != nil {
		log.Infof("[pushTxManager %s] depositTx: %s, update blockID err: %v", tm.l2Name, mTx.TxHash, err)
		rollbackErr := tm.storage.Rollback(ctx, dbTx)
		if rollbackErr != nil {
			log.Errorf("[pushTxManager %s] error rolling back state. RollbackErr: %s, err: %v", tm.l2Name, rollbackErr.Error(), err)
		}
		return err
	}
	err = tm.storage.Commit(ctx, dbTx)
	if err != nil {
		log.Errorf("[pushTxManager %s] UpdateDepositTx committing dbTx, err: %v", tm.l2Name, err)
		rollbackErr := tm.storage.Rollback(ctx, dbTx)
		if rollbackErr != nil {
			log.Errorf("[pushTxManager %s] error rolling back state. RollbackErr: %s, err: %v", tm.l2Name, rollbackErr.Error(), err)
		}
		return err
	}
	return nil
}

func (tm *PushTxManager) scanDepositTxs(ctx context.Context) error {
	mTxs, err := tm.storage.GetPendingPushDeposits(ctx, tm.l2NetworkID, 20, 0, nil)
	if err != nil {
		log.Errorf("failed to get %s pending deposit txs: %v", tm.l2Name, err)
		return fmt.Errorf("failed to get %s pending deposit txs: %v", tm.l2Name, err)
	}

	log.Infof("%s found %v pending deposit tx to process", tm.l2Name, len(mTxs))
	for _, v := range mTxs {
		mTx := v // force variable shadowing to avoid pointer conflicts
		_ = tm.depositTxs(ctx, mTx)
	}
	return nil
}

func (tm *PushTxManager) scanClaimTxs(ctx context.Context) error {
	mTxs, err := tm.storage.GetPendingPushTxsStatus(ctx, tm.l2NetworkID, 20, 0, nil)
	if err != nil {
		log.Errorf("failed to get %s pending claim txs: %v", tm.l2Name, err)
		return fmt.Errorf("failed to get %s pending claim txs: %v", tm.l2Name, err)
	}

	log.Infof("%s found %v pending claim tx to process", tm.l2Name, len(mTxs))
	for _, v := range mTxs {
		mTx := v // force variable shadowing to avoid pointer conflicts
		if mTx.ReadyForClaim {
			continue
		}
		r := utils.NewHTTPCli()
		ret, err := r.Get(tm.cfg.FullChainAPI + mTx.TxHash.String())
		if err != nil {
			continue
		}
		retRes := &TxStatus{}
		err = ret.Parse(retRes)
		if err != nil {
			continue
		}
		log.Debugf("%s deposit ——> claim tx result: %+v", tm.l2Name, retRes.Result)
		if retRes.Result.Status != TX_STATUS {
			log.Debugf("%s claim tx add invalid deposit", tm.l2Name)
			tm.depositMgr.AddInvalidDeposit(ctx, mTx.Id)
			continue
		}
		tm.depositMgr.DelInvalidDeposit(ctx, mTx.Id)
		if retRes.Result.OpStatus != OP_SUCCESS {
			continue
		}

		origNetID, _ := strconv.ParseUint(retRes.Result.ChainId, 10, 64)
		destNetID, _ := strconv.ParseUint(retRes.Result.TargetChain, 10, 64)
		//amount, _ := new(big.Int).SetString(retRes.Result.Amount, 10)
		targetHash := ""
		hashs := strings.Split(retRes.Result.TargetId, "-")
		if len(hashs) > 0 {
			targetHash = hashs[0]
		}
		if targetHash == "" || targetHash[:2] != "0x" || len(targetHash) < 30 {
			continue
		}
		claim := &etherman.Claim{
			TxHash:             common.HexToHash(targetHash),
			OriginalNetwork:    convertChainID(uint(origNetID)),
			OriginalAddress:    mTx.OriginalAddress,
			DestinationAddress: mTx.OriginalAddress,
			Amount:             mTx.Amount,
			NetworkID:          convertChainID(uint(destNetID)),
			Index:              mTx.DepositCount,
		}
		tm.chClaim[claim.NetworkID] <- claim
	}
	return nil
}

func (tm *PushTxManager) claimTxs(ctx context.Context, claim *etherman.Claim) error {
	if !tm.depositMgr.TryInvalidClaim(ctx, claim.TxHash.Hex()) {
		return nil
	}
	log.Debugf("%s claim: %+v", tm.l2Name, claim)
	tx, err := tm.l2Node.TransactionReceipt(ctx, claim.TxHash)
	if err != nil {
		if errors.Is(err, ethereum.NotFound) {
			tm.depositMgr.AddInvalidClaim(ctx, claim.TxHash.Hex())
		}
		log.Errorf("[pushTxManager %s] error getting claimTxByHash %s. Error: %v", tm.l2Name, claim.TxHash, err)
		return err
	}
	if tx == nil {
		tm.depositMgr.AddInvalidClaim(ctx, claim.TxHash.Hex())
		log.Debugf("%s claim tx is null, %s", tm.l2Name, claim.TxHash)
		return nil
	}
	if tx.BlockNumber == nil {
		tm.depositMgr.AddInvalidClaim(ctx, claim.TxHash.Hex())
		log.Infof("[pushTxManager %s] claimTx: %s not mined yet", tm.l2Name, claim.TxHash)
		return nil
	}
	tm.depositMgr.DelInvalidClaim(ctx, claim.TxHash.Hex())
	dbTx, err := tm.storage.BeginDBTransaction(ctx)
	if err != nil {
		return err
	}
	block := &etherman.Block{
		BlockNumber: tx.BlockNumber.Uint64(),
		BlockHash:   tx.BlockHash,
		ParentHash:  tx.BlockHash,
		NetworkID:   convertChainID(claim.NetworkID),
		ReceivedAt:  time.Now(),
	}
	blockID, err := tm.storage.AddBlock(ctx, block, dbTx)
	if err != nil {
		log.Infof("[pushTxManager %s] claimTx: %s, add block err: %v", tm.l2Name, claim.TxHash, err)
		rollbackErr := tm.storage.Rollback(ctx, dbTx)
		if rollbackErr != nil {
			log.Errorf("[pushTxManager %s] error rolling back state. RollbackErr: %s, err: %v", tm.l2Name, rollbackErr.Error(), err)
		}
		return err
	}
	claim.BlockID = blockID
	err = tm.storage.AddClaim(ctx, claim, dbTx)
	if err != nil {
		log.Infof("[pushTxManager %s] add claim：%s err: %v", tm.l2Name, claim.TxHash, err)
		rollbackErr := tm.storage.Rollback(ctx, dbTx)
		if rollbackErr != nil {
			log.Errorf("[pushTxManager %s] error rolling back state. RollbackErr: %s, err: %v", tm.l2Name, rollbackErr.Error(), err)
		}
		return err
	}
	err = tm.storage.UpdatePushDepositsStatus(ctx, convertChainID(claim.OriginalNetwork), convertChainID(claim.NetworkID), claim.OriginalAddress.Hex(), claim.Index, dbTx)
	if err != nil {
		log.Infof("[pushTxManager %s] update push deposit status err: %v", tm.l2Name, err)
		rollbackErr := tm.storage.Rollback(ctx, dbTx)
		if rollbackErr != nil {
			log.Errorf("[pushTxManager %s] error rolling back state. RollbackErr: %s, err: %v", tm.l2Name, rollbackErr.Error(), err)
		}
		return err
	}
	err = tm.storage.Commit(ctx, dbTx)
	if err != nil {
		log.Errorf("[pushTxManager %s] claimTx: %s, committing dbTx, err: %v", tm.l2Name, claim.TxHash, err)
		rollbackErr := tm.storage.Rollback(ctx, dbTx)
		if rollbackErr != nil {
			log.Errorf("[pushTxManager %s] error rolling back state. RollbackErr: %s, err: %v", tm.l2Name, rollbackErr.Error(), err)
		}
		return err
	}
	return nil
}
