package validator

type StrategyKey struct {
	Strategy string
	Vendor   string
}

type Strategy interface {
	InitAccountValidation(userID, accountID, customerID string) (*VendorResponse, error)
	CompleteAccountValidation(userID, accountID, customerID string, request *VendorRequest) (*VendorResponse, error)
}

type VendorRequest map[string]interface{}
type VendorResponse map[string]interface{}

type testStrategy struct{}

func TestStrategy() Strategy {
	return &testStrategy{}
}

func (t *testStrategy) InitAccountValidation(userID, accountID, customerID string) (*VendorResponse, error) {
	return &VendorResponse{
		"test": "ok",
	}, nil
}

func (t *testStrategy) CompleteAccountValidation(userID, accountID, customerID string, request *VendorRequest) (*VendorResponse, error) {
	return &VendorResponse{
		"test": "ok",
	}, nil
}
