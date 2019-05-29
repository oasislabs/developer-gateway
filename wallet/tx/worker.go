package tx

import (
	"context"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/oasislabs/developer-gateway/conc"
	"github.com/oasislabs/developer-gateway/wallet/core"
)

const (
	maxConcurrentWallets = 128
)

type signRequest struct {
	Transaction *types.Transaction
}

// Worker implements a very simple transaction signing service/
type Worker struct {
	key    string
	wallet core.Wallet
}

// NewWorker creates a new instance of a worker
func NewWorker(key string) *Worker {
	w := &Worker{
		key:    key,
		wallet: core.InternalWallet{
			PrivateKey: // TODO(ennsharma) initialize these fields 
			Signer:
			Nonce:
			Client:
			Logger: 
		},
	}

	return w
}

func (w *Worker) handle(ctx context.Context, ev conc.WorkerEvent) (interface{}, error) {
	switch ev := ev.(type) {
	case conc.RequestWorkerEvent:
		return w.handleRequestEvent(ctx, ev)
	case conc.ErrorWorkerEvent:
		return w.handleErrorEvent(ctx, ev)
	default:
		panic("receive unexpected event type")
	}
}

func (w *Worker) handleRequestEvent(ctx context.Context, ev conc.RequestWorkerEvent) (interface{}, error) {
	switch req := ev.Value.(type) {
	case signRequest:
		err := w.sign(req)
		return nil, err
	default:
		panic("invalid request received for worker")
	}
}

func (w *Worker) handleErrorEvent(ctx context.Context, ev conc.ErrorWorkerEvent) (interface{}, error) {
	// a worker should not be passing errors to the conc.Worker so
	// in that case the error is returned and the execution of the
	// worker should halt
	return nil, ev.Error
}

func (w *Worker) sign(req signRequest) (*types.Transaction, errors.Err) {
	return w.wallet.SignTransaction(req.Transaction)
}
