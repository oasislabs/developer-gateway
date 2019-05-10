package eth

import (
	"context"
	"crypto/ecdsa"
	stderr "errors"
	"fmt"
	"math/big"
	"net/url"
	"strings"
	"sync"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	erpc "github.com/ethereum/go-ethereum/rpc"

	"github.com/oasislabs/developer-gateway/api/v0/service"
	backend "github.com/oasislabs/developer-gateway/backend/core"
	"github.com/oasislabs/developer-gateway/errors"
	"github.com/oasislabs/developer-gateway/log"
	"github.com/oasislabs/developer-gateway/rpc"
)

const gasPrice int64 = 1000000000

type ethRequest interface {
	RequestID() uint64
	IncAttempts()
	GetAttempts() uint
	GetContext() context.Context
	OutCh() chan<- backend.Event
}

type oasisPublicKeyPayload struct {
	Timestamp uint64 `json:"timestamp"`
	PublicKey string `json:"public_key"`
	Signature string `json:"signature"`
}

type executeServiceRequest struct {
	Attempts uint
	Out      chan backend.Event
	Context  context.Context
	ID       uint64
	Request  backend.ExecuteServiceRequest
}

type deployServiceRequest struct {
	Attempts uint
	Out      chan backend.Event
	Context  context.Context
	ID       uint64
	Request  backend.DeployServiceRequest
}

func (r *executeServiceRequest) RequestID() uint64 {
	return r.ID
}

func (r *executeServiceRequest) IncAttempts() {
	r.Attempts++
}

func (r *executeServiceRequest) GetAttempts() uint {
	return r.Attempts
}

func (r *executeServiceRequest) OutCh() chan<- backend.Event {
	return r.Out
}

func (r *executeServiceRequest) GetContext() context.Context {
	return r.Context
}

func (r *deployServiceRequest) RequestID() uint64 {
	return r.ID
}

func (r *deployServiceRequest) IncAttempts() {
	r.Attempts++
}

func (r *deployServiceRequest) GetAttempts() uint {
	return r.Attempts
}

func (r *deployServiceRequest) OutCh() chan<- backend.Event {
	return r.Out
}

func (r *deployServiceRequest) GetContext() context.Context {
	return r.Context
}

type Wallet struct {
	PrivateKey *ecdsa.PrivateKey
}

type EthClientProperties struct {
	Wallet Wallet
	URL    string
}

type EthClient struct {
	ctx       context.Context
	wg        sync.WaitGroup
	inCh      chan ethRequest
	logger    log.Logger
	wallet    Wallet
	nonce     uint64
	signer    types.Signer
	rpcClient *erpc.Client
	client    *ethclient.Client
}

func (c *EthClient) startLoop(ctx context.Context) {
	c.wg.Add(1)

	go func() {
		defer func() {
			c.wg.Done()
		}()

		for {
			select {
			case <-c.ctx.Done():
				return
			case req, ok := <-c.inCh:
				if !ok {
					return
				}

				c.request(req)
			}
		}
	}()
}

func (c *EthClient) Stop() {
	close(c.inCh)
	c.wg.Wait()
}

func (c *EthClient) runTransaction(req ethRequest, fn func(uint64, ethRequest) (backend.Event, error)) {
	if req.GetAttempts() >= 10 {
		req.OutCh() <- service.ErrorEvent{
			ID:    req.RequestID(),
			Cause: rpc.Error{Description: "failed to execute service", ErrorCode: -1},
		}
		return
	}

	if req.GetAttempts() > 0 {
		// in case of previous failure make sure that the account nonce is correct
		if err := c.updateNonce(req.GetContext()); err != nil {
			req.OutCh() <- service.ErrorEvent{
				ID:    req.RequestID(),
				Cause: rpc.Error{Description: "failed to update nonce", ErrorCode: -1},
			}
			return
		}
	}

	nonce := c.nonce
	c.nonce++

	go func() {
		event, err := fn(nonce, req)
		if err != nil {
			// attempt a retry if there is a problem with the nonce.
			if strings.Contains(err.Error(), "nonce") {
				req.IncAttempts()
				c.inCh <- req
				return
			}

			event = backend.ErrorEvent{
				ID:    req.RequestID(),
				Cause: rpc.Error{Description: err.Error(), ErrorCode: -1},
			}
		}

		req.OutCh() <- event
	}()
}

func (c *EthClient) request(req ethRequest) {
	switch req := req.(type) {
	case *executeServiceRequest:
		c.runTransaction(req, func(nonce uint64, req ethRequest) (backend.Event, error) {
			request := req.(*executeServiceRequest)
			return c.executeService(request.Context, nonce, request.ID, request.Request)
		})
	case *deployServiceRequest:
		c.runTransaction(req, func(nonce uint64, req ethRequest) (backend.Event, error) {
			request := req.(*deployServiceRequest)
			return c.deployService(request.Context, nonce, request.ID, request.Request)
		})
	default:
		panic("invalid request type received")
	}
}

