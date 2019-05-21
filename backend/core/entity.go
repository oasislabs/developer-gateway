package core

import "github.com/oasislabs/developer-gateway/rpc"

// ExecuteServiceRequest is is used by the user to trigger a service
// execution. A client is always subscribed to a subcription with
// topic "service" from which the client can retrieve the asynchronous
// results to this request
type ExecuteServiceRequest struct {
	// Data is a blob of data that the user wants to pass to the service
	// as argument
	Data string

	// Address where the service can be found
	Address string

	// Key is the identifier of the request issuer
	Key string
}

// DeployServiceRequest is issued by the user to trigger a service
// execution. A client is always subscribed to a subcription with
// topic "service" from which the client can retrieve the asynchronous
// results to this request
type DeployServiceRequest struct {
	// Data is a blob of data that the user wants to pass as argument for
	// the deployment of a service
	Data string

	// Key is the identifier of the request issuer
	Key string
}

// GetPublicKeyServiceRequest is a request to retrieve the public key
// associated with a specific service
type GetPublicKeyServiceRequest struct {
	// Address is the unique address that identifies the service,
	// is generated when a service is deployed and it can be used
	// for service execution
	Address string `json:"address"`
}

// GetPublicKeyServiceResponse is the response in which the public key
// associated with the contract is provided
type GetPublicKeyServiceResponse struct {
	// Timestamp at which the key expired
	Timestamp uint64

	// Address is the unique address that identifies the service,
	// is generated when a service is deployed and it can be used
	// for service execution
	Address string

	// PublicKey associated to the service
	PublicKey string

	// Signature from the key manager to authenticate the public key
	Signature string
}

// ErrorEvent is the event that can be polled by the user
// as a result to a a request that failed
type ErrorEvent struct {
	// ID to identifiy an asynchronous response. It uniquely identifies the
	// event and orders it in the sequence of events expected by the user
	ID uint64

	// Cause is the error that caused the event to failed
	Cause rpc.Error
}

// ExecuteServiceResponse is the event that can be polled by the user
// as a result to a ServiceExecutionRequest
type ExecuteServiceResponse struct {
	// ID to identify an asynchronous response. It uniquely identifies the
	// event and orders it in the sequence of events expected by the user
	ID uint64

	// Address is the unique address that identifies the service,
	// is generated when a service is deployed and it can be used
	// for service execution
	Address string

	// Output generated by the service at the end of its execution
	Output string
}

// DeployServiceResponse is the event that can be polled by the user
// as a result to a ServiceDeployRequest
type DeployServiceResponse struct {
	// ID to identify an asynchronous response. It uniquely identifies the
	// event and orders it in the sequence of events expected by the user
	ID uint64

	// Address is the unique address that identifies the service,
	// is generated when a service is deployed and it can be used
	// for service execution
	Address string
}

// EventID is the implementation of rpc.Event for ExecuteServiceResponse
func (e ExecuteServiceResponse) EventID() uint64 {
	return e.ID
}

// EventID is the implementation of rpc.Event for DeployServiceResponse
func (e DeployServiceResponse) EventID() uint64 {
	return e.ID
}

// EventID is the implementation of rpc.Event for ErrorEvent
func (e ErrorEvent) EventID() uint64 {
	return e.ID
}

// SubscribeRequest is a request issued by the client to subscribe to a
// specific topic and receive events from it until the subscription is
// closed
type SubscribeRequest struct {
	// Topic is the subscription topic
	Topic string

	// Address will be used to filter events only issues by or to
	// the address
	Address string

	// Key is the identifier of the request issuer
	Key string
}
