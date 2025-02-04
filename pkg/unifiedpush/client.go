package unifiedpush

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"strconv"

	pushv1alpha1 "github.com/aerogear/unifiedpush-operator/pkg/apis/push/v1alpha1"
	"github.com/pkg/errors"
)

// variant is an internal base type with shared fields used in
// androidVariant and iOSVariant
type variant struct {
	Name        string
	Description string
	VariantId   string
	Secret      string
}

// androidVariant is an internal struct used for convenient JSON
// unmarshalling of the response received from UPS
type androidVariant struct {
	ProjectNumber string
	GoogleKey     string
	variant
}

// iOSVariant is an internal struct used for convenient JSON
// unmarshalling of the response received from UPS
type iOSVariant struct {
	Certificate []byte
	Passphrase  string
	Production  bool
	variant
}

// pushApplication is an internal struct used for convenient JSON
// unmarshalling of the response received from UPS
type pushApplication struct {
	PushApplicationId string
	MasterSecret      string
}

// UnifiedpushClient is a client to enable easy interaction with a UPS
// server
type UnifiedpushClient struct {
	Url string
}

// GetApplication does a GET for a given PushApplication based on the PushApplicationId
func (c UnifiedpushClient) GetApplication(p *pushv1alpha1.PushApplication) (string, error) {
	if p.Status.PushApplicationId == "" {
		// We haven't created it yet
		return "", nil
	}

	url := fmt.Sprintf("%s/rest/applications/%s", c.Url, p.Status.PushApplicationId)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	resp, err := doUPSRequest(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var foundApplication pushApplication
	b, _ := ioutil.ReadAll(resp.Body)
	json.Unmarshal(b, &foundApplication)
	fmt.Printf("Found app: %v\n", foundApplication)

	return foundApplication.PushApplicationId, nil
}

// CreateApplication creates an application in UPS
func (c UnifiedpushClient) CreateApplication(p *pushv1alpha1.PushApplication) (string, string, error) {
	url := fmt.Sprintf("%s/rest/applications/", c.Url)

	params := map[string]string{
		"name":        p.Name,
		"description": p.Spec.Description,
	}

	payload, err := json.Marshal(params)
	if err != nil {
		return "", "", errors.Wrap(err, "Failed to marshal push application params to json")
	}

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := doUPSRequest(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 201 {
		return "", "", errors.New(fmt.Sprintf("UPS responded with status code: %v, but expected 201", resp.StatusCode))
	}

	var createdApplication pushApplication
	b, _ := ioutil.ReadAll(resp.Body)
	json.Unmarshal(b, &createdApplication)

	return createdApplication.PushApplicationId, createdApplication.MasterSecret, nil
}

// DeleteApplication deletes a PushApplication in UPS
func (c UnifiedpushClient) DeleteApplication(p *pushv1alpha1.PushApplication) error {
	if p.Status.PushApplicationId == "" {
		return errors.New("No PushApplicationId set in the PushApplication status")
	}

	url := fmt.Sprintf("%s/rest/applications/%s", c.Url, p.Status.PushApplicationId)

	req, err := http.NewRequest(http.MethodDelete, url, nil)
	resp, err := doUPSRequest(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 204 {
		return errors.New(fmt.Sprintf("UPS responded with status code: %v, but expected 204", resp.StatusCode))
	}

	return nil
}

// GetAndroidVariant does a GET for a given Variant based on the VariantId
func (c UnifiedpushClient) GetAndroidVariant(v *pushv1alpha1.AndroidVariant) (string, error) {
	if v.Status.VariantId == "" {
		// We haven't created it yet
		return "", nil
	}

	url := fmt.Sprintf("%s/rest/applications/%s/android/%s", c.Url, v.Spec.PushApplicationId, v.Status.VariantId)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	resp, err := doUPSRequest(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var foundVariant androidVariant
	b, _ := ioutil.ReadAll(resp.Body)
	json.Unmarshal(b, &foundVariant)
	fmt.Printf("Found app: %v\n", foundVariant)

	// TODO: maybe now it's time to start using the "API types"
	return foundVariant.VariantId, nil
}

// CreateAndroidVariant creates a Variant on an Application in UPS
func (c UnifiedpushClient) CreateAndroidVariant(v *pushv1alpha1.AndroidVariant) (string, error) {
	url := fmt.Sprintf("%s/rest/applications/%s/android", c.Url, v.Spec.PushApplicationId)

	params := map[string]string{
		"projectNumber": "1",
		"name":          v.Name,
		"googleKey":     v.Spec.ServerKey,
		"description":   v.Spec.Description,
	}

	payload, err := json.Marshal(params)
	if err != nil {
		return "", errors.Wrap(err, "Failed to marshal android variant params to json")
	}

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := doUPSRequest(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 201 {
		return "", errors.New(fmt.Sprintf("UPS responded with status code: %v, but expected 201", resp.StatusCode))
	}

	var createdVariant androidVariant
	b, _ := ioutil.ReadAll(resp.Body)
	json.Unmarshal(b, &createdVariant)

	return createdVariant.VariantId, nil
}

// DeleteAndroidVariant deletes an Android variant in UPS
func (c UnifiedpushClient) DeleteAndroidVariant(v *pushv1alpha1.AndroidVariant) error {
	if v.Status.VariantId == "" {
		// We haven't created it yet
		return nil
	}

	url := fmt.Sprintf("%s/rest/applications/%s/android/%s", c.Url, v.Spec.PushApplicationId, v.Status.VariantId)

	req, err := http.NewRequest(http.MethodDelete, url, nil)
	resp, err := doUPSRequest(req)
	if err != nil {
		return err
	}

	if resp.StatusCode != 204 {
		return errors.New(fmt.Sprintf("Expected a status code of 204 for variant deletion in UPS, but got %v", resp.StatusCode))
	}

	return nil
}

// GetIOSVariant does a GET for a given iOS Variant based on the VariantId
func (c UnifiedpushClient) GetIOSVariant(v *pushv1alpha1.IOSVariant) (string, error) {
	if v.Status.VariantId == "" {
		// We haven't created it yet
		return "", nil
	}

	url := fmt.Sprintf("%s/rest/applications/%s/ios/%s", c.Url, v.Spec.PushApplicationId, v.Status.VariantId)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	resp, err := doUPSRequest(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var foundVariant iOSVariant
	b, _ := ioutil.ReadAll(resp.Body)
	json.Unmarshal(b, &foundVariant)
	fmt.Printf("Found app: %v\n", foundVariant)

	// TODO: maybe now it's time to start using the "API types"
	return foundVariant.VariantId, nil
}

// CreateIOSVariant creates a Variant on an Application in UPS
func (c UnifiedpushClient) CreateIOSVariant(v *pushv1alpha1.IOSVariant) (string, error) {
	url := fmt.Sprintf("%s/rest/applications/%s/ios", c.Url, v.Spec.PushApplicationId)

	params := map[string]string{
		"name":        v.Name,
		"passphrase":  v.Spec.Passphrase,
		"description": v.Spec.Description,
		"production":  strconv.FormatBool(v.Spec.Production),
	}

	// We need to decode it before sending
	decodedString, err := base64.StdEncoding.DecodeString(string(v.Spec.Certificate))
	if err != nil {
		return "", errors.Wrap(err, "Invalid cert - Please check this cert is in base64 encoded format: ")
	}

	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)
	defer writer.Close()

	part, err := writer.CreateFormFile("certificate", "certificate")
	if err != nil {
		return "", errors.Wrap(err, "Failed to create form for UPS iOS variant request")
	}
	part.Write(decodedString)

	for key, val := range params {
		_ = writer.WriteField(key, val)
	}

	req, err := http.NewRequest(http.MethodPost, url, body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Accept", "application/json")

	resp, err := doUPSRequest(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 201 {
		return "", errors.New(fmt.Sprintf("UPS responded with status code: %v, but expected 201", resp.StatusCode))
	}

	var createdVariant iOSVariant
	b, _ := ioutil.ReadAll(resp.Body)
	json.Unmarshal(b, &createdVariant)

	return createdVariant.VariantId, nil
}

// DeleteIOSVariant deletes an IOS variant in UPS
func (c UnifiedpushClient) DeleteIOSVariant(v *pushv1alpha1.IOSVariant) error {
	if v.Status.VariantId == "" {
		// We haven't created it yet
		return nil
	}

	url := fmt.Sprintf("%s/rest/applications/%s/ios/%s", c.Url, v.Spec.PushApplicationId, v.Status.VariantId)

	req, err := http.NewRequest(http.MethodDelete, url, nil)
	resp, err := doUPSRequest(req)
	if err != nil {
		return err
	}

	if resp.StatusCode != 204 {
		return errors.New(fmt.Sprintf("Expected a status code of 204 for variant deletion in UPS, but got %v", resp.StatusCode))
	}

	return nil
}

func doUPSRequest(req *http.Request) (*http.Response, error) {
	httpClient := http.Client{}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "error sending request to UPS")
	}

	return resp, nil
}