func (c *EthClient) updateNonce(ctx context.Context) error {
	for attempts := 0; attempts < 10; attempts++ {
		nonce, err := c.Nonce(ctx, crypto.PubkeyToAddress(c.wallet.PrivateKey.PublicKey).Hex())
		if err != nil {
			continue
		}

		if c.nonce < nonce {
			c.nonce = nonce
		}

		return nil
	}

	return stderr.New("exceeded attempts to update nonce")
}

func (c *EthClient) GetPublicKeyService(
	ctx context.Context,
	req backend.GetPublicKeyServiceRequest,
) (backend.GetPublicKeyServiceResponse, errors.Err) {
	c.logger.Debug(ctx, "", log.MapFields{
		"call_type": "GetPublicKeyServiceAttempt",
		"address":   req.Address,
	})

	var payload oasisPublicKeyPayload
	if err := c.rpcClient.CallContext(ctx, &payload, "oasis_getPublicKey", req.Address); err != nil {
		c.logger.Debug(ctx, "client call failed", log.MapFields{
			"call_type": "GetPublicKeyServiceFailure",
			"address":   req.Address,
		})
		return backend.GetPublicKeyServiceResponse{},
			errors.New(errors.ErrInternalError, fmt.Errorf("failed to get public key %s", err.Error()))
	}

	c.logger.Debug(ctx, "", log.MapFields{
		"call_type": "GetPublicKeyServiceSuccess",
		"address":   req.Address,
	})

	return backend.GetPublicKeyServiceResponse{
		Address:   req.Address,
		Timestamp: payload.Timestamp,
		PublicKey: payload.PublicKey,
		Signature: payload.Signature,
	}, nil
}

func (c *EthClient) DeployService(
	ctx context.Context,
	id uint64,
	req backend.DeployServiceRequest,
) backend.Event {
	out := make(chan backend.Event)
	c.inCh <- &deployServiceRequest{Attempts: 0, Out: out, Context: ctx, ID: id, Request: req}
	return <-out
}

func (c *EthClient) ExecuteService(
	ctx context.Context,
	id uint64,
	req backend.ExecuteServiceRequest,
) backend.Event {
	if len(req.Address) == 0 {
		return backend.ErrorEvent{
			ID: id,
			Cause: rpc.Error{
				Description: errors.ErrInvalidAddress.Desc(),
				ErrorCode:   errors.ErrInvalidAddress.Code(),
			},
		}
	}

	out := make(chan backend.Event)
	c.inCh <- &executeServiceRequest{Attempts: 0, Out: out, Context: ctx, ID: id, Request: req}
	return <-out
}

func (c *EthClient) executeTransaction(
	ctx context.Context,
	nonce uint64,
	id uint64,
	address string,
	data []byte,
) (backend.Event, error) {
	c.logger.Debug(ctx, "", log.MapFields{
		"call_type": "ExecuteTransactionAttempt",
		"id":        id,
		"address":   address,
	})

	gas, err := c.estimateGas(ctx, id, address, data)
	if err != nil {
		err := errors.New(errors.ErrEstimateGas, err)
		c.logger.Debug(ctx, "failed to estimate gas", log.MapFields{
			"call_type": "ExecuteTransactionFailure",
			"id":        id,
			"address":   address,
		}, err)

		return backend.ErrorEvent{
			ID: id,
			Cause: rpc.Error{
				Description: errors.ErrEstimateGas.Desc(),
				ErrorCode:   errors.ErrEstimateGas.Code(),
			},
		}, nil
	}

	var tx *types.Transaction
	if len(address) == 0 {
		tx = types.NewContractCreation(nonce,
			big.NewInt(0), gas, big.NewInt(gasPrice), data)
	} else {
		tx = types.NewTransaction(nonce, common.HexToAddress(address),
			big.NewInt(0), gas, big.NewInt(gasPrice), data)
	}

	tx, err = types.SignTx(tx, c.signer, c.wallet.PrivateKey)
	if err != nil {
		err := errors.New(errors.ErrSignedTx, err)
		c.logger.Debug(ctx, "failure to sign transaction", log.MapFields{
			"call_type": "ExecuteTransactionFailure",
			"id":        id,
			"address":   address,
		}, err)

		return backend.ErrorEvent{
			ID: id,
			Cause: rpc.Error{
				Description: errors.ErrSignedTx.Desc(),
				ErrorCode:   errors.ErrSignedTx.Code(),
			},
		}, nil
	}

	if err := c.client.SendTransaction(ctx, tx); err != nil {
		// depending on the error received it may be useful to return the error
		// and have an upper logic to decide whether to retry the request
		err := errors.New(errors.ErrSendTransaction, err)
		c.logger.Debug(ctx, "failure to send transaction", log.MapFields{
			"call_type": "ExecuteTransactionFailure",
			"id":        id,
			"address":   address,
		}, err)

		return nil, err
	}

	receipt, err := c.client.TransactionReceipt(ctx, tx.Hash())
	if err != nil {
		err := errors.New(errors.ErrTransactionReceipt, err)
		c.logger.Debug(ctx, "failure to retrieve transaction receipt", log.MapFields{
			"call_type": "ExecuteTransactionFailure",
			"id":        id,
			"address":   address,
		}, err)

		return backend.ErrorEvent{
			ID: id,
			Cause: rpc.Error{
				Description: errors.ErrTransactionReceipt.Desc(),
				ErrorCode:   errors.ErrTransactionReceipt.Code(),
			},
		}, nil
	}

	if receipt.Status != 1 {
		err := errors.New(errors.ErrTransactionReceipt, stderr.New(
			"transaction receipt has status 0 which indicates a transaction execution failure"))
		c.logger.Debug(ctx, "transaction execution failed", log.MapFields{
			"call_type": "ExecuteTransactionFailure",
			"id":        id,
			"address":   address,
		}, err)

		return backend.ErrorEvent{
			ID: id,
			Cause: rpc.Error{
				Description: errors.ErrTransactionReceiptStatus.Desc(),
				ErrorCode:   errors.ErrTransactionReceiptStatus.Code(),
			},
		}, nil
	}

	c.logger.Debug(ctx, "transaction sent successfully", log.MapFields{
		"call_type": "ExecuteTransactionSuccess",
		"id":        id,
		"address":   address,
	})
	return backend.ExecuteServiceEvent{
		ID:      id,
		Address: receipt.ContractAddress.Hex(),
	}, nil
}

