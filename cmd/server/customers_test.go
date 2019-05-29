// Copyright 2018 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/moov-io/base"
	client "github.com/moov-io/customers/client"

	"github.com/go-kit/kit/log"
	"github.com/gorilla/mux"
)

type testCustomerRepository struct {
	err      error
	customer *client.Customer
}

func (r *testCustomerRepository) getCustomer(customerId string) (*client.Customer, error) {
	if r.err != nil {
		return nil, r.err
	}
	return r.customer, nil
}

func (r *testCustomerRepository) createCustomer(req customerRequest) (*client.Customer, error) {
	if r.err != nil {
		return nil, r.err
	}
	return r.customer, nil
}

func (r *testCustomerRepository) getCustomerMetadata(customerId string) (map[string]string, error) {
	out := make(map[string]string)
	return out, r.err
}

func (r *testCustomerRepository) replaceCustomerMetadata(customerId string, metadata map[string]string) error {
	return r.err
}

func TestCustomers__getCustomerId(t *testing.T) {
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/ping", nil)

	if id := getCustomerId(w, req); id != "" {
		t.Errorf("unexpected id: %v", id)
	}
}

func TestCustomers__GetCustomer(t *testing.T) {
	repo := createTestCustomerRepository(t)
	defer repo.close()
	cust, err := repo.createCustomer(customerRequest{
		FirstName: "Jane",
		LastName:  "Doe",
		Email:     "jane@example.com",
	})
	if err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", fmt.Sprintf("/customers/%s", cust.Id), nil)
	req.Header.Set("x-user-id", "test")
	req.Header.Set("x-request-id", "test")

	router := mux.NewRouter()
	addCustomerRoutes(log.NewNopLogger(), router, repo)
	router.ServeHTTP(w, req)
	w.Flush()

	if w.Code != http.StatusOK {
		t.Errorf("bogus status code: %d", w.Code)
	}

	var customer client.Customer // TODO(adam): check more of Customer response?
	if err := json.NewDecoder(w.Body).Decode(&customer); err != nil {
		t.Fatal(err)
	}
	if customer.Id == "" {
		t.Error("empty Customer.Id")
	}
}

func TestCustomers__GetCustomersError(t *testing.T) {
	repo := &testCustomerRepository{err: errors.New("bad error")}

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/customers/foo", nil)
	req.Header.Set("x-user-id", "test")
	req.Header.Set("x-request-id", "test")

	router := mux.NewRouter()
	addCustomerRoutes(log.NewNopLogger(), router, repo)
	router.ServeHTTP(w, req)
	w.Flush()

	if w.Code != http.StatusBadRequest {
		t.Errorf("bogus status code: %d", w.Code)
	}
}

