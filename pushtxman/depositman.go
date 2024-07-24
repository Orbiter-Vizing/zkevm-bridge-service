package pushtxman

import (
	"context"
	"errors"
	"fmt"
	"github.com/0xPolygonHermez/zkevm-bridge-service/log"
	"github.com/jackc/pgx/v4"
	"sync"
	"time"
)

type DepositManager struct {
	ctx            context.Context
	cfg            Config
	storage        storageInterface
	depositCnt     map[uint]int
	lock           sync.Mutex
	invalidDeposit sync.Map
	invalidClaim   sync.Map
	existDeposit   sync.Map
}

func NewDepositManager(cfg Config, storage interface{}) *DepositManager {
	ctx := context.Background()
	log.Infof("Init deposit manager success")
	return &DepositManager{
		ctx:            ctx,
		cfg:            cfg,
		storage:        storage.(storageInterface),
		depositCnt:     make(map[uint]int),
		invalidDeposit: sync.Map{},
		invalidClaim:   sync.Map{},
		existDeposit:   sync.Map{},
	}
}

func (tm *DepositManager) Start() {
	ticker := time.NewTicker(time.Minute)
	for {
		select {
		case <-ticker.C:
			tm.CleanInvalidDeposit()
			tm.CleanExistDeposit()
		}
	}
}

func (tm *DepositManager) CleanExistDeposit() {
	tm.existDeposit.Range(func(key, value any) bool {
		if t, ok := value.(time.Time); ok {
			if time.Since(t).Minutes() > 2 {
				tm.existDeposit.Delete(key)
			}
		}
		return true
	})
}

func (tm *DepositManager) CleanInvalidDeposit() {
	invalidCnt := 0
	tm.invalidDeposit.Range(func(key, value any) bool {
		t, ok := value.(time.Time)
		if !ok {
			invalidCnt++
			return true
		}
		diff := time.Now().Sub(t)
		if diff.Hours() > 24 {
			err := tm.storage.DelPushDeposit(context.Background(), key.(uint64), nil)
			if err == nil {
				tm.invalidDeposit.Delete(key)
			} else {
				invalidCnt++
			}
		} else {
			invalidCnt++
		}
		return true
	})
	log.Infof("invalid push tx deposit count: %d", invalidCnt)
}

func (tm *DepositManager) AddInvalidDeposit(ctx context.Context, tx uint64) {
	if _, ok := tm.invalidDeposit.Load(tx); !ok {
		log.Infof("invalid deposit: %d", tx)
		tm.invalidDeposit.Store(tx, time.Now())
	}
}

func (tm *DepositManager) DelInvalidDeposit(ctx context.Context, tx uint64) {
	tm.invalidDeposit.Delete(tx)
}

func (tm *DepositManager) AddInvalidClaim(ctx context.Context, tx string) {
	if _, ok := tm.invalidClaim.Load(tx); !ok {
		log.Infof("invalid claim: %d", tx)
		tm.invalidClaim.Store(tx, time.Now())
	}
}

func (tm *DepositManager) DelInvalidClaim(ctx context.Context, tx string) {
	tm.invalidClaim.Delete(tx)
}

func (tm *DepositManager) TryInvalidClaim(ctx context.Context, tx string) bool {
	if expiredAt, ok := tm.invalidClaim.Load(tx); ok {
		if time.Now().Sub(expiredAt.(time.Time)).Hours() > 1 {
			return false
		}
	}
	return true
}

func (tm *DepositManager) GetDepositCnt(ctx context.Context, networkID uint) (int, error) {
	tm.lock.Lock()
	defer tm.lock.Unlock()
	var err error
	minIndex, ok := tm.depositCnt[networkID]
	if ok {
		minIndex -= 1
		tm.depositCnt[networkID] = minIndex
		return minIndex, nil
	}
	minIndex, err = tm.storage.GetMinDepositCount(ctx, networkID, nil)
	if errors.Is(err, pgx.ErrNoRows) {
		minIndex = 0
	} else if err != nil {
		return minIndex, err
	}
	minIndex -= 1
	tm.depositCnt[networkID] = minIndex
	return minIndex, nil
}

func (tm *DepositManager) AddExistDeposit(ctx context.Context, networkID uint, tx string) {
	tm.existDeposit.Store(fmt.Sprintf("%d-%s", networkID, tx), time.Now())
}

func (tm *DepositManager) DelExistDeposit(ctx context.Context, networkID uint, tx string) {
	tm.existDeposit.Delete(fmt.Sprintf("%d-%s", networkID, tx))
}

func (tm *DepositManager) IsExistDeposit(ctx context.Context, networkID uint, tx string) bool {
	if _, ok := tm.existDeposit.Load(fmt.Sprintf("%d-%s", networkID, tx)); ok {
		return true
	}
	return false
}
