package pushtxman

import (
	"context"

	"github.com/0xPolygonHermez/zkevm-bridge-service/bridgectrl"
	"github.com/0xPolygonHermez/zkevm-bridge-service/claimtxman/types"
	"github.com/0xPolygonHermez/zkevm-bridge-service/etherman"
	"github.com/jackc/pgx/v4"
)

type storageInterface interface {
	AddBlock(ctx context.Context, block *etherman.Block, dbTx pgx.Tx) (uint64, error)
	UpdateL1DepositsStatus(ctx context.Context, exitRoot []byte, dbTx pgx.Tx) ([]*etherman.Deposit, error)
	UpdateL2DepositsStatus(ctx context.Context, exitRoot []byte, networkID uint, dbTx pgx.Tx) error
	AddClaimTx(ctx context.Context, mTx types.MonitoredTx, dbTx pgx.Tx) error
	UpdateClaimTx(ctx context.Context, mTx types.MonitoredTx, dbTx pgx.Tx) error
	GetClaimTxsByStatus(ctx context.Context, statuses []types.MonitoredTxStatus, dbTx pgx.Tx) ([]types.MonitoredTx, error)
	// atomic
	Rollback(ctx context.Context, dbTx pgx.Tx) error
	BeginDBTransaction(ctx context.Context) (pgx.Tx, error)
	Commit(ctx context.Context, dbTx pgx.Tx) error
	GetPendingPushDeposits(context.Context, uint, uint, uint, pgx.Tx) ([]*etherman.Deposit, error)
	UpdatePushDepositsBlock(context.Context, uint64, uint64, pgx.Tx) error
	GetPendingPushTxsStatus(context.Context, uint, uint, uint, pgx.Tx) ([]*etherman.Deposit, error)
	AddClaim(ctx context.Context, claim *etherman.Claim, dbTx pgx.Tx) error
	UpdatePushDepositsStatus(context.Context, uint, uint, string, int, pgx.Tx) error
	GetMinDepositCount(ctx context.Context, networkID uint, dbTx pgx.Tx) (int, error)
	DelPushDeposit(ctx context.Context, depositID uint64, dbTx pgx.Tx) error
}

type bridgeServiceInterface interface {
	GetClaimProof(depositCnt int, networkID uint, dbTx pgx.Tx) (*etherman.GlobalExitRoot, [][bridgectrl.KeyLen]byte, error)
	GetDepositStatus(ctx context.Context, depositCount int, destNetworkID uint) (string, error)
}