func TestCustomers__customerRequest(t *testing.T) {
	req := &customerRequest{}
	if err := req.validate(); err == nil {
		t.Error("expected error")
	}
	req.FirstName = "jane"
	req.LastName = "doe"
	if err := req.validate(); err == nil {
		t.Error("expected error")
	}
	req.Email = "jane.doe@example.com"
	if err := req.validate(); err == nil {
		t.Error("expected error")
	}
	req.Phones = append(req.Phones, phone{
		Number: "123.456.7890",
		Type:   "Checking",
	})
	if err := req.validate(); err == nil {
		t.Error("expected error")
	}
	req.Addresses = append(req.Addresses, address{
		Address1:   "123 1st st",
		City:       "fake city",
		State:      "CA",
		PostalCode: "90210",
		Country:    "US",
	})
	if err := req.validate(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// asCustomer
	cust := req.asCustomer()
	if cust.Id == "" {
		t.Errorf("empty Customer: %#v", cust)
	}
	if len(cust.Phones) != 1 {
		t.Errorf("cust.Phones: %#v", cust.Phones)
	}
	if len(cust.Addresses) != 1 {
		t.Errorf("cust.Addresses: %#v", cust.Addresses)
	}
}

func TestCustomers__createCustomer(t *testing.T) {
	w := httptest.NewRecorder()
	phone := `{"number": "555.555.5555", "type": "mobile"}`
	address := `{"type": "home", "address1": "123 1st St", "city": "Denver", "state": "CO", "postalCode": "12345", "country": "USA"}`
	body := fmt.Sprintf(`{"firstName": "jane", "lastName": "doe", "email": "jane@example.com", "phones": [%s], "addresses": [%s]}`, phone, address)
	req := httptest.NewRequest("POST", "/customers", strings.NewReader(body))
	req.Header.Set("x-user-id", "test")
	req.Header.Set("x-request-id", "test")

	repo := createTestCustomerRepository(t)
	defer repo.close()

	router := mux.NewRouter()
	addCustomerRoutes(log.NewNopLogger(), router, repo)
	router.ServeHTTP(w, req)
	w.Flush()

	if w.Code != http.StatusOK {
		t.Errorf("bogus status code: %d", w.Code)
	}

	var cust client.Customer // TODO(adam): check more of Customer response?
	if err := json.NewDecoder(w.Body).Decode(&cust); err != nil {
		t.Fatal(err)
	}
	if cust.Id == "" {
		t.Error("empty Customer.Id")
	}

	// sad path
	w = httptest.NewRecorder()
	req = httptest.NewRequest("POST", "/customers", strings.NewReader("null"))
	req.Header.Set("x-user-id", "test")
	req.Header.Set("x-request-id", "test")
	router.ServeHTTP(w, req)
	w.Flush()

	if w.Code != http.StatusBadRequest {
		t.Fatalf("bogus HTTP status code: %d", w.Code)
	}
}

func createTestCustomerRepository(t *testing.T) *sqliteCustomerRepository {
	t.Helper()
	db, err := createTestSqliteDB()
	if err != nil {
		t.Fatal(err)
	}
	if err := migrate(nil, db.db); err != nil {
		t.Fatal(err)
	}
	return &sqliteCustomerRepository{db.db}
}

func TestCustomers__repository(t *testing.T) {
	repo := createTestCustomerRepository(t)
	defer repo.close()

	cust, err := repo.getCustomer(base.ID())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if cust != nil {
		t.Error("expected no Customer")
	}

	// write
	req := customerRequest{
		FirstName: "Jane",
		LastName:  "Doe",
		Email:     "jane@example.com",
		Phones: []phone{
			{
				Number: "123.456.7890",
				Type:   "Checking",
			},
		},
		Addresses: []address{
			{
				Address1:   "123 1st st",
				City:       "fake city",
				State:      "CA",
				PostalCode: "90210",
				Country:    "US",
			},
		},
	}
	cust, err = repo.createCustomer(req)
	if err != nil {
		t.Error(err)
	}
	if cust == nil {
		t.Fatal("nil Customer")
	}
	if len(cust.Phones) != 1 || len(cust.Addresses) != 1 {
		t.Errorf("len(cust.Phones)=%d and len(cust.Addresses)=%d", len(cust.Phones), len(cust.Addresses))
	}

	// read
	cust, err = repo.getCustomer(cust.Id)
	if err != nil {
		t.Error(err)
	}
	if cust == nil {
		t.Fatal("nil Customer")
	}
	if len(cust.Phones) != 1 || len(cust.Addresses) != 1 {
		t.Errorf("len(cust.Phones)=%d and len(cust.Addresses)=%d", len(cust.Phones), len(cust.Addresses))
	}
}

func TestCustomers__replaceCustomerMetadata(t *testing.T) {
	repo := createTestCustomerRepository(t)
	defer repo.close()

	cust, err := repo.createCustomer(customerRequest{
		FirstName: "Jane",
		LastName:  "Doe",
		Email:     "jane@example.com",
	})
	if err != nil {
		t.Fatal(err)
	}

	body := strings.NewReader(`{ "metadata": { "key": "bar"} }`)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("PUT", fmt.Sprintf("/customers/%s/metadata", cust.Id), body)
	req.Header.Set("x-user-id", "test")
	req.Header.Set("x-request-id", "test")

	router := mux.NewRouter()
	addCustomerRoutes(log.NewNopLogger(), router, repo)
	router.ServeHTTP(w, req)
	w.Flush()

	if w.Code != http.StatusOK {
		t.Errorf("bogus status code: %d", w.Code)
	}

	var customer client.Customer
	if err := json.NewDecoder(w.Body).Decode(&customer); err != nil {
		t.Fatal(err)
	}
	if customer.Metadata["key"] != "bar" {
		t.Errorf("unknown Customer metadata: %#v", customer.Metadata)
	}

	// sad path
	repo2 := &testCustomerRepository{err: errors.New("bad error")}

	w = httptest.NewRecorder()
	req = httptest.NewRequest("PUT", "/customers/foo/metadata", nil)
	req.Header.Set("x-user-id", "test")
	req.Header.Set("x-request-id", "test")

	router2 := mux.NewRouter()
	addCustomerRoutes(log.NewNopLogger(), router2, repo2)
	router.ServeHTTP(w, req)
	w.Flush()

	if w.Code != http.StatusBadRequest {
		t.Errorf("bogus status code: %d", w.Code)
	}
}

func TestCustomers__replaceCustomerMetadataInvalid(t *testing.T) {
	repo := createTestCustomerRepository(t)
	defer repo.close()

	cust, err := repo.createCustomer(customerRequest{
		FirstName: "Jane",
		LastName:  "Doe",
		Email:     "jane@example.com",
	})
	if err != nil {
		t.Fatal(err)
	}

	r := repalceMetadataRequest{
		Metadata: map[string]string{"key": strings.Repeat("a", 100000)},
	}
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(r); err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	req := httptest.NewRequest("PUT", fmt.Sprintf("/customers/%s/metadata", cust.Id), &buf)
	req.Header.Set("x-user-id", "test")
	req.Header.Set("x-request-id", "test")

	router := mux.NewRouter()
	addCustomerRoutes(log.NewNopLogger(), router, repo)
	router.ServeHTTP(w, req)
	w.Flush()

	if w.Code != http.StatusBadRequest {
		t.Errorf("bogus status code: %d", w.Code)
	}

	// invalid JSON
	w = httptest.NewRecorder()
	req = httptest.NewRequest("PUT", fmt.Sprintf("/customers/%s/metadata", cust.Id), strings.NewReader("{invalid-json"))
	req.Header.Set("x-user-id", "test")
	req.Header.Set("x-request-id", "test")

	router.ServeHTTP(w, req)
	w.Flush()

	if w.Code != http.StatusBadRequest {
		t.Errorf("bogus status code: %d", w.Code)
	}
}

func TestCustomers__replaceCustomerMetadataError(t *testing.T) {
	repo := &testCustomerRepository{err: errors.New("bad error")}

	body := strings.NewReader(`{ "metadata": { "key": "bar"} }`)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("PUT", "/customers/foo/metadata", body)
	req.Header.Set("x-user-id", "test")
	req.Header.Set("x-request-id", "test")

	router := mux.NewRouter()
	addCustomerRoutes(log.NewNopLogger(), router, repo)
	router.ServeHTTP(w, req)
	w.Flush()

	if w.Code != http.StatusBadRequest {
		t.Errorf("bogus status code: %d", w.Code)
	}
}

func TestCustomerRepository__metadata(t *testing.T) {
	repo := createTestCustomerRepository(t)
	defer repo.close()

	customerId := base.ID()

	meta, err := repo.getCustomerMetadata(customerId)
	if err != nil {
		t.Fatal(err)
	}
	if len(meta) != 0 {
		t.Errorf("unknown metadata: %#v", meta)
	}

	// replace
	if err := repo.replaceCustomerMetadata(customerId, map[string]string{"key": "bar"}); err != nil {
		t.Fatal(err)
	}
	meta, err = repo.getCustomerMetadata(customerId)
	if err != nil {
		t.Fatal(err)
	}
	if len(meta) != 1 || meta["key"] != "bar" {
		t.Errorf("unknown metadata: %#v", meta)
	}
}

func TestCustomers__validateMetadata(t *testing.T) {
	meta := make(map[string]string)
	if err := validateMetadata(meta); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	meta["key"] = "foo"
	if err := validateMetadata(meta); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	meta["bar"] = strings.Repeat("b", 100000) // invalid
	if err := validateMetadata(meta); err == nil {
		t.Error("expected error")
	}

	meta["bar"] = "baz"         // valid again
	for i := 0; i < 1000; i++ { // add too many keys
		meta[fmt.Sprintf("key-%d", i)] = fmt.Sprintf("%d", i)
	}
	if err := validateMetadata(meta); err == nil {
		t.Error("expected error")
	}
}