func (c *EthClient) deployService(ctx context.Context, nonce, id uint64, req backend.DeployServiceRequest) (backend.Event, error) {
	return c.executeTransaction(ctx, nonce, id, "", []byte(req.Data))
}

func (c *EthClient) executeService(ctx context.Context, nonce, id uint64, req backend.ExecuteServiceRequest) (backend.Event, error) {
	return c.executeTransaction(ctx, nonce, id, req.Address, []byte(req.Data))
}

func (c *EthClient) Nonce(ctx context.Context, address string) (uint64, errors.Err) {
	c.logger.Debug(ctx, "", log.MapFields{
		"call_type": "NonceAttempt",
		"address":   address,
	})

	nonce, err := c.client.PendingNonceAt(ctx, common.HexToAddress(address))
	if err != nil {
		err := errors.New(errors.ErrFetchPendingNonce, err)
		c.logger.Debug(ctx, "PendingNonceAt request failed", log.MapFields{
			"call_type": "NonceFailure",
			"address":   address,
		}, err)

		return 0, err
	}

	c.logger.Debug(ctx, "", log.MapFields{
		"call_type": "NonceSuccess",
		"address":   address,
	})

	return nonce, nil
}

func (c *EthClient) estimateGas(ctx context.Context, id uint64, address string, data []byte) (uint64, error) {
	c.logger.Debug(ctx, "", log.MapFields{
		"call_type": "EstimateGasAttempt",
		"id":        id,
		"address":   address,
	})

	var to *common.Address
	var hex common.Address
	if len(address) > 0 {
		hex = common.HexToAddress(address)
		to = &hex
	}

	gas, err := c.client.EstimateGas(ctx, ethereum.CallMsg{
		From:     crypto.PubkeyToAddress(c.wallet.PrivateKey.PublicKey),
		To:       to,
		Gas:      0,
		GasPrice: big.NewInt(gasPrice),
		Value:    big.NewInt(0),
		Data:     data,
	})

	if err != nil {
		c.logger.Debug(ctx, "", log.MapFields{
			"call_type": "EstimateGasFailure",
			"id":        id,
			"address":   address,
			"err":       err.Error(),
		})
		return 0, err
	}

	c.logger.Debug(ctx, "", log.MapFields{
		"call_type": "EstimateGasSuccess",
		"id":        id,
		"address":   address,
	})

	return gas, nil
}

func Dial(ctx context.Context, logger log.Logger, properties EthClientProperties) (*EthClient, error) {
	if len(properties.URL) == 0 {
		return nil, stderr.New("no url provided for eth client")
	}

	url, err := url.Parse(properties.URL)
	if err != nil {
		return nil, fmt.Errorf("Failed to parse url %s", err.Error())
	}

	if url.Scheme != "wss" && url.Scheme != "ws" {
		return nil, stderr.New("Only schemes supported are ws and wss")
	}

	rpcClient, err := erpc.DialWebsocket(ctx, properties.URL, "")
	if err != nil {
		return nil, fmt.Errorf("Failed to create websocket connection %s", err.Error())
	}

	client := ethclient.NewClient(rpcClient)
	c := &EthClient{
		ctx:       ctx,
		wg:        sync.WaitGroup{},
		inCh:      make(chan ethRequest, 64),
		logger:    logger.ForClass("eth", "EthClient"),
		nonce:     0,
		signer:    types.FrontierSigner{},
		wallet:    properties.Wallet,
		client:    client,
		rpcClient: rpcClient,
	}

	c.startLoop(ctx)
	return c, nil
}
