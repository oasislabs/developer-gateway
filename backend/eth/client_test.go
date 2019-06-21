package eth

import (
	"context"
	"crypto/ecdsa"
	"errors"
	"fmt"
	"io/ioutil"
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/oasislabs/developer-gateway/backend/core"
	backend "github.com/oasislabs/developer-gateway/backend/core"
	"github.com/oasislabs/developer-gateway/callback/callbacktest"
	"github.com/oasislabs/developer-gateway/eth"
	"github.com/oasislabs/developer-gateway/eth/ethtest"
	"github.com/oasislabs/developer-gateway/log"
	"github.com/oasislabs/developer-gateway/tx"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

var (
	Context           = context.TODO()
	PrivateKey string = "17be884d0713e46a983fe65900c0ee0f45696cee60e5611ebc80841cfad407b7"
	Logger            = log.NewLogrus(log.LogrusLoggerProperties{
		Level:  logrus.DebugLevel,
		Output: ioutil.Discard,
	})
)

func GetPrivateKey() *ecdsa.PrivateKey {
	privateKey, err := crypto.HexToECDSA(PrivateKey)
	if err != nil {
		panic(fmt.Sprintf("failed to create private key: %s", err.Error()))
	}

	return privateKey
}

func NewClientWithMock() (*Client, error) {
	mockclient := &ethtest.MockClient{}
	mockcallbacks := &callbacktest.MockClient{}

	mockclient.On("BalanceAt",
		mock.AnythingOfType("*context.emptyCtx"),
		mock.AnythingOfType("common.Address"),
		mock.AnythingOfType("*big.Int")).
		Return(big.NewInt(1), nil)

	executor, err := tx.NewExecutor(Context, &tx.ExecutorServices{
		Logger:    Logger,
		Client:    mockclient,
		Callbacks: mockcallbacks,
	}, &tx.ExecutorProps{PrivateKeys: []*ecdsa.PrivateKey{GetPrivateKey()}})
	if err != nil {
		return nil, err
	}

	return NewClientWithDeps(Context, &ClientDeps{
		Logger:   Logger,
		Client:   mockclient,
		Executor: executor,
	}), nil
}

func TestGetPublicKeyInvalidAddress(t *testing.T) {
	client, err := NewClientWithMock()
	assert.Nil(t, err)

	_, err = client.GetPublicKey(Context, backend.GetPublicKeyRequest{
		Address: "0x",
	})
	assert.Error(t, err)
	assert.Equal(t, "[2006] error code InputError with desc Provided invalid address.", err.Error())
}

func TestGetPublicKeyErr(t *testing.T) {
	client, err := NewClientWithMock()
	assert.Nil(t, err)

	client.client.(*ethtest.MockClient).On("GetPublicKey",
		mock.AnythingOfType("*context.emptyCtx"),
		mock.AnythingOfType("common.Address")).
		Return(eth.PublicKey{}, errors.New("error"))

	_, err = client.GetPublicKey(Context, backend.GetPublicKeyRequest{
		Address: "0x0000000000000000000000000000000000000000",
	})
	assert.Error(t, err)
	assert.Equal(t, "[1000] error code InternalError with desc Internal Error. Please check the status of the service. with cause failed to get public key error", err.Error())
}

func TestGetPublicKeyOK(t *testing.T) {
	client, err := NewClientWithMock()
	assert.Nil(t, err)

	client.client.(*ethtest.MockClient).On("GetPublicKey",
		mock.AnythingOfType("*context.emptyCtx"),
		mock.AnythingOfType("common.Address")).
		Return(eth.PublicKey{
			Timestamp: 1234,
			PublicKey: "0x6f6704e5a10332af6672e50b3d9754dc460dfa4d",
			Signature: "0x6f6704e5a10332af6672e50b3d9754dc460dfa4d",
		}, nil)

	pk, err := client.GetPublicKey(Context, backend.GetPublicKeyRequest{
		Address: "0x0000000000000000000000000000000000000000",
	})

	assert.Nil(t, err)
	assert.Equal(t, core.GetPublicKeyResponse{
		Timestamp: 1234,
		Address:   "0x0000000000000000000000000000000000000000",
		PublicKey: "0x6f6704e5a10332af6672e50b3d9754dc460dfa4d",
		Signature: "0x6f6704e5a10332af6672e50b3d9754dc460dfa4d",
	}, pk)
}

func TestDeployServiceOK(t *testing.T) {
	client, err := NewClientWithMock()
	assert.Nil(t, err)

	client.client.(*ethtest.MockClient).On("EstimateGas",
		mock.AnythingOfType("*context.emptyCtx"),
		mock.AnythingOfType("ethereum.CallMsg")).
		Return(uint64(0), nil)
	client.client.(*ethtest.MockClient).On("NonceAt",
		mock.AnythingOfType("*context.emptyCtx"),
		mock.AnythingOfType("common.Address")).
		Return(uint64(1), nil)
	client.client.(*ethtest.MockClient).On("BalanceAt",
		mock.AnythingOfType("*context.emptyCtx"),
		mock.AnythingOfType("common.Address"),
		mock.AnythingOfType("*big.Int")).
		Return(big.NewInt(1), nil)
	client.client.(*ethtest.MockClient).On("TransactionReceipt",
		mock.AnythingOfType("*context.emptyCtx"),
		mock.AnythingOfType("common.Hash")).
		Return(&types.Receipt{
			ContractAddress: common.HexToAddress(strings.Repeat("0", 20)),
		}, nil)
	client.client.(*ethtest.MockClient).On("SendTransaction",
		mock.AnythingOfType("*context.emptyCtx"),
		mock.Anything).
		Return(eth.SendTransactionResponse{
			Status: StatusOK,
			Output: "Success",
			Hash:   "Some hash",
		}, nil)

	res, err := client.DeployService(Context, 1, backend.DeployServiceRequest{
		Data: "0x0000000000000000000000000000000000000000",
	})

	assert.Nil(t, err)
	assert.Equal(t, backend.DeployServiceResponse{
		ID:      uint64(1),
		Address: "0x0000000000000000000000000000000000000000",
	}, res)
}

func TestDeployServiceEstimateGasErr(t *testing.T) {
	client, err := NewClientWithMock()
	assert.Nil(t, err)

	client.client.(*ethtest.MockClient).On("EstimateGas",
		mock.AnythingOfType("*context.emptyCtx"),
		mock.AnythingOfType("ethereum.CallMsg")).
		Return(uint64(0), errors.New("error"))

	_, err = client.DeployService(Context, 1, backend.DeployServiceRequest{
		Data: "0x0000000000000000000000000000000000000000",
	})

	assert.Equal(t, "[1002] error code InternalError with desc Internal Error. Please check the status of the service. with cause error", err.Error())
}

func TestExecuteServiceOK(t *testing.T) {
	client, err := NewClientWithMock()
	assert.Nil(t, err)

	client.client.(*ethtest.MockClient).On("EstimateGas",
		mock.AnythingOfType("*context.emptyCtx"),
		mock.AnythingOfType("ethereum.CallMsg")).
		Return(uint64(0), nil)
	client.client.(*ethtest.MockClient).On("NonceAt",
		mock.AnythingOfType("*context.emptyCtx"),
		mock.AnythingOfType("common.Address")).
		Return(uint64(1), nil)
	client.client.(*ethtest.MockClient).On("BalanceAt",
		mock.AnythingOfType("*context.emptyCtx"),
		mock.AnythingOfType("common.Address"),
		mock.AnythingOfType("*big.Int")).
		Return(big.NewInt(1), nil)
	client.client.(*ethtest.MockClient).On("TransactionReceipt",
		mock.AnythingOfType("*context.emptyCtx"),
		mock.AnythingOfType("common.Hash")).
		Return(&types.Receipt{
			ContractAddress: common.HexToAddress(strings.Repeat("0", 20)),
		}, nil)
	client.client.(*ethtest.MockClient).On("SendTransaction",
		mock.AnythingOfType("*context.emptyCtx"),
		mock.Anything).
		Return(eth.SendTransactionResponse{
			Status: StatusOK,
			Output: "Success",
			Hash:   "Some hash",
		}, nil)

	res, err := client.ExecuteService(Context, 1, backend.ExecuteServiceRequest{
		Address: "0x5d352cf2160f79CBF3554534cF25A4b42C43D502",
		Data:    "0x0000000000000000000000000000000000000000",
	})

	assert.Nil(t, err)
	assert.Equal(t, backend.ExecuteServiceResponse{
		ID:      uint64(1),
		Address: "0x5d352cf2160f79CBF3554534cF25A4b42C43D502",
		Output:  "Success",
	}, res)
}

func TestExecuteServiceEstimateGasErr(t *testing.T) {
	client, err := NewClientWithMock()
	assert.Nil(t, err)

	client.client.(*ethtest.MockClient).On("EstimateGas",
		mock.AnythingOfType("*context.emptyCtx"),
		mock.AnythingOfType("ethereum.CallMsg")).
		Return(uint64(0), errors.New("error"))

	_, err = client.ExecuteService(Context, 1, backend.ExecuteServiceRequest{
		Address: "0x5d352cf2160f79CBF3554534cF25A4b42C43D502",
		Data:    "0x0000000000000000000000000000000000000000",
	})

	assert.Equal(t, "[1002] error code InternalError with desc Internal Error. Please check the status of the service. with cause error", err.Error())
}

func TestExecuteServiceEmptyAddressErr(t *testing.T) {
	client, err := NewClientWithMock()
	assert.Nil(t, err)

	client.client.(*ethtest.MockClient).On("EstimateGas",
		mock.AnythingOfType("*context.emptyCtx"),
		mock.AnythingOfType("ethereum.CallMsg")).
		Return(uint64(0), errors.New("error"))

	_, err = client.ExecuteService(Context, 1, backend.ExecuteServiceRequest{
		Data:    "0x0000000000000000000000000000000000000000",
		Address: "",
	})

	assert.Equal(t, "[2006] error code InputError with desc Provided invalid address.", err.Error())
}

func TestExecuteServiceNoHexAddressErr(t *testing.T) {
	client, err := NewClientWithMock()
	assert.Nil(t, err)

	client.client.(*ethtest.MockClient).On("EstimateGas",
		mock.AnythingOfType("*context.emptyCtx"),
		mock.AnythingOfType("ethereum.CallMsg")).
		Return(uint64(0), errors.New("error"))

	_, err = client.ExecuteService(Context, 1, backend.ExecuteServiceRequest{
		Data:    "0x0000000000000000000000000000000000000000",
		Address: "addressaddressaddressaddressaddressad",
	})

	assert.Equal(t, "[2006] error code InputError with desc Provided invalid address.", err.Error())
}
